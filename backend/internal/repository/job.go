package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// JobRepository persists VM provisioning jobs (provisioning_jobs).
type JobRepository interface {
	// FindByID returns a job; with ownerID set, jobs of other owners are not
	// found.
	FindByID(ctx context.Context, id uuid.UUID, ownerID *uuid.UUID) (*models.ProvisioningJob, error)
	FindByIdempotencyKey(ctx context.Context, ownerID uuid.UUID, key string) (*models.ProvisioningJob, error)
	// FindWithInstance returns a job with its Instance preloaded.
	FindWithInstance(ctx context.Context, id uuid.UUID) (*models.ProvisioningJob, error)
	// FindWithInstanceNode returns a job with Instance and Instance.Node
	// preloaded.
	FindWithInstanceNode(ctx context.Context, id uuid.UUID) (*models.ProvisioningJob, error)
	// ListRecentWithInstance returns the newest jobs with Instance preloaded.
	ListRecentWithInstance(ctx context.Context, limit int) ([]models.ProvisioningJob, error)
	CountByStatuses(ctx context.Context, statuses []string) (int64, error)
	CountByStatus(ctx context.Context, status string) (int64, error)

	// Create inserts job. It returns ErrDuplicate when the owner already has
	// a job with the same idempotency key (unique index
	// uq_jobs_owner_idempotency); inside a transaction the transaction is
	// then aborted.
	Create(ctx context.Context, job *models.ProvisioningJob) error
	// FindForUpdate returns a job and locks its row until the transaction in
	// ctx ends (callers use it to make terminal writes conditional).
	FindForUpdate(ctx context.Context, id uuid.UUID) (*models.ProvisioningJob, error)
	// Heartbeat records that the worker running the job is alive. It reports
	// false when the job is no longer queued or in progress (finished, or
	// failed by the stale-job reaper).
	Heartbeat(ctx context.Context, id uuid.UUID) (bool, error)
	// ReapStale fails, in one statement, active jobs whose last sign of life
	// (heartbeat or update) is older than staleAfter, with error code
	// JOB_STALE, and returns them with Instance.Node preloaded. Rows locked
	// by another transaction are skipped, so replicas never reap a job twice.
	ReapStale(ctx context.Context, staleAfter time.Duration, limit int) ([]models.ProvisioningJob, error)
	// Save writes every column of job.
	Save(ctx context.Context, job *models.ProvisioningJob) error
	// SetInstance links job to an instance.
	SetInstance(ctx context.Context, job *models.ProvisioningJob, instanceID uuid.UUID) error
	// UpdateProgress records the current step and percentage and marks the
	// job in progress (only while the job is queued or in progress).
	UpdateProgress(ctx context.Context, job *models.ProvisioningJob, step string, progress int) error
}

type jobRepository struct{ base }

var _ JobRepository = (*jobRepository)(nil)

func NewJobRepository(db *gorm.DB) JobRepository {
	return &jobRepository{base{db}}
}

func (r *jobRepository) FindByID(ctx context.Context, id uuid.UUID, ownerID *uuid.UUID) (*models.ProvisioningJob, error) {
	var job models.ProvisioningJob
	query := r.conn(ctx).Where("id = ?", id)
	if ownerID != nil {
		query = query.Where("owner_id = ?", *ownerID)
	}
	if err := query.First(&job).Error; err != nil {
		return nil, mapErr(err)
	}
	return &job, nil
}

func (r *jobRepository) FindByIdempotencyKey(ctx context.Context, ownerID uuid.UUID, key string) (*models.ProvisioningJob, error) {
	var job models.ProvisioningJob
	if err := r.conn(ctx).Where("owner_id = ? AND idempotency_key = ?", ownerID, key).First(&job).Error; err != nil {
		return nil, mapErr(err)
	}
	return &job, nil
}

func (r *jobRepository) FindWithInstance(ctx context.Context, id uuid.UUID) (*models.ProvisioningJob, error) {
	var job models.ProvisioningJob
	if err := r.conn(ctx).Preload("Instance").First(&job, "id = ?", id).Error; err != nil {
		return nil, mapErr(err)
	}
	return &job, nil
}

func (r *jobRepository) FindWithInstanceNode(ctx context.Context, id uuid.UUID) (*models.ProvisioningJob, error) {
	var job models.ProvisioningJob
	if err := r.conn(ctx).Preload("Instance.Node").First(&job, "id = ?", id).Error; err != nil {
		return nil, mapErr(err)
	}
	return &job, nil
}

