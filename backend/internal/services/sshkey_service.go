package services

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

type SSHKeyService struct {
	db *gorm.DB
}

func NewSSHKeyService(db *gorm.DB) *SSHKeyService {
	return &SSHKeyService{db: db}
}

func (s *SSHKeyService) ListKeys(userID uuid.UUID) ([]models.SSHKey, error) {
	var keys []models.SSHKey
	err := s.db.Where("owner_id = ?", userID).Order("created_at desc").Find(&keys).Error
	return keys, err
}

func (s *SSHKeyService) CreateKey(userID uuid.UUID, name, rawPublicKey string) (*models.SSHKey, error) {
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
	var count int64
	s.db.Model(&models.SSHKey{}).Where("owner_id = ? AND fingerprint = ?", userID, fingerprint).Count(&count)
	if count > 0 {
		return nil, errors.New("this SSH key has already been added to your account")
	}

	key := models.SSHKey{
		OwnerID:     userID,
		Name:        name,
		PublicKey:   rawPublicKey,
		Fingerprint: fingerprint,
	}

	if err := s.db.Create(&key).Error; err != nil {
		return nil, err
	}

	return &key, nil
}

func (s *SSHKeyService) RenameKey(userID uuid.UUID, keyID uuid.UUID, newName string) error {
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return errors.New("name cannot be empty")
	}

	res := s.db.Model(&models.SSHKey{}).
		Where("id = ? AND owner_id = ?", keyID, userID).
		Update("name", newName)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("key not found or access denied")
	}
	return nil
}

func (s *SSHKeyService) DeleteKey(userID uuid.UUID, keyID uuid.UUID) error {
	res := s.db.Where("id = ? AND owner_id = ?", keyID, userID).Delete(&models.SSHKey{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("key not found or access denied")
	}
	return nil
}
