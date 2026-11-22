package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Observability event types
const (
	EventVMCreated             = "VM_CREATED"
	EventVMStarted             = "VM_STARTED"
	EventVMStopped             = "VM_STOPPED"
	EventVMRestarted           = "VM_RESTARTED"
	EventVMDeleted             = "VM_DELETED"
	EventVMIPAssigned          = "VM_IP_ASSIGNED"
	EventProvisioningStarted   = "PROVISIONING_STARTED"
	EventProvisioningFailed    = "PROVISIONING_FAILED"
	EventProvisioningRetrying  = "PROVISIONING_RETRYING"
	EventProvisioningCompleted = "PROVISIONING_COMPLETED"
	EventNodeDegraded          = "NODE_DEGRADED"
	EventNodeRecovered         = "NODE_RECOVERED"
	EventStorageWarning        = "STORAGE_WARNING"
	EventStorageCritical       = "STORAGE_CRITICAL"
	EventUserSuspended         = "USER_SUSPENDED"
	EventQuotaChanged          = "QUOTA_CHANGED"
	EventAIPlanCreated         = "AI_PLAN_CREATED"
	EventAIPlanApproved        = "AI_PLAN_APPROVED"
	EventAIPlanExecuted        = "AI_PLAN_EXECUTED"
	EventAIPlanFailed          = "AI_PLAN_FAILED"
)

// Observability Severities
const (
	SeverityInfo     = "info"
	SeverityWarning  = "warning"
	SeverityError    = "error"
	SeverityCritical = "critical"
)

// Observability Alert Statuses
const (
	AlertStatusTriggered    = "triggered"
	AlertStatusAcknowledged = "acknowledged"
	AlertStatusResolved     = "resolved"
)

// Observability Incident Statuses
const (
	IncidentStatusDetected      = "detected"
	IncidentStatusInvestigating = "investigating"
	IncidentStatusMitigating    = "mitigating"
	IncidentStatusResolved      = "resolved"
	IncidentStatusPostmortem    = "postmortem"
)

// Observability Incident Severities
const (
	IncidentSev1 = "SEV-1"
	IncidentSev2 = "SEV-2"
	IncidentSev3 = "SEV-3"
	IncidentSev4 = "SEV-4"
)

// ObservabilityMetric stores time-series measurements.
type ObservabilityMetric struct {
	ID           uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Timestamp    time.Time  `gorm:"index;not null;default:CURRENT_TIMESTAMP" json:"timestamp"`
	MetricName   string     `gorm:"type:varchar(120);not null;index" json:"metric_name"`
	Value        float64    `gorm:"not null" json:"value"`
	TenantID     *uuid.UUID `gorm:"type:uuid;index" json:"tenant_id,omitempty"`
	ResourceType string     `gorm:"type:varchar(64);not null;default:''" json:"resource_type"`
	ResourceID   *uuid.UUID `gorm:"type:uuid;index" json:"resource_id,omitempty"`
	Unit         string     `gorm:"type:varchar(32);not null;default:''" json:"unit"`
	Labels       string     `gorm:"type:text;not null;default:'{}'" json:"labels"`
}

func (m *ObservabilityMetric) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	if m.Timestamp.IsZero() {
		m.Timestamp = time.Now()
	}
	return nil
}

func (ObservabilityMetric) TableName() string { return "observability_metrics" }

// ObservabilityEvent stores state transitions across the platform.
type ObservabilityEvent struct {
	ID            uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Timestamp     time.Time  `gorm:"index;not null;default:CURRENT_TIMESTAMP" json:"timestamp"`
	EventType     string     `gorm:"type:varchar(64);not null;index" json:"event_type"`
	Severity      string     `gorm:"type:varchar(16);not null;default:'info'" json:"severity"`
	Actor         string     `gorm:"type:varchar(120);not null;default:'system'" json:"actor"`
	TenantID      *uuid.UUID `gorm:"type:uuid;index" json:"tenant_id,omitempty"`
	ResourceType  string     `gorm:"type:varchar(64);not null;default:''" json:"resource_type"`
	ResourceID    *uuid.UUID `gorm:"type:uuid;index" json:"resource_id,omitempty"`
	Message       string     `gorm:"type:text;not null" json:"message"`
	Metadata      string     `gorm:"type:text;not null;default:'{}'" json:"metadata"`
	CorrelationID string     `gorm:"type:varchar(64);not null;default:'';index" json:"correlation_id"`
	IsInternal    bool       `gorm:"not null;default:false" json:"is_internal"`

	Tenant *User `gorm:"foreignKey:TenantID" json:"tenant,omitempty"`
}

