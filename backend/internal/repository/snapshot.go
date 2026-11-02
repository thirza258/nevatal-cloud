package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"gorm.io/gorm"
)

// SnapshotRepository persists VM snapshot records (instance_snapshots).
type SnapshotRepository interface {
	// ListByInstance returns the instance's snapshots, newest first.
	ListByInstance(ctx context.Context, instanceID uuid.UUID) ([]models.InstanceSnapshot, error)
	FindByName(ctx context.Context, instanceID uuid.UUID, name string) (*models.InstanceSnapshot, error)
	CountByName(ctx context.Context, instanceID uuid.UUID, name string) (int64, error)
	Create(ctx context.Context, snap *models.InstanceSnapshot) error
	Delete(ctx context.Context, snap *models.InstanceSnapshot) error
}

type snapshotRepository struct{ base }

var _ SnapshotRepository = (*snapshotRepository)(nil)

func NewSnapshotRepository(db *gorm.DB) SnapshotRepository {
	return &snapshotRepository{base{db}}
}

func (r *snapshotRepository) ListByInstance(ctx context.Context, instanceID uuid.UUID) ([]models.InstanceSnapshot, error) {
	var snapshots []models.InstanceSnapshot
	err := r.conn(ctx).Where("instance_id = ?", instanceID).Order("created_at desc").Find(&snapshots).Error
	return snapshots, err
}

func (r *snapshotRepository) FindByName(ctx context.Context, instanceID uuid.UUID, name string) (*models.InstanceSnapshot, error) {
	var snap models.InstanceSnapshot
	if err := r.conn(ctx).Where("instance_id = ? AND name = ?", instanceID, name).First(&snap).Error; err != nil {
		return nil, mapErr(err)
	}
	return &snap, nil
}

func (r *snapshotRepository) CountByName(ctx context.Context, instanceID uuid.UUID, name string) (int64, error) {
	var count int64
	err := r.conn(ctx).Model(&models.InstanceSnapshot{}).Where("instance_id = ? AND name = ?", instanceID, name).Count(&count).Error
	return count, err
}

func (r *snapshotRepository) Create(ctx context.Context, snap *models.InstanceSnapshot) error {
	return r.conn(ctx).Create(snap).Error
}

func (r *snapshotRepository) Delete(ctx context.Context, snap *models.InstanceSnapshot) error {
	return r.conn(ctx).Delete(snap).Error
}
