package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/config"
	"github.com/nevatal/cloud/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthService struct {
	db        *gorm.DB
	jwtSecret []byte
}

func NewAuthService(db *gorm.DB, cfg *config.Config) *AuthService {
	return &AuthService{
		db:        db,
		jwtSecret: []byte(cfg.JWTSecret),
	}
}

type UserClaims struct {
	UserID uuid.UUID `json:"user_id"`
	Email  string    `json:"email"`
	Role   string    `json:"role"`
	jwt.RegisteredClaims
}

func (s *AuthService) Register(email, password, displayName string) (*models.User, string, error) {
	var count int64
	s.db.Model(&models.User{}).Where("email = ?", email).Count(&count)
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

	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		// Default quota: 4 vCPU, 8192 MB RAM, 100 GB Storage, 3 instances
		quota := models.ResourceQuota{
			UserID:       user.ID,
			MaxVCPUs:     4,
			MaxMemoryMB:  8192,
			MaxStorageGB: 100,
			MaxInstances: 3,
		}
		return tx.Create(&quota).Error
	})
	if err != nil {
		return nil, "", err
	}

	token, err := s.GenerateToken(&user)
	if err != nil {
		return nil, "", err
	}

	return &user, token, nil
}

func (s *AuthService) Login(email, password string) (*models.User, string, error) {
	var user models.User
	if err := s.db.Where("email = ?", email).First(&user).Error; err != nil {
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
	s.db.Model(&user).Update("last_login_at", &now)

	token, err := s.GenerateToken(&user)
	if err != nil {
		return nil, "", err
	}

	return &user, token, nil
}

func (s *AuthService) GenerateToken(user *models.User) (string, error) {
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

func (s *AuthService) ValidateToken(tokenStr string) (*UserClaims, error) {
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

func (s *AuthService) GetUserByID(id uuid.UUID) (*models.User, error) {
	var user models.User
	err := s.db.Preload("Quota").First(&user, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

type OAuthUserInput struct {
	Provider    string `json:"provider"` // google, github, wordpress
	ProviderID  string `json:"provider_id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url"`
}

func (s *AuthService) OAuthLoginOrRegister(input OAuthUserInput) (*models.User, string, error) {
	if input.Email == "" {
		return nil, "", errors.New("email is required for social authentication")
	}

	var user models.User
	err := s.db.Where("email = ?", input.Email).First(&user).Error
	now := time.Now()

	if err == nil {
		// Existing user
		if user.Status == models.UserStatusSuspended {
			return nil, "", errors.New("this account is suspended; contact platform support")
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

		s.db.Save(&user)
		token, err := s.GenerateToken(&user)
		return &user, token, err
	}

	// New user registration
	displayName := input.DisplayName
	if displayName == "" {
		displayName = input.Email
	}

	user = models.User{
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

	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&user).Error; err != nil {
			return err
		}

		quota := models.ResourceQuota{
			UserID:       user.ID,
			MaxVCPUs:     4,
			MaxMemoryMB:  8192,
			MaxStorageGB: 100,
			MaxInstances: 3,
		}
		return tx.Create(&quota).Error
	})
	if err != nil {
		return nil, "", err
	}

	token, err := s.GenerateToken(&user)
	if err != nil {
		return nil, "", err
	}

	return &user, token, nil
}
