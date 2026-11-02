package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"gorm.io/gorm"
)

// SSHKeyRepository persists users' SSH public keys (ssh_keys).
type SSHKeyRepository interface {
	// ListByOwner returns the owner's keys, newest first.
	ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]models.SSHKey, error)
	// ListByOwnerByName returns the owner's keys sorted by name.
	ListByOwnerByName(ctx context.Context, ownerID uuid.UUID) ([]models.SSHKey, error)
	FindByIDAndOwner(ctx context.Context, id, ownerID uuid.UUID) (*models.SSHKey, error)
	CountByFingerprint(ctx context.Context, ownerID uuid.UUID, fingerprint string) (int64, error)
	Create(ctx context.Context, key *models.SSHKey) error
	// Rename renames the owner's key and reports how many rows changed.
	Rename(ctx context.Context, id, ownerID uuid.UUID, name string) (int64, error)
	// Delete removes the owner's key and reports how many rows were deleted.
	Delete(ctx context.Context, id, ownerID uuid.UUID) (int64, error)
}

type sshKeyRepository struct{ base }

var _ SSHKeyRepository = (*sshKeyRepository)(nil)

func NewSSHKeyRepository(db *gorm.DB) SSHKeyRepository {
	return &sshKeyRepository{base{db}}
}

func (r *sshKeyRepository) ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]models.SSHKey, error) {
	var keys []models.SSHKey
	err := r.conn(ctx).Where("owner_id = ?", ownerID).Order("created_at desc").Find(&keys).Error
	return keys, err
}

func (r *sshKeyRepository) FindByIDAndOwner(ctx context.Context, id, ownerID uuid.UUID) (*models.SSHKey, error) {
	var key models.SSHKey
	if err := r.conn(ctx).First(&key, "id = ? AND owner_id = ?", id, ownerID).Error; err != nil {
		return nil, mapErr(err)
	}
	return &key, nil
}

func (r *sshKeyRepository) CountByFingerprint(ctx context.Context, ownerID uuid.UUID, fingerprint string) (int64, error) {
	var count int64
	err := r.conn(ctx).Model(&models.SSHKey{}).Where("owner_id = ? AND fingerprint = ?", ownerID, fingerprint).Count(&count).Error
	return count, err
}

func (r *sshKeyRepository) Create(ctx context.Context, key *models.SSHKey) error {
	return r.conn(ctx).Create(key).Error
}

func (r *sshKeyRepository) Rename(ctx context.Context, id, ownerID uuid.UUID, name string) (int64, error) {
	res := r.conn(ctx).Model(&models.SSHKey{}).
		Where("id = ? AND owner_id = ?", id, ownerID).
		Update("name", name)
	return res.RowsAffected, res.Error
}

func (r *sshKeyRepository) Delete(ctx context.Context, id, ownerID uuid.UUID) (int64, error) {
	res := r.conn(ctx).Where("id = ? AND owner_id = ?", id, ownerID).Delete(&models.SSHKey{})
	return res.RowsAffected, res.Error
}

func (r *sshKeyRepository) ListByOwnerByName(ctx context.Context, ownerID uuid.UUID) ([]models.SSHKey, error) {
	var keys []models.SSHKey
	err := r.conn(ctx).Where("owner_id = ?", ownerID).Order("name asc").Find(&keys).Error
	return keys, err
}
