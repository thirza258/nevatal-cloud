package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"gorm.io/gorm"
)

// DatabaseRepository persists managed databases and their logical backups.
type DatabaseRepository interface {
	ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]models.ManagedDatabase, error)
	FindByID(ctx context.Context, id, ownerID uuid.UUID) (*models.ManagedDatabase, error)
	Create(ctx context.Context, d *models.ManagedDatabase) error
	Save(ctx context.Context, d *models.ManagedDatabase) error
	SoftDelete(ctx context.Context, id, ownerID uuid.UUID) error
	// SumResources returns the compute resources of the owner's active
	// databases and their count (soft-deleted rows excluded).
	SumResources(ctx context.Context, ownerID uuid.UUID) (vcpus, memoryMB, diskGB int, count int64, err error)
	CountByOwner(ctx context.Context, ownerID uuid.UUID) (int64, error)

	CreateBackup(ctx context.Context, b *models.ManagedDatabaseBackup) error
	ListBackups(ctx context.Context, databaseID uuid.UUID) ([]models.ManagedDatabaseBackup, error)
	FindBackup(ctx context.Context, id, databaseID uuid.UUID) (*models.ManagedDatabaseBackup, error)
	DeleteBackup(ctx context.Context, id, databaseID uuid.UUID) error
	SetLastBackup(ctx context.Context, databaseID uuid.UUID) error
	// ClaimDueBackups atomically claims up to limit running databases whose
	// automatic backup is due and advances next_backup_at by interval. Rows
	// are locked with FOR UPDATE SKIP LOCKED, so replicas never claim the
	// same database and never take a duplicate backup.
	ClaimDueBackups(ctx context.Context, now time.Time, interval time.Duration, limit int) ([]models.ManagedDatabase, error)
}

type databaseRepository struct{ base }

var _ DatabaseRepository = (*databaseRepository)(nil)

func NewDatabaseRepository(db *gorm.DB) DatabaseRepository {
	return &databaseRepository{base{db}}
}

func (r *databaseRepository) ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]models.ManagedDatabase, error) {
	var out []models.ManagedDatabase
	err := r.conn(ctx).Where("owner_id = ? AND deleted_at IS NULL", ownerID).
		Order("created_at desc").Find(&out).Error
	return out, err
}

func (r *databaseRepository) FindByID(ctx context.Context, id, ownerID uuid.UUID) (*models.ManagedDatabase, error) {
	var d models.ManagedDatabase
	err := r.conn(ctx).Where("id = ? AND owner_id = ? AND deleted_at IS NULL", id, ownerID).First(&d).Error
	if err != nil {
		return nil, mapErr(err)
	}
	return &d, nil
}

func (r *databaseRepository) Create(ctx context.Context, d *models.ManagedDatabase) error {
	err := r.conn(ctx).Create(d).Error
	if uniqueViolation(r.db, err) {
		return ErrDuplicate
	}
	return err
}

func (r *databaseRepository) Save(ctx context.Context, d *models.ManagedDatabase) error {
	return r.conn(ctx).Save(d).Error
}

func (r *databaseRepository) SoftDelete(ctx context.Context, id, ownerID uuid.UUID) error {
	res := r.conn(ctx).Model(&models.ManagedDatabase{}).
		Where("id = ? AND owner_id = ? AND deleted_at IS NULL", id, ownerID).
		Updates(map[string]any{"status": models.DatabaseStatusDeleting, "deleted_at": gorm.Expr("CURRENT_TIMESTAMP")})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *databaseRepository) SumResources(ctx context.Context, ownerID uuid.UUID) (int, int, int, int64, error) {
	var row struct {
		VCPUs     int
		MemoryMB  int
		DiskGB    int
		Instances int64
	}
	err := r.conn(ctx).Model(&models.ManagedDatabase{}).
		Select("COALESCE(SUM(vcpus),0) AS vcpus, COALESCE(SUM(memory_mb),0) AS memory_mb, COALESCE(SUM(storage_gb),0) AS disk_gb, COUNT(*) AS instances").
		Where("owner_id = ? AND deleted_at IS NULL", ownerID).
		Scan(&row).Error
	return row.VCPUs, row.MemoryMB, row.DiskGB, row.Instances, err
}

func (r *databaseRepository) CountByOwner(ctx context.Context, ownerID uuid.UUID) (int64, error) {
	var n int64
	err := r.conn(ctx).Model(&models.ManagedDatabase{}).
		Where("owner_id = ? AND deleted_at IS NULL", ownerID).Count(&n).Error
	return n, err
}

func (r *databaseRepository) CreateBackup(ctx context.Context, b *models.ManagedDatabaseBackup) error {
	return r.conn(ctx).Create(b).Error
}

func (r *databaseRepository) ListBackups(ctx context.Context, databaseID uuid.UUID) ([]models.ManagedDatabaseBackup, error) {
	var out []models.ManagedDatabaseBackup
	err := r.conn(ctx).Where("database_id = ?", databaseID).Order("created_at desc").Find(&out).Error
	return out, err
}

func (r *databaseRepository) FindBackup(ctx context.Context, id, databaseID uuid.UUID) (*models.ManagedDatabaseBackup, error) {
	var b models.ManagedDatabaseBackup
	err := r.conn(ctx).Where("id = ? AND database_id = ?", id, databaseID).First(&b).Error
	if err != nil {
		return nil, mapErr(err)
	}
	return &b, nil
}

func (r *databaseRepository) DeleteBackup(ctx context.Context, id, databaseID uuid.UUID) error {
	res := r.conn(ctx).Where("id = ? AND database_id = ?", id, databaseID).Delete(&models.ManagedDatabaseBackup{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *databaseRepository) SetLastBackup(ctx context.Context, databaseID uuid.UUID) error {
	return r.conn(ctx).Model(&models.ManagedDatabase{}).Where("id = ?", databaseID).
		Update("last_backup_at", gorm.Expr("CURRENT_TIMESTAMP")).Error
}

func (r *databaseRepository) ClaimDueBackups(ctx context.Context, now time.Time, interval time.Duration, limit int) ([]models.ManagedDatabase, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	seconds := int64(interval / time.Second)
	if seconds <= 0 {
		return nil, nil
	}
	var out []models.ManagedDatabase
	err := r.conn(ctx).Raw(`
		UPDATE managed_databases
		SET next_backup_at = CURRENT_TIMESTAMP + (CAST(? AS double precision) * INTERVAL '1 second'),
		    updated_at = CURRENT_TIMESTAMP
		WHERE id IN (
			SELECT id FROM managed_databases
			WHERE deleted_at IS NULL AND status = ? AND auto_backup = true
			  AND (next_backup_at IS NULL OR next_backup_at <= ?)
			ORDER BY COALESCE(next_backup_at, created_at)
			LIMIT ?
			FOR UPDATE SKIP LOCKED
		)
		RETURNING *`, seconds, models.DatabaseStatusRunning, now, limit).Scan(&out).Error
	return out, err
}
