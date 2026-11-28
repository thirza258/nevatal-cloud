package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"gorm.io/gorm"
)

type ObservabilityRepository interface {
	// Metrics
	InsertMetrics(ctx context.Context, metrics []models.ObservabilityMetric) error
	QueryMetrics(ctx context.Context, metricName string, tenantID *uuid.UUID, resourceID *uuid.UUID, since time.Time, limit int) ([]models.ObservabilityMetric, error)
	LatestMetric(ctx context.Context, metricName string, tenantID *uuid.UUID, resourceID *uuid.UUID) (*models.ObservabilityMetric, error)

	// Events
	CreateEvent(ctx context.Context, event *models.ObservabilityEvent) error
	QueryEvents(ctx context.Context, tenantID *uuid.UUID, resourceID *uuid.UUID, eventType string, severity string, correlationID string, since time.Time, limit int, offset int) ([]models.ObservabilityEvent, int64, error)

	// Logs
	CreateLog(ctx context.Context, logEntry *models.ObservabilityLog) error
	QueryLogs(ctx context.Context, tenantID *uuid.UUID, resourceID *uuid.UUID, jobID *uuid.UUID, service string, level string, correlationID string, search string, since time.Time, limit int, offset int) ([]models.ObservabilityLog, int64, error)

	// Alert Rules
	CreateAlertRule(ctx context.Context, rule *models.ObservabilityAlertRule) error
	UpdateAlertRule(ctx context.Context, rule *models.ObservabilityAlertRule) error
	DeleteAlertRule(ctx context.Context, id uuid.UUID, tenantID *uuid.UUID) error
	GetAlertRule(ctx context.Context, id uuid.UUID, tenantID *uuid.UUID) (*models.ObservabilityAlertRule, error)
	ListAlertRules(ctx context.Context, tenantID *uuid.UUID, resourceType string, enabledOnly bool) ([]models.ObservabilityAlertRule, error)

	// Alerts
	CreateAlert(ctx context.Context, alert *models.ObservabilityAlert) error
	UpdateAlert(ctx context.Context, alert *models.ObservabilityAlert) error
	GetAlert(ctx context.Context, id uuid.UUID, tenantID *uuid.UUID) (*models.ObservabilityAlert, error)
	ListAlerts(ctx context.Context, tenantID *uuid.UUID, status string, limit int) ([]models.ObservabilityAlert, error)
	CountAlerts(ctx context.Context, tenantID *uuid.UUID, status string) (int64, error)
	AcknowledgeAlert(ctx context.Context, id uuid.UUID, tenantID *uuid.UUID, actor string) error
	ResolveAlert(ctx context.Context, id uuid.UUID, tenantID *uuid.UUID, resolution string) error

	// Incidents
	CreateIncident(ctx context.Context, inc *models.ObservabilityIncident) error
	UpdateIncident(ctx context.Context, inc *models.ObservabilityIncident) error
	GetIncident(ctx context.Context, id uuid.UUID) (*models.ObservabilityIncident, error)
	GetIncidentByNumber(ctx context.Context, number string) (*models.ObservabilityIncident, error)
	ListIncidents(ctx context.Context, status string, limit int) ([]models.ObservabilityIncident, error)
	CountIncidents(ctx context.Context, status string) (int64, error)
	AddIncidentResource(ctx context.Context, res *models.ObservabilityIncidentResource) error

	// AI Executions aggregated telemetry
	CountAIExecutions(ctx context.Context) (total int64, completed int64, failed int64, err error)
}

type observabilityRepository struct{ base }

var _ ObservabilityRepository = (*observabilityRepository)(nil)

func NewObservabilityRepository(db *gorm.DB) ObservabilityRepository {
	return &observabilityRepository{base{db}}
}

func (r *observabilityRepository) InsertMetrics(ctx context.Context, metrics []models.ObservabilityMetric) error {
	if len(metrics) == 0 {
		return nil
	}
	return r.conn(ctx).Create(&metrics).Error
}

func (r *observabilityRepository) QueryMetrics(ctx context.Context, metricName string, tenantID *uuid.UUID, resourceID *uuid.UUID, since time.Time, limit int) ([]models.ObservabilityMetric, error) {
	q := r.conn(ctx).Model(&models.ObservabilityMetric{})
	if metricName != "" {
		q = q.Where("metric_name = ?", metricName)
	}
	if tenantID != nil {
		q = q.Where("tenant_id = ?", *tenantID)
	}
	if resourceID != nil {
		q = q.Where("resource_id = ?", *resourceID)
	}
	if !since.IsZero() {
		q = q.Where("timestamp >= ?", since)
	}
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	var res []models.ObservabilityMetric
	err := q.Order("timestamp ASC").Limit(limit).Find(&res).Error
	return res, err
}

