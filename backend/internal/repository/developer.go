package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"gorm.io/gorm"
)

type DeveloperRepository interface {
	CreateAPIKey(ctx context.Context, key *models.APIKey) error
	ListAPIKeysByTenant(ctx context.Context, tenantID uuid.UUID) ([]models.APIKey, error)
	GetAPIKeyByID(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) (*models.APIKey, error)
	FindAPIKeyBySecretHash(ctx context.Context, secretHash string) (*models.APIKey, error)
	UpdateAPIKeyLastUsed(ctx context.Context, id uuid.UUID) error
	RevokeAPIKey(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) error

	CreateWebhookEndpoint(ctx context.Context, ep *models.WebhookEndpoint) error
	ListWebhookEndpointsByTenant(ctx context.Context, tenantID uuid.UUID) ([]models.WebhookEndpoint, error)
	GetWebhookEndpointByID(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) (*models.WebhookEndpoint, error)
	UpdateWebhookEndpoint(ctx context.Context, ep *models.WebhookEndpoint) error
	DeleteWebhookEndpoint(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) error

	AddWebhookSubscription(ctx context.Context, sub *models.WebhookSubscription) error
	DeleteWebhookSubscription(ctx context.Context, tenantID uuid.UUID, endpointID uuid.UUID, eventType string) error

	CreateWebhookDelivery(ctx context.Context, delivery *models.WebhookDelivery) error
	ListWebhookDeliveries(ctx context.Context, tenantID uuid.UUID, endpointID *uuid.UUID, limit int) ([]models.WebhookDelivery, error)
	GetWebhookDeliveryByID(ctx context.Context, tenantID uuid.UUID, deliveryID uuid.UUID) (*models.WebhookDelivery, error)
	UpdateWebhookDelivery(ctx context.Context, delivery *models.WebhookDelivery) error
	RecordDeliveryAttempt(ctx context.Context, attempt *models.WebhookDeliveryAttempt) error

	SaveEventToOutbox(ctx context.Context, event *models.EventOutbox) error
	GetUnpublishedOutboxEvents(ctx context.Context, limit int) ([]models.EventOutbox, error)
	MarkOutboxEventPublished(ctx context.Context, id uuid.UUID) error
}

type developerRepository struct {
	db *gorm.DB
}

func NewDeveloperRepository(db *gorm.DB) DeveloperRepository {
	return &developerRepository{db: db}
}

func (r *developerRepository) CreateAPIKey(ctx context.Context, key *models.APIKey) error {
	return r.db.WithContext(ctx).Create(key).Error
}

func (r *developerRepository) ListAPIKeysByTenant(ctx context.Context, tenantID uuid.UUID) ([]models.APIKey, error) {
	var keys []models.APIKey
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("created_at DESC").
		Find(&keys).Error
	return keys, err
}

func (r *developerRepository) GetAPIKeyByID(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) (*models.APIKey, error) {
	var key models.APIKey
	err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		First(&key).Error
	if err != nil {
		return nil, err
	}
	return &key, nil
}

func (r *developerRepository) FindAPIKeyBySecretHash(ctx context.Context, secretHash string) (*models.APIKey, error) {
	var key models.APIKey
	err := r.db.WithContext(ctx).
		Where("secret_hash = ? AND revoked_at IS NULL", secretHash).
		First(&key).Error
	if err != nil {
		return nil, err
	}
	if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()) {
		return nil, errors.New("api key expired")
	}
	return &key, nil
}

func (r *developerRepository) UpdateAPIKeyLastUsed(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&models.APIKey{}).
		Where("id = ?", id).
		Update("last_used_at", &now).Error
}

func (r *developerRepository) RevokeAPIKey(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&models.APIKey{}).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Update("revoked_at", &now).Error
}

func (r *developerRepository) CreateWebhookEndpoint(ctx context.Context, ep *models.WebhookEndpoint) error {
	return r.db.WithContext(ctx).Create(ep).Error
}