func (e *ObservabilityEvent) BeforeCreate(tx *gorm.DB) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
	return nil
}

func (ObservabilityEvent) TableName() string { return "observability_events" }

// ObservabilityLog represents structured operational evidence without secrets.
type ObservabilityLog struct {
	ID            uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Timestamp     time.Time  `gorm:"index;not null;default:CURRENT_TIMESTAMP" json:"timestamp"`
	Level         string     `gorm:"type:varchar(16);not null;default:'INFO';index" json:"level"`
	Service       string     `gorm:"type:varchar(64);not null;default:'api';index" json:"service"`
	Event         string     `gorm:"type:varchar(64);not null;default:''" json:"event"`
	TenantID      *uuid.UUID `gorm:"type:uuid;index" json:"tenant_id,omitempty"`
	ResourceID    *uuid.UUID `gorm:"type:uuid;index" json:"resource_id,omitempty"`
	JobID         *uuid.UUID `gorm:"type:uuid;index" json:"job_id,omitempty"`
	CorrelationID string     `gorm:"type:varchar(64);not null;default:'';index" json:"correlation_id"`
	ErrorCode     string     `gorm:"type:varchar(64);not null;default:''" json:"error_code"`
	Message       string     `gorm:"type:text;not null" json:"message"`
	Metadata      string     `gorm:"type:text;not null;default:'{}'" json:"metadata"`
}

func (l *ObservabilityLog) BeforeCreate(tx *gorm.DB) error {
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	if l.Timestamp.IsZero() {
		l.Timestamp = time.Now()
	}
	return nil
}

func (ObservabilityLog) TableName() string { return "observability_logs" }

