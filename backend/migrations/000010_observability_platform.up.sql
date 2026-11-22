-- Migration 000010: Nevatal Cloud Observability & Operations Platform
--
-- Adds standardized telemetry tables: metrics, events, logs, alert rules,
-- alerts, incidents, and incident-resource associations.

-- 1. Metrics: high-frequency and aggregate telemetry points
CREATE TABLE IF NOT EXISTS observability_metrics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    timestamp TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    metric_name VARCHAR(120) NOT NULL,
    value DOUBLE PRECISION NOT NULL,
    tenant_id UUID REFERENCES users(id) ON DELETE CASCADE,
    resource_type VARCHAR(64) NOT NULL DEFAULT '',
    resource_id UUID,
    unit VARCHAR(32) NOT NULL DEFAULT '',
    labels TEXT NOT NULL DEFAULT '{}'
);
CREATE INDEX IF NOT EXISTS idx_obs_metrics_name_ts ON observability_metrics(metric_name, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_obs_metrics_tenant_ts ON observability_metrics(tenant_id, timestamp DESC) WHERE tenant_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_obs_metrics_resource ON observability_metrics(resource_id, metric_name, timestamp DESC) WHERE resource_id IS NOT NULL;

-- 2. Events: discrete state transitions across VM, hypervisor, jobs, and AI orchestrator
CREATE TABLE IF NOT EXISTS observability_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    timestamp TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    event_type VARCHAR(64) NOT NULL,
    severity VARCHAR(16) NOT NULL DEFAULT 'info',
    actor VARCHAR(120) NOT NULL DEFAULT 'system',
    tenant_id UUID REFERENCES users(id) ON DELETE CASCADE,
    resource_type VARCHAR(64) NOT NULL DEFAULT '',
    resource_id UUID,
    message TEXT NOT NULL,
    metadata TEXT NOT NULL DEFAULT '{}',
    correlation_id VARCHAR(64) NOT NULL DEFAULT '',
    is_internal BOOLEAN NOT NULL DEFAULT false
);
CREATE INDEX IF NOT EXISTS idx_obs_events_tenant_ts ON observability_events(tenant_id, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_obs_events_type_ts ON observability_events(event_type, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_obs_events_correlation ON observability_events(correlation_id) WHERE correlation_id != '';
CREATE INDEX IF NOT EXISTS idx_obs_events_resource ON observability_events(resource_id, timestamp DESC) WHERE resource_id IS NOT NULL;

-- 3. Structured operational logs
CREATE TABLE IF NOT EXISTS observability_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    timestamp TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    level VARCHAR(16) NOT NULL DEFAULT 'INFO',
    service VARCHAR(64) NOT NULL DEFAULT 'api',
    event VARCHAR(64) NOT NULL DEFAULT '',
    tenant_id UUID REFERENCES users(id) ON DELETE CASCADE,
    resource_id UUID,
    job_id UUID,
    correlation_id VARCHAR(64) NOT NULL DEFAULT '',
    error_code VARCHAR(64) NOT NULL DEFAULT '',
    message TEXT NOT NULL,
    metadata TEXT NOT NULL DEFAULT '{}'
);
CREATE INDEX IF NOT EXISTS idx_obs_logs_ts ON observability_logs(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_obs_logs_service_ts ON observability_logs(service, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_obs_logs_level_ts ON observability_logs(level, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_obs_logs_correlation ON observability_logs(correlation_id) WHERE correlation_id != '';
CREATE INDEX IF NOT EXISTS idx_obs_logs_tenant_ts ON observability_logs(tenant_id, timestamp DESC) WHERE tenant_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_obs_logs_resource ON observability_logs(resource_id, timestamp DESC) WHERE resource_id IS NOT NULL;

-- 4. Alert rules configured by customer or platform operators
CREATE TABLE IF NOT EXISTS observability_alert_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(120) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    resource_type VARCHAR(64) NOT NULL DEFAULT 'vm',
    resource_id UUID,
    condition_type VARCHAR(32) NOT NULL DEFAULT 'threshold',
    metric_name VARCHAR(120) NOT NULL DEFAULT '',
    operator VARCHAR(8) NOT NULL DEFAULT '>',
    threshold DOUBLE PRECISION NOT NULL DEFAULT 80.0,
    duration_seconds INTEGER NOT NULL DEFAULT 300,
    severity VARCHAR(16) NOT NULL DEFAULT 'warning',
    channels TEXT NOT NULL DEFAULT '["dashboard"]',
    webhook_url VARCHAR(255) NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_obs_alert_rules_tenant ON observability_alert_rules(tenant_id);
CREATE INDEX IF NOT EXISTS idx_obs_alert_rules_enabled ON observability_alert_rules(enabled);

-- 5. Incident records for major service disruptions and correlated anomalies
CREATE TABLE IF NOT EXISTS observability_incidents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    incident_number VARCHAR(32) UNIQUE NOT NULL,
    title VARCHAR(255) NOT NULL,
    severity VARCHAR(16) NOT NULL DEFAULT 'SEV-2',
    status VARCHAR(32) NOT NULL DEFAULT 'detected',
    root_cause TEXT NOT NULL DEFAULT '',
    symptoms TEXT NOT NULL DEFAULT '[]',
    affected_resources_count INTEGER NOT NULL DEFAULT 0,
    affected_tenants_count INTEGER NOT NULL DEFAULT 0,
    started_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    detected_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    investigating_at TIMESTAMPTZ,
    mitigating_at TIMESTAMPTZ,
    resolved_at TIMESTAMPTZ,
    postmortem TEXT NOT NULL DEFAULT '',
    lead_actor VARCHAR(120) NOT NULL DEFAULT 'system',
    timeline TEXT NOT NULL DEFAULT '[]',
    correlation_id VARCHAR(64) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_obs_incidents_status ON observability_incidents(status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_obs_incidents_number ON observability_incidents(incident_number);

-- 6. Alerts triggered by alert rules or automated system monitors
CREATE TABLE IF NOT EXISTS observability_alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rule_id UUID REFERENCES observability_alert_rules(id) ON DELETE SET NULL,
    incident_id UUID REFERENCES observability_incidents(id) ON DELETE SET NULL,
    tenant_id UUID REFERENCES users(id) ON DELETE CASCADE,
    resource_type VARCHAR(64) NOT NULL DEFAULT 'vm',
    resource_id UUID,
    resource_name VARCHAR(120) NOT NULL DEFAULT '',
    severity VARCHAR(16) NOT NULL DEFAULT 'warning',
    title VARCHAR(255) NOT NULL,
    message TEXT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'triggered',
    correlation_id VARCHAR(64) NOT NULL DEFAULT '',
    triggered_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    acknowledged_at TIMESTAMPTZ,
    acknowledged_by VARCHAR(120) NOT NULL DEFAULT '',
    resolved_at TIMESTAMPTZ,
    resolution TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_obs_alerts_status_ts ON observability_alerts(status, triggered_at DESC);
CREATE INDEX IF NOT EXISTS idx_obs_alerts_tenant_ts ON observability_alerts(tenant_id, triggered_at DESC) WHERE tenant_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_obs_alerts_resource ON observability_alerts(resource_id, triggered_at DESC) WHERE resource_id IS NOT NULL;

-- 7. Correlated resources affected by an incident
CREATE TABLE IF NOT EXISTS observability_incident_resources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    incident_id UUID NOT NULL REFERENCES observability_incidents(id) ON DELETE CASCADE,
    resource_type VARCHAR(64) NOT NULL DEFAULT 'vm',
    resource_id UUID,
    resource_name VARCHAR(120) NOT NULL DEFAULT '',
    tenant_id UUID REFERENCES users(id) ON DELETE SET NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'affected'
);
CREATE INDEX IF NOT EXISTS idx_obs_inc_res_incident ON observability_incident_resources(incident_id);
