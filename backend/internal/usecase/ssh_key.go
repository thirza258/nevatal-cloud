package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/repository"
	"golang.org/x/crypto/ssh"
)

type SSHKeyUsecase struct {
	keys         repository.SSHKeyRepository
	auditService *AuditUsecase
}

func NewSSHKeyUsecase(keys repository.SSHKeyRepository, auditService *AuditUsecase) *SSHKeyUsecase {
	return &SSHKeyUsecase{keys: keys, auditService: auditService}
}

func (s *SSHKeyUsecase) ListKeys(ctx context.Context, userID uuid.UUID) ([]models.SSHKey, error) {
	return s.keys.ListByOwner(ctx, userID)
}

// CreateKey validates, stores and audits a public key. All validation
// happens before any repository call.
func (s *SSHKeyUsecase) CreateKey(ctx context.Context, userID uuid.UUID, name, rawPublicKey, clientIP string) (*models.SSHKey, error) {
	ctx = context.WithoutCancel(ctx)

	name = strings.TrimSpace(name)
	rawPublicKey = strings.TrimSpace(rawPublicKey)

	if name == "" {
		return nil, errors.New("key name is required")
	}
	if rawPublicKey == "" {
		return nil, errors.New("public key content is required")
	}

	// Strictly reject private keys if mistakenly supplied
	if strings.Contains(rawPublicKey, "PRIVATE KEY") {
		return nil, errors.New("private keys must never be submitted; please provide your public key only (.pub)")
	}

	pubKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(rawPublicKey))
	if err != nil {
		return nil, fmt.Errorf("invalid SSH public key format: %w", err)
	}

	fingerprint := ssh.FingerprintSHA256(pubKey)

	// Check if this user already registered this exact fingerprint
	count, _ := s.keys.CountByFingerprint(ctx, userID, fingerprint)
	if count > 0 {
		return nil, errors.New("this SSH key has already been added to your account")
	}

	key := models.SSHKey{
		OwnerID:     userID,
		Name:        name,
		PublicKey:   rawPublicKey,
		Fingerprint: fingerprint,
	}

	if err := s.keys.Create(ctx, &key); err != nil {
		return nil, err
	}

	s.auditService.Log(ctx, &userID, "ssh_key.create", "ssh_key", &key.ID, "success", clientIP, map[string]string{"name": key.Name})
	return &key, nil
}

func (s *SSHKeyUsecase) RenameKey(ctx context.Context, userID uuid.UUID, keyID uuid.UUID, newName string) error {
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return errors.New("name cannot be empty")
	}

	rows, err := s.keys.Rename(context.WithoutCancel(ctx), keyID, userID, newName)
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("key not found or access denied")
	}
	return nil
}

// DeleteKey removes the caller's key and audits it.
func (s *SSHKeyUsecase) DeleteKey(ctx context.Context, userID uuid.UUID, keyID uuid.UUID, clientIP string) error {
	ctx = context.WithoutCancel(ctx)

	rows, err := s.keys.Delete(ctx, keyID, userID)
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("key not found or access denied")
	}

	s.auditService.Log(ctx, &userID, "ssh_key.delete", "ssh_key", &keyID, "success", clientIP, nil)
	return nil
}