func (r *observabilityRepository) LatestMetric(ctx context.Context, metricName string, tenantID *uuid.UUID, resourceID *uuid.UUID) (*models.ObservabilityMetric, error) {
	q := r.conn(ctx).Model(&models.ObservabilityMetric{}).Where("metric_name = ?", metricName)
	if tenantID != nil {
		q = q.Where("tenant_id = ?", *tenantID)
	}
	if resourceID != nil {
		q = q.Where("resource_id = ?", *resourceID)
	}
	var res models.ObservabilityMetric
	if err := q.Order("timestamp DESC").First(&res).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &res, nil
}

func (r *observabilityRepository) CreateEvent(ctx context.Context, event *models.ObservabilityEvent) error {
	return r.conn(ctx).Create(event).Error
}

func (r *observabilityRepository) QueryEvents(ctx context.Context, tenantID *uuid.UUID, resourceID *uuid.UUID, eventType string, severity string, correlationID string, since time.Time, limit int, offset int) ([]models.ObservabilityEvent, int64, error) {
	q := r.conn(ctx).Model(&models.ObservabilityEvent{})
	if tenantID != nil {
		q = q.Where("tenant_id = ? AND is_internal = false", *tenantID)
	}
	if resourceID != nil {
		q = q.Where("resource_id = ?", *resourceID)
	}
	if eventType != "" {
		q = q.Where("event_type = ?", eventType)
	}
	if severity != "" {
		q = q.Where("severity = ?", severity)
	}
	if correlationID != "" {
		q = q.Where("correlation_id = ?", correlationID)
	}
	if !since.IsZero() {
		q = q.Where("timestamp >= ?", since)
	}

	var count int64
	if err := q.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var events []models.ObservabilityEvent
	err := q.Order("timestamp DESC").Limit(limit).Offset(offset).Find(&events).Error
	return events, count, err
}

func (r *observabilityRepository) CreateLog(ctx context.Context, logEntry *models.ObservabilityLog) error {
	return r.conn(ctx).Create(logEntry).Error
}

func (r *observabilityRepository) QueryLogs(ctx context.Context, tenantID *uuid.UUID, resourceID *uuid.UUID, jobID *uuid.UUID, service string, level string, correlationID string, search string, since time.Time, limit int, offset int) ([]models.ObservabilityLog, int64, error) {
	q := r.conn(ctx).Model(&models.ObservabilityLog{})
	if tenantID != nil {
		q = q.Where("tenant_id = ?", *tenantID)
	}
	if resourceID != nil {
		q = q.Where("resource_id = ?", *resourceID)
	}
	if jobID != nil {
		q = q.Where("job_id = ?", *jobID)
	}
	if service != "" {
		q = q.Where("service = ?", service)
	}
	if level != "" {
		q = q.Where("level = ?", level)
	}
	if correlationID != "" {
		q = q.Where("correlation_id = ?", correlationID)
	}
	if search != "" {
		q = q.Where("message ILIKE ?", "%"+search+"%")
	}
	if !since.IsZero() {
		q = q.Where("timestamp >= ?", since)
	}

	var count int64
	if err := q.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var logs []models.ObservabilityLog
	err := q.Order("timestamp DESC").Limit(limit).Offset(offset).Find(&logs).Error
	return logs, count, err
}

func (r *observabilityRepository) CreateAlertRule(ctx context.Context, rule *models.ObservabilityAlertRule) error {
	return r.conn(ctx).Create(rule).Error
}

func (r *observabilityRepository) UpdateAlertRule(ctx context.Context, rule *models.ObservabilityAlertRule) error {
	return r.conn(ctx).Save(rule).Error
}

func (r *observabilityRepository) DeleteAlertRule(ctx context.Context, id uuid.UUID, tenantID *uuid.UUID) error {
	q := r.conn(ctx).Where("id = ?", id)
	if tenantID != nil {
		q = q.Where("tenant_id = ?", *tenantID)
	}
	return deleteChecked(q.Delete(&models.ObservabilityAlertRule{}))
}

func (r *observabilityRepository) GetAlertRule(ctx context.Context, id uuid.UUID, tenantID *uuid.UUID) (*models.ObservabilityAlertRule, error) {
	var rule models.ObservabilityAlertRule
	q := r.conn(ctx).Where("id = ?", id)
	if tenantID != nil {
		q = q.Where("tenant_id = ?", *tenantID)
	}
	if err := q.First(&rule).Error; err != nil {
		return nil, mapErr(err)
	}
	return &rule, nil
}

func (r *observabilityRepository) ListAlertRules(ctx context.Context, tenantID *uuid.UUID, resourceType string, enabledOnly bool) ([]models.ObservabilityAlertRule, error) {
	q := r.conn(ctx).Model(&models.ObservabilityAlertRule{})
	if tenantID != nil {
		q = q.Where("tenant_id = ? OR tenant_id IS NULL", *tenantID)
	}
	if resourceType != "" {
		q = q.Where("resource_type = ?", resourceType)
	}
	if enabledOnly {
		q = q.Where("enabled = true")
	}
	var rules []models.ObservabilityAlertRule
	err := q.Order("created_at DESC").Find(&rules).Error
	return rules, err
}

func (r *observabilityRepository) CreateAlert(ctx context.Context, alert *models.ObservabilityAlert) error {
	return r.conn(ctx).Create(alert).Error
}