// ObservabilityAlertRule represents configurable threshold and condition rules.
type ObservabilityAlertRule struct {
	ID              uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	TenantID        *uuid.UUID `gorm:"type:uuid;index" json:"tenant_id,omitempty"`
	Name            string     `gorm:"type:varchar(120);not null" json:"name"`
	Description     string     `gorm:"type:text;not null;default:''" json:"description"`
	ResourceType    string     `gorm:"type:varchar(64);not null;default:'vm'" json:"resource_type"`
	ResourceID      *uuid.UUID `gorm:"type:uuid" json:"resource_id,omitempty"`
	ConditionType   string     `gorm:"type:varchar(32);not null;default:'threshold'" json:"condition_type"`
	MetricName      string     `gorm:"type:varchar(120);not null;default:''" json:"metric_name"`
	Operator        string     `gorm:"type:varchar(8);not null;default:'>'" json:"operator"`
	Threshold       float64    `gorm:"not null;default:80.0" json:"threshold"`
	DurationSeconds int        `gorm:"not null;default:300" json:"duration_seconds"`
	Severity        string     `gorm:"type:varchar(16);not null;default:'warning'" json:"severity"`
	Channels        string     `gorm:"type:text;not null;default:'[\"dashboard\"]'" json:"channels"`
	WebhookURL      string     `gorm:"type:varchar(255);not null;default:''" json:"webhook_url"`
	Enabled         bool       `gorm:"not null;default:true" json:"enabled"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (r *ObservabilityAlertRule) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

func (ObservabilityAlertRule) TableName() string { return "observability_alert_rules" }

// ObservabilityAlert represents active or historical alert triggers.
type ObservabilityAlert struct {
	ID             uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	RuleID         *uuid.UUID `gorm:"type:uuid" json:"rule_id,omitempty"`
	IncidentID     *uuid.UUID `gorm:"type:uuid" json:"incident_id,omitempty"`
	TenantID       *uuid.UUID `gorm:"type:uuid;index" json:"tenant_id,omitempty"`
	ResourceType   string     `gorm:"type:varchar(64);not null;default:'vm'" json:"resource_type"`
	ResourceID     *uuid.UUID `gorm:"type:uuid;index" json:"resource_id,omitempty"`
	ResourceName   string     `gorm:"type:varchar(120);not null;default:''" json:"resource_name"`
	Severity       string     `gorm:"type:varchar(16);not null;default:'warning'" json:"severity"`
	Title          string     `gorm:"type:varchar(255);not null" json:"title"`
	Message        string     `gorm:"type:text;not null" json:"message"`
	Status         string     `gorm:"type:varchar(32);not null;default:'triggered';index" json:"status"`
	CorrelationID  string     `gorm:"type:varchar(64);not null;default:''" json:"correlation_id"`
	TriggeredAt    time.Time  `gorm:"index;not null;default:CURRENT_TIMESTAMP" json:"triggered_at"`
	AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty"`
	AcknowledgedBy string     `gorm:"type:varchar(120);not null;default:''" json:"acknowledged_by"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
	Resolution     string     `gorm:"type:text;not null;default:''" json:"resolution"`

	Rule *ObservabilityAlertRule `gorm:"foreignKey:RuleID" json:"rule,omitempty"`
}

func (a *ObservabilityAlert) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	if a.TriggeredAt.IsZero() {
		a.TriggeredAt = time.Now()
	}
	return nil
}

func (ObservabilityAlert) TableName() string { return "observability_alerts" }

// ObservabilityIncident tracks operational incidents and correlated blast radius.
type ObservabilityIncident struct {
	ID                     uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	IncidentNumber         string     `gorm:"type:varchar(32);unique;not null" json:"incident_number"`
	Title                  string     `gorm:"type:varchar(255);not null" json:"title"`
	Severity               string     `gorm:"type:varchar(16);not null;default:'SEV-2'" json:"severity"`
	Status                 string     `gorm:"type:varchar(32);not null;default:'detected';index" json:"status"`
	RootCause              string     `gorm:"type:text;not null;default:''" json:"root_cause"`
	Symptoms               string     `gorm:"type:text;not null;default:'[]'" json:"symptoms"`
	AffectedResourcesCount int        `gorm:"not null;default:0" json:"affected_resources_count"`
	AffectedTenantsCount   int        `gorm:"not null;default:0" json:"affected_tenants_count"`
	StartedAt              time.Time  `json:"started_at"`
	DetectedAt             time.Time  `json:"detected_at"`
	InvestigatingAt        *time.Time `json:"investigating_at,omitempty"`
	MitigatingAt           *time.Time `json:"mitigating_at,omitempty"`
	ResolvedAt             *time.Time `json:"resolved_at,omitempty"`
	Postmortem             string     `gorm:"type:text;not null;default:''" json:"postmortem"`
	LeadActor              string     `gorm:"type:varchar(120);not null;default:'system'" json:"lead_actor"`
	Timeline               string     `gorm:"type:text;not null;default:'[]'" json:"timeline"`
	CorrelationID          string     `gorm:"type:varchar(64);not null;default:''" json:"correlation_id"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`

	AffectedResources []ObservabilityIncidentResource `gorm:"foreignKey:IncidentID" json:"affected_resources,omitempty"`
}

func (inc *ObservabilityIncident) BeforeCreate(tx *gorm.DB) error {
	if inc.ID == uuid.Nil {
		inc.ID = uuid.New()
	}
	if inc.StartedAt.IsZero() {
		inc.StartedAt = time.Now()
	}
	if inc.DetectedAt.IsZero() {
		inc.DetectedAt = time.Now()
	}
	return nil
}

func (ObservabilityIncident) TableName() string { return "observability_incidents" }

// ObservabilityIncidentResource maps resources and tenants impacted by an incident.
type ObservabilityIncidentResource struct {
	ID           uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	IncidentID   uuid.UUID  `gorm:"type:uuid;not null;index" json:"incident_id"`
	ResourceType string     `gorm:"type:varchar(64);not null;default:'vm'" json:"resource_type"`
	ResourceID   *uuid.UUID `gorm:"type:uuid" json:"resource_id,omitempty"`
	ResourceName string     `gorm:"type:varchar(120);not null;default:''" json:"resource_name"`
	TenantID     *uuid.UUID `gorm:"type:uuid" json:"tenant_id,omitempty"`
	Status       string     `gorm:"type:varchar(32);not null;default:'affected'" json:"status"`
}

func (ir *ObservabilityIncidentResource) BeforeCreate(tx *gorm.DB) error {
	if ir.ID == uuid.Nil {
		ir.ID = uuid.New()
	}
	return nil
}

func (ObservabilityIncidentResource) TableName() string { return "observability_incident_resources" }
