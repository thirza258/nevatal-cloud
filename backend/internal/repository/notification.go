package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// NotificationRepository persists in-app notifications.
type NotificationRepository interface {
	Create(ctx context.Context, n *models.Notification) error
	// CreateOnce inserts n unless the user already has a notification with
	// the same non-empty dedupe_key; inserted reports whether it was stored.
	CreateOnce(ctx context.Context, n *models.Notification) (inserted bool, err error)
	List(ctx context.Context, userID uuid.UUID, unreadOnly bool, limit int) ([]models.Notification, error)
	UnreadCount(ctx context.Context, userID uuid.UUID) (int64, error)
	MarkRead(ctx context.Context, id, userID uuid.UUID) (bool, error)
	MarkAllRead(ctx context.Context, userID uuid.UUID) (int64, error)
	// DeleteOlderThan removes notifications created before the cutoff and
	// reports how many rows were deleted.
	DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}

type notificationRepository struct{ base }

var _ NotificationRepository = (*notificationRepository)(nil)

func NewNotificationRepository(db *gorm.DB) NotificationRepository {
	return &notificationRepository{base{db}}
}

func (r *notificationRepository) Create(ctx context.Context, n *models.Notification) error {
	return r.conn(ctx).Create(n).Error
}

func (r *notificationRepository) CreateOnce(ctx context.Context, n *models.Notification) (bool, error) {
	if n.DedupeKey == "" {
		return true, r.Create(ctx, n)
	}
	// The unique index on (user_id, dedupe_key) is partial (dedupe_key <> ''),
	// so the conflict target must not name its columns: PostgreSQL cannot infer
	// a partial index from a bare column list. Without a target, ON CONFLICT
	// DO NOTHING applies to every unique constraint on the table, which is
	// exactly what a "deliver at most once" notification needs.
	res := r.conn(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(n)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

func (r *notificationRepository) List(ctx context.Context, userID uuid.UUID, unreadOnly bool, limit int) ([]models.Notification, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := r.conn(ctx).Where("user_id = ?", userID)
	if unreadOnly {
		q = q.Where("read_at IS NULL")
	}
	var out []models.Notification
	err := q.Order("created_at desc").Limit(limit).Find(&out).Error
	return out, err
}

func (r *notificationRepository) UnreadCount(ctx context.Context, userID uuid.UUID) (int64, error) {
	var n int64
	err := r.conn(ctx).Model(&models.Notification{}).
		Where("user_id = ? AND read_at IS NULL", userID).Count(&n).Error
	return n, err
}

func (r *notificationRepository) MarkRead(ctx context.Context, id, userID uuid.UUID) (bool, error) {
	res := r.conn(ctx).Model(&models.Notification{}).
		Where("id = ? AND user_id = ? AND read_at IS NULL", id, userID).
		Update("read_at", gorm.Expr("CURRENT_TIMESTAMP"))
	return res.RowsAffected == 1, res.Error
}

func (r *notificationRepository) MarkAllRead(ctx context.Context, userID uuid.UUID) (int64, error) {
	res := r.conn(ctx).Model(&models.Notification{}).
		Where("user_id = ? AND read_at IS NULL", userID).
		Update("read_at", gorm.Expr("CURRENT_TIMESTAMP"))
	return res.RowsAffected, res.Error
}

func (r *notificationRepository) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	res := r.conn(ctx).Where("created_at < ?", cutoff).Delete(&models.Notification{})
	return res.RowsAffected, res.Error
}
