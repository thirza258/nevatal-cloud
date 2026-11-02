package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"gorm.io/gorm"
)

// InstanceRepository persists compute VMs (instances). Soft-deleted rows are
// excluded from every query.
type InstanceRepository interface {
	// FindByID returns a live instance with Node and OSTemplate preloaded.
	// With ownerID set, instances of other owners are not found.
	FindByID(ctx context.Context, id uuid.UUID, ownerID *uuid.UUID) (*models.Instance, error)
	// ListWithPlacement returns live instances (newest first) with Node and
	// OSTemplate preloaded, optionally filtered by owner and status.
	ListWithPlacement(ctx context.Context, ownerID *uuid.UUID, status string) ([]models.Instance, error)
	// ListByOwnerWithTemplate returns the owner's live instances (newest
	// first) with only OSTemplate preloaded.
	ListByOwnerWithTemplate(ctx context.Context, ownerID uuid.UUID) ([]models.Instance, error)
	// ListAllWithOwner returns every live instance (newest first) with Owner,
	// Node and OSTemplate preloaded.
	ListAllWithOwner(ctx context.Context) ([]models.Instance, error)
	// FindAnyByID is FindByID including soft-deleted instances (Node
	// preloaded).
	FindAnyByID(ctx context.Context, id uuid.UUID, ownerID *uuid.UUID) (*models.Instance, error)
	// IsOwnedBy reports whether the instance belongs to ownerID, including
	// soft-deleted instances.
	IsOwnedBy(ctx context.Context, id, ownerID uuid.UUID) (bool, error)
	CountActive(ctx context.Context) (int64, error)
	CountActiveByStatus(ctx context.Context, status string) (int64, error)
	// SumUsageByOwner totals vCPU, memory and disk of the owner's live
	// instances and counts them.
	SumUsageByOwner(ctx context.Context, ownerID uuid.UUID) (ResourceTotals, error)
	// SumAllocatedVCPUs and SumAllocatedMemory total the vCPUs and memory of
	// every live instance that is not stopped (all tenants).
	SumAllocatedVCPUs(ctx context.Context) (int64, error)
	SumAllocatedMemory(ctx context.Context) (int64, error)

	Create(ctx context.Context, inst *models.Instance) error
	// Save writes every column of inst.
	Save(ctx context.Context, inst *models.Instance) error
	// UpdateStatus sets the status column of inst (and inst.Status).
	UpdateStatus(ctx context.Context, inst *models.Instance, status string) error
	// UpdateStatusByID sets the status of the instance with the given id.
	UpdateStatusByID(ctx context.Context, id uuid.UUID, status string) error
	// UpdateVMID sets the proxmox_vmid column of inst.
	UpdateVMID(ctx context.Context, inst *models.Instance, vmid int) error
	// UpdatePrivateIP sets only the private_ip column of a live instance.
	UpdatePrivateIP(ctx context.Context, id uuid.UUID, ip string) error
	// ListPrivateIPs returns the private address of every live instance
	// that has one (all tenants, duplicates kept).
	ListPrivateIPs(ctx context.Context) ([]string, error)
	// SoftDelete marks inst deleted.
	SoftDelete(ctx context.Context, inst *models.Instance) error
}

type instanceRepository struct{ base }

var _ InstanceRepository = (*instanceRepository)(nil)

func NewInstanceRepository(db *gorm.DB) InstanceRepository {
	return &instanceRepository{base{db}}
}

func (r *instanceRepository) FindByID(ctx context.Context, id uuid.UUID, ownerID *uuid.UUID) (*models.Instance, error) {
	var inst models.Instance
	query := r.conn(ctx).Preload("Node").Preload("OSTemplate").Where("id = ? AND deleted_at IS NULL", id)
	if ownerID != nil {
		query = query.Where("owner_id = ?", *ownerID)
	}
	if err := query.First(&inst).Error; err != nil {
		return nil, mapErr(err)
	}
	return &inst, nil
}

func (r *instanceRepository) FindAnyByID(ctx context.Context, id uuid.UUID, ownerID *uuid.UUID) (*models.Instance, error) {
	var inst models.Instance
	query := r.conn(ctx).Unscoped().Preload("Node").Where("id = ?", id)
	if ownerID != nil {
		query = query.Where("owner_id = ?", *ownerID)
	}
	if err := query.First(&inst).Error; err != nil {
		return nil, mapErr(err)
	}
	return &inst, nil
}

