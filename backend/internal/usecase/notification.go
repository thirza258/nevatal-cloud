package usecase

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/repository"
)

// NotificationUsecase writes and reads in-app notifications. Other use cases
// call Notify (or NotifyOnce with a dedupe key) when something needs the
// user's attention.
type NotificationUsecase struct {
	repo repository.NotificationRepository
}

func NewNotificationUsecase(repo repository.NotificationRepository) *NotificationUsecase {
	return &NotificationUsecase{repo: repo}
}

// NotificationInput is one notification to store.
type NotificationInput struct {
	UserID       uuid.UUID
	Type         string
	Severity     string
	Title        string
	Message      string
	ResourceType string
	ResourceID   *uuid.UUID
	ActionURL    string
	DedupeKey    string
}

// Notify stores the notification (best effort: reporting failures must never
// fail the operation that caused them).
func (s *NotificationUsecase) Notify(ctx context.Context, in NotificationInput) {
	_, _ = s.NotifyOnce(ctx, in)
}

// NotifyOnce stores the notification unless the same user already has one
// with the same non-empty dedupe key. It reports whether it was stored.
func (s *NotificationUsecase) NotifyOnce(ctx context.Context, in NotificationInput) (bool, error) {
	if in.UserID == uuid.Nil || in.Title == "" {
		return false, nil
	}
	if in.Type == "" {
		in.Type = models.NotificationTypeSystem
	}
	if in.Severity == "" {
		in.Severity = models.NotificationSeverityInfo
	}
	n := &models.Notification{
		UserID: in.UserID, Type: in.Type, Severity: in.Severity, Title: in.Title, Message: in.Message,
		ResourceType: in.ResourceType, ResourceID: in.ResourceID, ActionURL: in.ActionURL,
		DedupeKey: in.DedupeKey, CreatedAt: time.Now(),
	}
	return s.repo.CreateOnce(ctx, n)
}

// List returns the user's notifications, newest first.
func (s *NotificationUsecase) List(ctx context.Context, userID uuid.UUID, unreadOnly bool, limit int) ([]models.Notification, error) {
	return s.repo.List(ctx, userID, unreadOnly, limit)
}

// UnreadCount returns the number of unread notifications.
func (s *NotificationUsecase) UnreadCount(ctx context.Context, userID uuid.UUID) (int64, error) {
	return s.repo.UnreadCount(ctx, userID)
}

// MarkRead marks one notification read.
func (s *NotificationUsecase) MarkRead(ctx context.Context, id, userID uuid.UUID) error {
	ok, err := s.repo.MarkRead(ctx, id, userID)
	if err != nil {
		return err
	}
	if !ok {
		return newError(KindNotFound, "NOTIFICATION_NOT_FOUND", "notification not found")
	}
	return nil
}

// MarkAllRead marks every unread notification read and returns how many.
func (s *NotificationUsecase) MarkAllRead(ctx context.Context, userID uuid.UUID) (int64, error) {
	return s.repo.MarkAllRead(ctx, userID)
}

// StartRetention deletes notifications older than retention once at start and
// then hourly, until ctx is cancelled. A non-positive retention disables
// pruning, so rows are kept until removed manually.
func (s *NotificationUsecase) StartRetention(ctx context.Context, retention time.Duration) {
	if retention <= 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		s.Prune(ctx, retention)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.Prune(ctx, retention)
			}
		}
	}()
}

// Prune deletes notifications created before the retention window and returns
// how many rows were removed. Failures are logged and never affect delivery.
func (s *NotificationUsecase) Prune(ctx context.Context, retention time.Duration) int64 {
	if retention <= 0 {
		return 0
	}
	n, err := s.repo.DeleteOlderThan(ctx, time.Now().Add(-retention))
	if err != nil {
		log.Printf("Warning: notification retention failed: %v", err)
		return 0
	}
	if n > 0 {
		log.Printf("Notifications: pruned %d row(s) older than %s", n, retention)
	}
	return n
}