func (r *jobRepository) ListRecentWithInstance(ctx context.Context, limit int) ([]models.ProvisioningJob, error) {
	var jobs []models.ProvisioningJob
	err := r.conn(ctx).Preload("Instance").Order("created_at desc").Limit(limit).Find(&jobs).Error
	return jobs, err
}

func (r *jobRepository) CountByStatuses(ctx context.Context, statuses []string) (int64, error) {
	var n int64
	err := r.conn(ctx).Model(&models.ProvisioningJob{}).Where("status IN ?", statuses).Count(&n).Error
	return n, err
}

func (r *jobRepository) CountByStatus(ctx context.Context, status string) (int64, error) {
	var n int64
	err := r.conn(ctx).Model(&models.ProvisioningJob{}).Where("status = ?", status).Count(&n).Error
	return n, err
}

func (r *jobRepository) Create(ctx context.Context, job *models.ProvisioningJob) error {
	err := r.conn(ctx).Create(job).Error
	if uniqueViolation(r.db, err) {
		return ErrDuplicate
	}
	return err
}

func (r *jobRepository) FindForUpdate(ctx context.Context, id uuid.UUID) (*models.ProvisioningJob, error) {
	var job models.ProvisioningJob
	if err := r.conn(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).First(&job, "id = ?", id).Error; err != nil {
		return nil, mapErr(err)
	}
	return &job, nil
}

func (r *jobRepository) Heartbeat(ctx context.Context, id uuid.UUID) (bool, error) {
	res := r.conn(ctx).Exec(`UPDATE provisioning_jobs SET heartbeat_at = now()
		WHERE id = ? AND status IN (?, ?)`, id, models.JobStatusQueued, models.JobStatusInProgress)
	return res.RowsAffected > 0, res.Error
}

func (r *jobRepository) ReapStale(ctx context.Context, staleAfter time.Duration, limit int) ([]models.ProvisioningJob, error) {
	if limit <= 0 {
		limit = 50
	}
	message := fmt.Sprintf("The worker running this job stopped reporting progress for more than %s (API replica restarted or crashed). Resources reserved by the job were released.", staleAfter.Round(time.Second))
	var reaped []models.ProvisioningJob
	err := r.conn(ctx).Raw(`
		UPDATE provisioning_jobs j
		SET status = ?, error_code = ?, error_message = ?, completed_at = now(), updated_at = now()
		WHERE j.id IN (
			SELECT id FROM provisioning_jobs
			WHERE status IN (?, ?)
			  AND GREATEST(COALESCE(heartbeat_at, created_at), COALESCE(updated_at, created_at)) < now() - make_interval(secs => ?)
			ORDER BY created_at
			LIMIT ?
			FOR UPDATE SKIP LOCKED
		)
		RETURNING j.*`,
		models.JobStatusFailed, models.JobErrorStale, message,
		models.JobStatusQueued, models.JobStatusInProgress,
		staleAfter.Seconds(), limit,
	).Scan(&reaped).Error
	if err != nil || len(reaped) == 0 {
		return reaped, err
	}
	// Preload the instance and its node for the cleanup.
	for i := range reaped {
		if reaped[i].InstanceID == nil {
			continue
		}
		var inst models.Instance
		if err := r.conn(ctx).Unscoped().Preload("Node").First(&inst, "id = ?", *reaped[i].InstanceID).Error; err == nil {
			reaped[i].Instance = &inst
		}
	}
	return reaped, nil
}

func (r *jobRepository) Save(ctx context.Context, job *models.ProvisioningJob) error {
	return r.conn(ctx).Save(job).Error
}

func (r *jobRepository) SetInstance(ctx context.Context, job *models.ProvisioningJob, instanceID uuid.UUID) error {
	return r.conn(ctx).Model(job).Update("instance_id", instanceID).Error
}

func (r *jobRepository) UpdateProgress(ctx context.Context, job *models.ProvisioningJob, step string, progress int) error {
	// Only active jobs: a job the stale-job reaper failed stays failed.
	return r.conn(ctx).Model(job).Where("status IN ?", []string{models.JobStatusQueued, models.JobStatusInProgress}).Updates(map[string]interface{}{
		"current_step":     step,
		"progress_percent": progress,
		"status":           models.JobStatusInProgress,
	}).Error
}
