package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// APIKey represents a customer scoped API authentication key.
type APIKey struct {
	ID         uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	TenantID   uuid.UUID  `gorm:"type:uuid;not null;index" json:"tenant_id"`
	ProjectID  string     `gorm:"type:varchar(64);not null;default:'default';index" json:"project_id"`
	Name       string     `gorm:"type:varchar(120);not null" json:"name"`
	KeyPrefix  string     `gorm:"type:varchar(32);not null" json:"key_prefix"`
	SecretHash string     `gorm:"type:varchar(128);not null;unique" json:"-"`
	Scopes     string     `gorm:"type:text;not null;default:'[]'" json:"scopes"` // JSON array of string scopes
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

func (k *APIKey) BeforeCreate(tx *gorm.DB) error {
	if k.ID == uuid.Nil {
		k.ID = uuid.New()
	}
	return nil
}

func (APIKey) TableName() string { return "api_keys" }

// WebhookEndpoint represents a customer subscription endpoint destination.
type WebhookEndpoint struct {
	ID              uuid.UUID             `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	TenantID        uuid.UUID             `gorm:"type:uuid;not null;index" json:"tenant_id"`
	ProjectID       string                `gorm:"type:varchar(64);not null;default:'default';index" json:"project_id"`
	Name            string                `gorm:"type:varchar(120);not null" json:"name"`
	URL             string                `gorm:"type:varchar(512);not null" json:"url"`
	SecretEncrypted string                `gorm:"type:text;not null" json:"-"`
	Enabled         bool                  `gorm:"not null;default:true" json:"enabled"`
	CreatedAt       time.Time             `json:"created_at"`
	UpdatedAt       time.Time             `json:"updated_at"`
	Subscriptions   []WebhookSubscription `gorm:"foreignKey:EndpointID" json:"subscriptions,omitempty"`
}

func (w *WebhookEndpoint) BeforeCreate(tx *gorm.DB) error {
	if w.ID == uuid.Nil {
		w.ID = uuid.New()
	}
	return nil
}

func (WebhookEndpoint) TableName() string { return "webhook_endpoints" }

// WebhookSubscription links an endpoint to specific domain event types.
type WebhookSubscription struct {
	ID         uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	TenantID   uuid.UUID `gorm:"type:uuid;not null;index" json:"tenant_id"`
	EndpointID uuid.UUID `gorm:"type:uuid;not null;index" json:"endpoint_id"`
	EventType  string    `gorm:"type:varchar(120);not null" json:"event_type"`
	CreatedAt  time.Time `json:"created_at"`
}

func (ws *WebhookSubscription) BeforeCreate(tx *gorm.DB) error {
	if ws.ID == uuid.Nil {
		ws.ID = uuid.New()
	}
	return nil
}

func (WebhookSubscription) TableName() string { return "webhook_subscriptions" }

// WebhookDelivery tracks an individual webhook delivery lifecycle.
type WebhookDelivery struct {
	ID             uuid.UUID                `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	TenantID       uuid.UUID                `gorm:"type:uuid;not null;index" json:"tenant_id"`
	EndpointID     uuid.UUID                `gorm:"type:uuid;not null;index" json:"endpoint_id"`
	EventID        string                   `gorm:"type:varchar(64);not null" json:"event_id"`
	EventType      string                   `gorm:"type:varchar(120);not null" json:"event_type"`
	Payload        string                   `gorm:"type:text;not null" json:"payload"`
	Status         string                   `gorm:"type:varchar(32);not null;default:'pending';index" json:"status"` // pending, delivered, failed
	AttemptCount   int                      `gorm:"not null;default:0" json:"attempt_count"`
	NextAttemptAt  *time.Time               `json:"next_attempt_at,omitempty"`
	LastAttemptAt  *time.Time               `json:"last_attempt_at,omitempty"`
	LastHTTPStatus int                      `gorm:"not null;default:0" json:"last_http_status"`
	LastError      string                   `gorm:"type:text;not null;default:''" json:"last_error"`
	DeliveredAt    *time.Time               `json:"delivered_at,omitempty"`
	CreatedAt      time.Time                `json:"created_at"`
	Attempts       []WebhookDeliveryAttempt `gorm:"foreignKey:DeliveryID" json:"attempts,omitempty"`

	Endpoint *WebhookEndpoint `gorm:"foreignKey:EndpointID" json:"endpoint,omitempty"`
}

func (wd *WebhookDelivery) BeforeCreate(tx *gorm.DB) error {
	if wd.ID == uuid.Nil {
		wd.ID = uuid.New()
	}
	return nil
}

func (WebhookDelivery) TableName() string { return "webhook_deliveries" }

// WebhookDeliveryAttempt records individual delivery HTTP attempts and responses.
type WebhookDeliveryAttempt struct {
	ID             uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	TenantID       uuid.UUID `gorm:"type:uuid;not null;index" json:"tenant_id"`
	DeliveryID     uuid.UUID `gorm:"type:uuid;not null;index" json:"delivery_id"`
	AttemptNumber  int       `gorm:"not null" json:"attempt_number"`
	HTTPStatus     int       `gorm:"not null;default:0" json:"http_status"`
	ResponseTimeMS int       `gorm:"not null;default:0" json:"response_time_ms"`
	ErrorMessage   string    `gorm:"type:text;not null;default:''" json:"error_message"`
	AttemptedAt    time.Time `json:"attempted_at"`
}

func (wda *WebhookDeliveryAttempt) BeforeCreate(tx *gorm.DB) error {
	if wda.ID == uuid.Nil {
		wda.ID = uuid.New()
	}
	if wda.AttemptedAt.IsZero() {
		wda.AttemptedAt = time.Now()
	}
	return nil
}

func (WebhookDeliveryAttempt) TableName() string { return "webhook_delivery_attempts" }

// EventOutbox records infrastructure events transactionally before publication.
type EventOutbox struct {
	ID            uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	TenantID      uuid.UUID  `gorm:"type:uuid;not null;index" json:"tenant_id"`
	ProjectID     string     `gorm:"type:varchar(64);not null;default:'default';index" json:"project_id"`
	EventType     string     `gorm:"type:varchar(120);not null" json:"event_type"`
	AggregateType string     `gorm:"type:varchar(64);not null;default:''" json:"aggregate_type"`
	AggregateID   string     `gorm:"type:varchar(64);not null;default:''" json:"aggregate_id"`
	Payload       string     `gorm:"type:text;not null" json:"payload"`
	CreatedAt     time.Time  `json:"created_at"`
	PublishedAt   *time.Time `gorm:"index" json:"published_at,omitempty"`
}

func (eo *EventOutbox) BeforeCreate(tx *gorm.DB) error {
	if eo.ID == uuid.Nil {
		eo.ID = uuid.New()
	}
	if eo.CreatedAt.IsZero() {
		eo.CreatedAt = time.Now()
	}
	return nil
}

func (EventOutbox) TableName() string { return "event_outbox" }
