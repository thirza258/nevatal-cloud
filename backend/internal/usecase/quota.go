package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/repository"
)

type QuotaUsecase struct {
	quotas       repository.QuotaRepository
	instances    repository.InstanceRepository
	reservations repository.ReservationRepository
}

func NewQuotaUsecase(quotas repository.QuotaRepository, instances repository.InstanceRepository, reservations repository.ReservationRepository) *QuotaUsecase {
	return &QuotaUsecase{quotas: quotas, instances: instances, reservations: reservations}
}

// GetUserQuota returns the user's limits, creating the default quota when the
// user has none.
func (s *QuotaUsecase) GetUserQuota(ctx context.Context, userID uuid.UUID) (*models.ResourceQuota, error) {
	quota, err := s.quotas.FindByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			// Create fallback quota
			quota = &models.ResourceQuota{
				UserID:       userID,
				MaxVCPUs:     4,
				MaxMemoryMB:  8192,
				MaxStorageGB: 100,
				MaxInstances: 3,
			}
			_ = s.quotas.Create(ctx, quota)
			return quota, nil
		}
		return nil, err
	}
	return quota, nil
}

func (s *QuotaUsecase) GetUsage(ctx context.Context, userID uuid.UUID) (*models.QuotaUsage, error) {
	quota, err := s.GetUserQuota(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 1. Sum up all active instances (excluding soft-deleted)
	instSum, err := s.instances.SumUsageByOwner(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 2. Sum up pending active reservations that don't have an instance created yet
	resSum, err := s.reservations.SumActiveByOwner(ctx, userID)
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

func (s *QuotaUsecase) CheckQuota(ctx context.Context, userID uuid.UUID, vcpus, memoryMB, diskGB int) error {
	usage, err := s.GetUsage(ctx, userID)
	if err != nil {
		return err
	}
	if usage.UsedVCPUs+vcpus > usage.MaxVCPUs {
		return fmt.Errorf("exceeds vCPU quota: %d requested, %d available", vcpus, usage.MaxVCPUs-usage.UsedVCPUs)
	}
	if usage.UsedMemoryMB+memoryMB > usage.MaxMemoryMB {
		return fmt.Errorf("exceeds memory quota: %d MB requested, %d MB available", memoryMB, usage.MaxMemoryMB-usage.UsedMemoryMB)
	}
	if usage.UsedStorageGB+diskGB > usage.MaxStorageGB {
		return fmt.Errorf("exceeds storage quota: %d GB requested, %d GB available", diskGB, usage.MaxStorageGB-usage.UsedStorageGB)
	}
	return nil
}

// Reserve checks a new instance against usage (vCPU, memory, storage and
// instance count) and records an active reservation for jobID. Callers read
// usage with GetUsage first; Instance creation does that outside its
// transaction and calls Reserve inside it.
func (s *QuotaUsecase) Reserve(ctx context.Context, usage *models.QuotaUsage, userID uuid.UUID, jobID uuid.UUID, vcpus, memoryMB, diskGB int) (*models.ResourceReservation, error) {
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

	if err := s.reservations.Create(ctx, &reservation); err != nil {
		return nil, fmt.Errorf("failed to create resource reservation: %w", err)
	}

	return &reservation, nil
}

func (s *QuotaUsecase) CommitReservation(ctx context.Context, jobID uuid.UUID) error {
	return s.reservations.CommitByJob(ctx, jobID)
}

func (s *QuotaUsecase) ReleaseReservation(ctx context.Context, jobID uuid.UUID) error {
	return s.reservations.ReleaseByJob(ctx, jobID)
}

// UpdateUserQuota replaces the user's limits, creating the quota row when the
// user has none.
func (s *QuotaUsecase) UpdateUserQuota(ctx context.Context, userID uuid.UUID, maxVCPUs, maxMemoryMB, maxStorageGB, maxInstances int) (*models.ResourceQuota, error) {
	quota, err := s.quotas.FindByUserID(ctx, userID)
	if err != nil {
		quota = &models.ResourceQuota{
			UserID: userID,
		}
	}

	quota.MaxVCPUs = maxVCPUs
	quota.MaxMemoryMB = maxMemoryMB
	quota.MaxStorageGB = maxStorageGB
	quota.MaxInstances = maxInstances

	if err := s.quotas.Save(ctx, quota); err != nil {
		return nil, err
	}
	return quota, nil
}
