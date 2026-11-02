package usecase

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/config"
	"github.com/nevatal/cloud/internal/models"
	"golang.org/x/crypto/bcrypt"
)

func TestAuthTokenAndHashing(t *testing.T) {
	cfg := &config.Config{
		JWTSecret: "test-secret-key-for-testing-purposes",
	}
	svc := NewAuthUsecase(nil, nil, nil, nil, cfg)

	testUser := &models.User{
		ID:          uuid.New(),
		Email:       "developer@nevatal.id",
		DisplayName: "Developer",
		Role:        models.RoleCustomer,
		Status:      models.UserStatusActive,
		CreatedAt:   time.Now(),
	}

	// 1. Test token generation
	token, err := svc.GenerateToken(testUser)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	if token == "" {
		t.Fatalf("Expected non-empty JWT token")
	}

	// 2. Test token validation
	claims, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}
	if claims.UserID != testUser.ID {
		t.Errorf("Expected UserID %s, got %s", testUser.ID, claims.UserID)
	}
	if claims.Email != testUser.Email {
		t.Errorf("Expected Email %s, got %s", testUser.Email, claims.Email)
	}
	if claims.Role != testUser.Role {
		t.Errorf("Expected Role %s, got %s", testUser.Role, claims.Role)
	}

	// 3. Test password hashing & comparison
	password := "SecureCloudPass123!"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Bcrypt hash failed: %v", err)
	}

	if err := bcrypt.CompareHashAndPassword(hash, []byte(password)); err != nil {
		t.Errorf("Expected password to match hash")
	}

	if err := bcrypt.CompareHashAndPassword(hash, []byte("WrongPassword")); err == nil {
		t.Errorf("Expected wrong password to fail hash comparison")
	}
}
