package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"gorm.io/gorm"
)

type QuotaService struct {
	db *gorm.DB
}

func NewQuotaService(db *gorm.DB) *QuotaService {
	return &QuotaService{db: db}
}

func (s *QuotaService) GetUserQuota(userID uuid.UUID) (*models.ResourceQuota, error) {
	var quota models.ResourceQuota
	err := s.db.Where("user_id = ?", userID).First(&quota).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Create fallback quota
			quota = models.ResourceQuota{
				UserID:       userID,
				MaxVCPUs:     4,
				MaxMemoryMB:  8192,
				MaxStorageGB: 100,
				MaxInstances: 3,
			}
			_ = s.db.Create(&quota)
			return &quota, nil
		}
		return nil, err
	}
	return &quota, nil
}

func (s *QuotaService) GetUsage(userID uuid.UUID) (*models.QuotaUsage, error) {
	quota, err := s.GetUserQuota(userID)
	if err != nil {
		return nil, err
	}

	// 1. Sum up all active instances (excluding soft-deleted)
	type ResourceSum struct {
		TotalVCPUs   int
		TotalMemory  int
		TotalStorage int
		Count        int
	}

	var instSum ResourceSum
	err = s.db.Model(&models.Instance{}).
		Where("owner_id = ? AND deleted_at IS NULL", userID).
		Select("COALESCE(SUM(vcpus), 0) as total_vcpus, COALESCE(SUM(memory_mb), 0) as total_memory, COALESCE(SUM(disk_gb), 0) as total_storage, COUNT(id) as count").
		Scan(&instSum).Error
	if err != nil {
		return nil, err
	}

	// 2. Sum up pending active reservations that don't have an instance created yet
	var resSum ResourceSum
	err = s.db.Model(&models.ResourceReservation{}).
		Where("owner_id = ? AND status = ?", userID, models.ReservationActive).
		Select("COALESCE(SUM(reserved_vcpus), 0) as total_vcpus, COALESCE(SUM(reserved_memory_mb), 0) as total_memory, COALESCE(SUM(reserved_storage_gb), 0) as total_storage").
		Scan(&resSum).Error
	if err != nil {
		return nil, err
	}

	return &models.QuotaUsage{
		MaxVCPUs:      quota.MaxVCPUs,
		UsedVCPUs:     instSum.TotalVCPUs + resSum.TotalVCPUs,
		MaxMemoryMB:   quota.MaxMemoryMB,
		UsedMemoryMB:  instSum.TotalMemory + resSum.TotalMemory,
		MaxStorageGB:  quota.MaxStorageGB,
		UsedStorageGB: instSum.TotalStorage + resSum.TotalStorage,
		MaxInstances:  quota.MaxInstances,
		UsedInstances: instSum.Count,
	}, nil
}

func (s *QuotaService) CheckAndReserve(tx *gorm.DB, userID uuid.UUID, jobID uuid.UUID, vcpus, memoryMB, diskGB int) (*models.ResourceReservation, error) {
	usage, err := s.GetUsage(userID)
	if err != nil {
		return nil, err
	}

	if usage.UsedVCPUs+vcpus > usage.MaxVCPUs {
		return nil, fmt.Errorf("exceeds vCPU quota: %d requested, %d available", vcpus, usage.MaxVCPUs-usage.UsedVCPUs)
	}
	if usage.UsedMemoryMB+memoryMB > usage.MaxMemoryMB {
		return nil, fmt.Errorf("exceeds memory quota: %d MB requested, %d MB available", memoryMB, usage.MaxMemoryMB-usage.UsedMemoryMB)
	}
	if usage.UsedStorageGB+diskGB > usage.MaxStorageGB {
		return nil, fmt.Errorf("exceeds storage quota: %d GB requested, %d GB available", diskGB, usage.MaxStorageGB-usage.UsedStorageGB)
	}
	if usage.UsedInstances+1 > usage.MaxInstances {
		return nil, fmt.Errorf("exceeds maximum instance count: limit is %d instances", usage.MaxInstances)
	}

	expiresAt := time.Now().Add(1 * time.Hour)
	reservation := models.ResourceReservation{
		OwnerID:           userID,
		JobID:             jobID,
		ReservedVCPUs:     vcpus,
		ReservedMemoryMB:  memoryMB,
		ReservedStorageGB: diskGB,
		Status:            models.ReservationActive,
		ExpiresAt:         &expiresAt,
	}

	if err := tx.Create(&reservation).Error; err != nil {
		return nil, fmt.Errorf("failed to create resource reservation: %w", err)
	}

	return &reservation, nil
}

func (s *QuotaService) CommitReservation(tx *gorm.DB, jobID uuid.UUID) error {
	return tx.Model(&models.ResourceReservation{}).
		Where("job_id = ? AND status = ?", jobID, models.ReservationActive).
		Update("status", models.ReservationCommitted).Error
}

func (s *QuotaService) ReleaseReservation(tx *gorm.DB, jobID uuid.UUID) error {
	now := time.Now()
	return tx.Model(&models.ResourceReservation{}).
		Where("job_id = ? AND status = ?", jobID, models.ReservationActive).
		Updates(map[string]interface{}{
			"status":      models.ReservationReleased,
			"released_at": &now,
		}).Error
}