func (r *developerRepository) ListWebhookEndpointsByTenant(ctx context.Context, tenantID uuid.UUID) ([]models.WebhookEndpoint, error) {
	var eps []models.WebhookEndpoint
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Preload("Subscriptions").
		Order("created_at DESC").
		Find(&eps).Error
	return eps, err
}

func (r *developerRepository) GetWebhookEndpointByID(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) (*models.WebhookEndpoint, error) {
	var ep models.WebhookEndpoint
	err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Preload("Subscriptions").
		First(&ep).Error
	if err != nil {
		return nil, err
	}
	return &ep, nil
}

func (r *developerRepository) UpdateWebhookEndpoint(ctx context.Context, ep *models.WebhookEndpoint) error {
	return r.db.WithContext(ctx).Save(ep).Error
}

func (r *developerRepository) DeleteWebhookEndpoint(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&models.WebhookEndpoint{}).Error
}

func (r *developerRepository) AddWebhookSubscription(ctx context.Context, sub *models.WebhookSubscription) error {
	return r.db.WithContext(ctx).Create(sub).Error
}

func (r *developerRepository) DeleteWebhookSubscription(ctx context.Context, tenantID uuid.UUID, endpointID uuid.UUID, eventType string) error {
	return r.db.WithContext(ctx).
		Where("endpoint_id = ? AND event_type = ? AND tenant_id = ?", endpointID, eventType, tenantID).
		Delete(&models.WebhookSubscription{}).Error
}

func (r *developerRepository) CreateWebhookDelivery(ctx context.Context, delivery *models.WebhookDelivery) error {
	return r.db.WithContext(ctx).Create(delivery).Error
}

func (r *developerRepository) ListWebhookDeliveries(ctx context.Context, tenantID uuid.UUID, endpointID *uuid.UUID, limit int) ([]models.WebhookDelivery, error) {
	var deliveries []models.WebhookDelivery
	q := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Preload("Attempts", func(db *gorm.DB) *gorm.DB {
			return db.Order("attempt_number ASC")
		}).
		Preload("Endpoint")

	if endpointID != nil {
		q = q.Where("endpoint_id = ?", *endpointID)
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	err := q.Order("created_at DESC").Limit(limit).Find(&deliveries).Error
	return deliveries, err
}

func (r *developerRepository) GetWebhookDeliveryByID(ctx context.Context, tenantID uuid.UUID, deliveryID uuid.UUID) (*models.WebhookDelivery, error) {
	var delivery models.WebhookDelivery
	err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", deliveryID, tenantID).
		Preload("Attempts", func(db *gorm.DB) *gorm.DB {
			return db.Order("attempt_number ASC")
		}).
		Preload("Endpoint").
		First(&delivery).Error
	if err != nil {
		return nil, err
	}
	return &delivery, nil
}

func (r *developerRepository) UpdateWebhookDelivery(ctx context.Context, delivery *models.WebhookDelivery) error {
	return r.db.WithContext(ctx).Save(delivery).Error
}

func (r *developerRepository) RecordDeliveryAttempt(ctx context.Context, attempt *models.WebhookDeliveryAttempt) error {
	return r.db.WithContext(ctx).Create(attempt).Error
}

func (r *developerRepository) SaveEventToOutbox(ctx context.Context, event *models.EventOutbox) error {
	return r.db.WithContext(ctx).Create(event).Error
}

func (r *developerRepository) GetUnpublishedOutboxEvents(ctx context.Context, limit int) ([]models.EventOutbox, error) {
	var events []models.EventOutbox
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	err := r.db.WithContext(ctx).
		Where("published_at IS NULL").
		Order("created_at ASC").
		Limit(limit).
		Find(&events).Error
	return events, err
}

func (r *developerRepository) MarkOutboxEventPublished(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&models.EventOutbox{}).
		Where("id = ?", id).
		Update("published_at", &now).Error
}
