package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"gorm.io/gorm"
)

// QuotaRepository persists per-user resource limits (resource_quotas).
type QuotaRepository interface {
	FindByUserID(ctx context.Context, userID uuid.UUID) (*models.ResourceQuota, error)
	Create(ctx context.Context, quota *models.ResourceQuota) error
	// Save inserts the quota when it has no ID yet and updates it otherwise.
	Save(ctx context.Context, quota *models.ResourceQuota) error
}

// ResourceTotals is the result of a resource SUM query. Count is only filled
// by queries that count rows.
//
// Explicit column tags matter: GORM would map TotalVCPUs to "total_v_cpus",
// which never matches the "total_vcpus" alias, leaving used vCPUs at 0 and the
// vCPU quota unenforced.
type ResourceTotals struct {
	TotalVCPUs   int `gorm:"column:total_vcpus"`
	TotalMemory  int `gorm:"column:total_memory"`
	TotalStorage int `gorm:"column:total_storage"`
	Count        int `gorm:"column:count"`
}

type quotaRepository struct{ base }

var _ QuotaRepository = (*quotaRepository)(nil)

func NewQuotaRepository(db *gorm.DB) QuotaRepository {
	return &quotaRepository{base{db}}
}

func (r *quotaRepository) FindByUserID(ctx context.Context, userID uuid.UUID) (*models.ResourceQuota, error) {
	var quota models.ResourceQuota
	if err := r.conn(ctx).Where("user_id = ?", userID).First(&quota).Error; err != nil {
		return nil, mapErr(err)
	}
	return &quota, nil
}

func (r *quotaRepository) Create(ctx context.Context, quota *models.ResourceQuota) error {
	return r.conn(ctx).Create(quota).Error
}

func (r *quotaRepository) Save(ctx context.Context, quota *models.ResourceQuota) error {
	return r.conn(ctx).Save(quota).Error
}
