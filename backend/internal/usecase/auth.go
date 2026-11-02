package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/config"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

type AuthUsecase struct {
	tx           repository.Transactor
	users        repository.UserRepository
	quotas       repository.QuotaRepository
	auditService *AuditUsecase
	cfg          *config.Config
	jwtSecret    []byte
}

func NewAuthUsecase(tx repository.Transactor, users repository.UserRepository, quotas repository.QuotaRepository, auditService *AuditUsecase, cfg *config.Config) *AuthUsecase {
	return &AuthUsecase{
		tx:           tx,
		users:        users,
		quotas:       quotas,
		auditService: auditService,
		cfg:          cfg,
		jwtSecret:    []byte(cfg.JWTSecret),
	}
}

type UserClaims struct {
	UserID uuid.UUID `json:"user_id"`
	Email  string    `json:"email"`
	Role   string    `json:"role"`
	jwt.RegisteredClaims
}

// createWithDefaultQuota inserts user and its default quota (4 vCPU,
// 8192 MB RAM, 100 GB storage, 3 instances) in one transaction.
func (s *AuthUsecase) createWithDefaultQuota(ctx context.Context, user *models.User) error {
	return s.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.users.Create(txCtx, user); err != nil {
			return err
		}
		quota := models.ResourceQuota{
			UserID:       user.ID,
			MaxVCPUs:     4,
			MaxMemoryMB:  8192,
			MaxStorageGB: 100,
			MaxInstances: 3,
		}
		return s.quotas.Create(txCtx, &quota)
	})
}

// Register creates a customer account with the default quota, audits it and
// returns a session token.
func (s *AuthUsecase) Register(ctx context.Context, email, password, displayName, clientIP string) (*models.User, string, error) {
	ctx = context.WithoutCancel(ctx)

	count, _ := s.users.CountByEmail(ctx, email)
	if count > 0 {
		return nil, "", errors.New("email is already registered")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", fmt.Errorf("failed to hash password: %w", err)
	}

	now := time.Now()
	user := models.User{
		Email:           email,
		PasswordHash:    string(hash),
		DisplayName:     displayName,
		Role:            models.RoleCustomer,
		Status:          models.UserStatusActive,
		EmailVerifiedAt: &now,
	}

	if err := s.createWithDefaultQuota(ctx, &user); err != nil {
		return nil, "", err
	}

	token, err := s.GenerateToken(&user)
	if err != nil {
		return nil, "", err
	}

	s.auditService.Log(ctx, &user.ID, "auth.register", "user", &user.ID, "success", clientIP, nil)
	return &user, token, nil
}

// Login checks the password and returns a session token. Successful and
// failed attempts are audited.
func (s *AuthUsecase) Login(ctx context.Context, email, password, clientIP string) (*models.User, string, error) {
	ctx = context.WithoutCancel(ctx)

	user, token, err := s.login(ctx, email, password)
	if err != nil {
		s.auditService.Log(ctx, nil, "auth.login_failed", "user", nil, "failure", clientIP, map[string]string{"email": email})
		return nil, "", err
	}

	s.auditService.Log(ctx, &user.ID, "auth.login", "user", &user.ID, "success", clientIP, nil)
	return user, token, nil
}

func (s *AuthUsecase) login(ctx context.Context, email, password string) (*models.User, string, error) {
	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		return nil, "", errors.New("invalid email or password")
	}

	if user.Status == models.UserStatusSuspended {
		return nil, "", errors.New("this account is suspended; contact platform support")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, "", errors.New("invalid email or password")
	}

	now := time.Now()
	user.LastLoginAt = &now
	_ = s.users.UpdateLastLogin(ctx, user, &now)

	token, err := s.GenerateToken(user)
	if err != nil {
		return nil, "", err
	}

	return user, token, nil
}

func (s *AuthUsecase) GenerateToken(user *models.User) (string, error) {
	claims := UserClaims{
		UserID: user.ID,
		Email:  user.Email,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   user.ID.String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

func (s *AuthUsecase) ValidateToken(tokenStr string) (*UserClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &UserClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*UserClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// GetUserByID returns the user with its quota preloaded.
func (s *AuthUsecase) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	return s.users.FindByIDWithQuota(ctx, id)
}

type OAuthUserInput struct {
	Provider    string `json:"provider"` // google, github, wordpress
	ProviderID  string `json:"provider_id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url"`
	// DenyAdmin refuses to sign in existing administrator accounts through
	// this provider (used for assertion-based SSO such as the WordPress bridge).
	DenyAdmin bool `json:"-"`
}

// OAuthLoginOrRegister signs in the account with input.Email, creating a
// customer account (with the default quota) when none exists.
func (s *AuthUsecase) OAuthLoginOrRegister(ctx context.Context, input OAuthUserInput) (*models.User, string, error) {
	ctx = context.WithoutCancel(ctx)

	if input.Email == "" {
		return nil, "", errors.New("email is required for social authentication")
	}

	existing, err := s.users.FindByEmail(ctx, input.Email)
	now := time.Now()

	if err == nil {
		// Existing user
		user := existing
		if user.Status == models.UserStatusSuspended {
			return nil, "", errors.New("this account is suspended; contact platform support")
		}
		if input.DenyAdmin && user.Role == models.RoleAdmin {
			return nil, "", errors.New("administrator accounts must sign in with their password")
		}

		user.LastLoginAt = &now
		if user.AuthProvider == "" || user.AuthProvider == "local" {
			user.AuthProvider = input.Provider
		}
		if input.ProviderID != "" {
			user.AuthID = input.ProviderID
		}
		if input.AvatarURL != "" {
			user.AvatarURL = input.AvatarURL
		}
		if user.DisplayName == "" && input.DisplayName != "" {
			user.DisplayName = input.DisplayName
		}

		_ = s.users.Save(ctx, user)
		token, err := s.GenerateToken(user)
		return user, token, err
	}

	// New user registration
	displayName := input.DisplayName
	if displayName == "" {
		displayName = input.Email
	}

	user := models.User{
		Email:           input.Email,
		DisplayName:     displayName,
		Role:            models.RoleCustomer,
		Status:          models.UserStatusActive,
		AuthProvider:    input.Provider,
		AuthID:          input.ProviderID,
		AvatarURL:       input.AvatarURL,
		EmailVerifiedAt: &now,
		LastLoginAt:     &now,
	}

	if err := s.createWithDefaultQuota(ctx, &user); err != nil {
		return nil, "", err
	}

	token, err := s.GenerateToken(&user)
	if err != nil {
		return nil, "", err
	}

	return &user, token, nil
}
