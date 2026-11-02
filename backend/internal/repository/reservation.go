package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"gorm.io/gorm"
)

// ReservationRepository persists quota reservations held by provisioning
// jobs (resource_reservations).
type ReservationRepository interface {
	Create(ctx context.Context, reservation *models.ResourceReservation) error
	// SumActiveByOwner totals the resources of the owner's active
	// reservations (Count is not filled).
	SumActiveByOwner(ctx context.Context, ownerID uuid.UUID) (ResourceTotals, error)
	// CommitByJob marks the job's active reservations committed.
	CommitByJob(ctx context.Context, jobID uuid.UUID) error
	// ReleaseByJob marks the job's active reservations released.
	ReleaseByJob(ctx context.Context, jobID uuid.UUID) error
}

type reservationRepository struct{ base }

var _ ReservationRepository = (*reservationRepository)(nil)

func NewReservationRepository(db *gorm.DB) ReservationRepository {
	return &reservationRepository{base{db}}
}

func (r *reservationRepository) Create(ctx context.Context, reservation *models.ResourceReservation) error {
	return r.conn(ctx).Create(reservation).Error
}

func (r *reservationRepository) SumActiveByOwner(ctx context.Context, ownerID uuid.UUID) (ResourceTotals, error) {
	var sum ResourceTotals
	err := r.conn(ctx).Model(&models.ResourceReservation{}).
		Where("owner_id = ? AND status = ?", ownerID, models.ReservationActive).
		Select("COALESCE(SUM(reserved_vcpus), 0) as total_vcpus, COALESCE(SUM(reserved_memory_mb), 0) as total_memory, COALESCE(SUM(reserved_storage_gb), 0) as total_storage").
		Scan(&sum).Error
	return sum, err
}

func (r *reservationRepository) CommitByJob(ctx context.Context, jobID uuid.UUID) error {
	return r.conn(ctx).Model(&models.ResourceReservation{}).
		Where("job_id = ? AND status = ?", jobID, models.ReservationActive).
		Update("status", models.ReservationCommitted).Error
}

func (r *reservationRepository) ReleaseByJob(ctx context.Context, jobID uuid.UUID) error {
	now := time.Now()
	return r.conn(ctx).Model(&models.ResourceReservation{}).
		Where("job_id = ? AND status = ?", jobID, models.ReservationActive).
		Updates(map[string]interface{}{
			"status":      models.ReservationReleased,
			"released_at": &now,
		}).Error
}