func (r *instanceRepository) ListWithPlacement(ctx context.Context, ownerID *uuid.UUID, status string) ([]models.Instance, error) {
	query := r.conn(ctx).Model(&models.Instance{}).
		Preload("Node").
		Preload("OSTemplate").
		Where("deleted_at IS NULL")
	if ownerID != nil {
		query = query.Where("owner_id = ?", *ownerID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var instances []models.Instance
	err := query.Order("created_at desc").Find(&instances).Error
	return instances, err
}

func (r *instanceRepository) ListByOwnerWithTemplate(ctx context.Context, ownerID uuid.UUID) ([]models.Instance, error) {
	var instances []models.Instance
	err := r.conn(ctx).Model(&models.Instance{}).
		Preload("OSTemplate").
		Where("owner_id = ? AND deleted_at IS NULL", ownerID).
		Order("created_at desc").
		Find(&instances).Error
	return instances, err
}

func (r *instanceRepository) ListAllWithOwner(ctx context.Context) ([]models.Instance, error) {
	var instances []models.Instance
	err := r.conn(ctx).Preload("Owner").Preload("Node").Preload("OSTemplate").
		Where("deleted_at IS NULL").
		Order("created_at desc").
		Find(&instances).Error
	return instances, err
}

func (r *instanceRepository) CountActive(ctx context.Context) (int64, error) {
	var n int64
	err := r.conn(ctx).Model(&models.Instance{}).Where("deleted_at IS NULL").Count(&n).Error
	return n, err
}

func (r *instanceRepository) CountActiveByStatus(ctx context.Context, status string) (int64, error) {
	var n int64
	err := r.conn(ctx).Model(&models.Instance{}).Where("status = ? AND deleted_at IS NULL", status).Count(&n).Error
	return n, err
}

func (r *instanceRepository) SumUsageByOwner(ctx context.Context, ownerID uuid.UUID) (ResourceTotals, error) {
	var sum ResourceTotals
	err := r.conn(ctx).Model(&models.Instance{}).
		Where("owner_id = ? AND deleted_at IS NULL", ownerID).
		Select("COALESCE(SUM(vcpus), 0) as total_vcpus, COALESCE(SUM(memory_mb), 0) as total_memory, COALESCE(SUM(disk_gb), 0) as total_storage, COUNT(id) as count").
		Scan(&sum).Error
	return sum, err
}

func (r *instanceRepository) Create(ctx context.Context, inst *models.Instance) error {
	return r.conn(ctx).Create(inst).Error
}

func (r *instanceRepository) Save(ctx context.Context, inst *models.Instance) error {
	return r.conn(ctx).Save(inst).Error
}

func (r *instanceRepository) UpdateStatus(ctx context.Context, inst *models.Instance, status string) error {
	return r.conn(ctx).Model(inst).Update("status", status).Error
}

func (r *instanceRepository) UpdateStatusByID(ctx context.Context, id uuid.UUID, status string) error {
	return r.conn(ctx).Model(&models.Instance{}).Where("id = ?", id).Update("status", status).Error
}

func (r *instanceRepository) UpdateVMID(ctx context.Context, inst *models.Instance, vmid int) error {
	return r.conn(ctx).Model(inst).Update("proxmox_vmid", vmid).Error
}

func (r *instanceRepository) UpdatePrivateIP(ctx context.Context, id uuid.UUID, ip string) error {
	return r.conn(ctx).Model(&models.Instance{}).Where("id = ? AND deleted_at IS NULL", id).Update("private_ip", ip).Error
}

func (r *instanceRepository) ListPrivateIPs(ctx context.Context) ([]string, error) {
	var ips []string
	err := r.conn(ctx).Model(&models.Instance{}).
		Where("deleted_at IS NULL AND private_ip IS NOT NULL AND private_ip <> ''").
		Pluck("private_ip", &ips).Error
	return ips, err
}

func (r *instanceRepository) SoftDelete(ctx context.Context, inst *models.Instance) error {
	return r.conn(ctx).Delete(inst).Error
}

func (r *instanceRepository) SumAllocatedVCPUs(ctx context.Context) (int64, error) {
	var total int64
	err := r.conn(ctx).Model(&models.Instance{}).Where("status != ?", "stopped").Select("coalesce(sum(vcpus), 0)").Scan(&total).Error
	return total, err
}

func (r *instanceRepository) SumAllocatedMemory(ctx context.Context) (int64, error) {
	var total int64
	err := r.conn(ctx).Model(&models.Instance{}).Where("status != ?", "stopped").Select("coalesce(sum(memory_mb), 0)").Scan(&total).Error
	return total, err
}

func (r *instanceRepository) IsOwnedBy(ctx context.Context, id, ownerID uuid.UUID) (bool, error) {
	var n int64
	err := r.conn(ctx).Unscoped().Model(&models.Instance{}).Where("id = ? AND owner_id = ?", id, ownerID).Count(&n).Error
	return n > 0, err
}