func (r *observabilityRepository) UpdateAlert(ctx context.Context, alert *models.ObservabilityAlert) error {
	return r.conn(ctx).Save(alert).Error
}

func (r *observabilityRepository) GetAlert(ctx context.Context, id uuid.UUID, tenantID *uuid.UUID) (*models.ObservabilityAlert, error) {
	var alert models.ObservabilityAlert
	q := r.conn(ctx).Where("id = ?", id)
	if tenantID != nil {
		q = q.Where("tenant_id = ?", *tenantID)
	}
	if err := q.Preload("Rule").First(&alert).Error; err != nil {
		return nil, mapErr(err)
	}
	return &alert, nil
}

func (r *observabilityRepository) ListAlerts(ctx context.Context, tenantID *uuid.UUID, status string, limit int) ([]models.ObservabilityAlert, error) {
	q := r.conn(ctx).Model(&models.ObservabilityAlert{})
	if tenantID != nil {
		q = q.Where("tenant_id = ?", *tenantID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var alerts []models.ObservabilityAlert
	err := q.Order("triggered_at DESC").Limit(limit).Find(&alerts).Error
	return alerts, err
}

func (r *observabilityRepository) CountAlerts(ctx context.Context, tenantID *uuid.UUID, status string) (int64, error) {
	q := r.conn(ctx).Model(&models.ObservabilityAlert{})
	if tenantID != nil {
		q = q.Where("tenant_id = ?", *tenantID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var count int64
	err := q.Count(&count).Error
	return count, err
}

func (r *observabilityRepository) AcknowledgeAlert(ctx context.Context, id uuid.UUID, tenantID *uuid.UUID, actor string) error {
	now := time.Now()
	q := r.conn(ctx).Model(&models.ObservabilityAlert{}).Where("id = ?", id)
	if tenantID != nil {
		q = q.Where("tenant_id = ?", *tenantID)
	}
	return deleteChecked(q.Updates(map[string]any{
		"status":          models.AlertStatusAcknowledged,
		"acknowledged_at": &now,
		"acknowledged_by": actor,
	}))
}

func (r *observabilityRepository) ResolveAlert(ctx context.Context, id uuid.UUID, tenantID *uuid.UUID, resolution string) error {
	now := time.Now()
	q := r.conn(ctx).Model(&models.ObservabilityAlert{}).Where("id = ?", id)
	if tenantID != nil {
		q = q.Where("tenant_id = ?", *tenantID)
	}
	return deleteChecked(q.Updates(map[string]any{
		"status":      models.AlertStatusResolved,
		"resolved_at": &now,
		"resolution":  resolution,
	}))
}

func (r *observabilityRepository) CreateIncident(ctx context.Context, inc *models.ObservabilityIncident) error {
	return r.conn(ctx).Create(inc).Error
}

func (r *observabilityRepository) UpdateIncident(ctx context.Context, inc *models.ObservabilityIncident) error {
	return r.conn(ctx).Save(inc).Error
}

func (r *observabilityRepository) GetIncident(ctx context.Context, id uuid.UUID) (*models.ObservabilityIncident, error) {
	var inc models.ObservabilityIncident
	if err := r.conn(ctx).Preload("AffectedResources").Where("id = ?", id).First(&inc).Error; err != nil {
		return nil, mapErr(err)
	}
	return &inc, nil
}

func (r *observabilityRepository) GetIncidentByNumber(ctx context.Context, number string) (*models.ObservabilityIncident, error) {
	var inc models.ObservabilityIncident
	if err := r.conn(ctx).Preload("AffectedResources").Where("incident_number = ?", number).First(&inc).Error; err != nil {
		return nil, mapErr(err)
	}
	return &inc, nil
}

func (r *observabilityRepository) ListIncidents(ctx context.Context, status string, limit int) ([]models.ObservabilityIncident, error) {
	q := r.conn(ctx).Model(&models.ObservabilityIncident{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var incs []models.ObservabilityIncident
	err := q.Preload("AffectedResources").Order("created_at DESC").Limit(limit).Find(&incs).Error
	return incs, err
}

func (r *observabilityRepository) CountIncidents(ctx context.Context, status string) (int64, error) {
	q := r.conn(ctx).Model(&models.ObservabilityIncident{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var count int64
	err := q.Count(&count).Error
	return count, err
}

func (r *observabilityRepository) AddIncidentResource(ctx context.Context, res *models.ObservabilityIncidentResource) error {
	return r.conn(ctx).Create(res).Error
}

func (r *observabilityRepository) CountAIExecutions(ctx context.Context) (total int64, completed int64, failed int64, err error) {
	var t, c, f int64
	_ = r.conn(ctx).Table("ai_executions").Count(&t).Error
	_ = r.conn(ctx).Table("ai_executions").Where("status = ?", "completed").Count(&c).Error
	_ = r.conn(ctx).Table("ai_executions").Where("status = ?", "failed").Count(&f).Error
	return t, c, f, nil
}
