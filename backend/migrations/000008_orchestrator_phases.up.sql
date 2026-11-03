-- Migration 000008: AI Infrastructure Orchestrator Phases 2-4
--
-- Phase 2: reusable infrastructure templates, managed databases and
-- resource-topology edges.
-- Phase 3: in-app notifications and scheduled infrastructure checks.
-- Phase 4: automation policies (notify or propose a plan, never execute).

-- 1. Reusable, parameterized infrastructure templates. Specifications never
--    contain secrets: the use case rejects secret-looking keys.
CREATE TABLE IF NOT EXISTS infra_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(120) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    intent VARCHAR(64) NOT NULL DEFAULT '',
    specification TEXT NOT NULL,
    parameters TEXT NOT NULL DEFAULT '[]',
    version INTEGER NOT NULL DEFAULT 1,
    is_public BOOLEAN NOT NULL DEFAULT false,
    usage_count BIGINT NOT NULL DEFAULT 0,
    validation_status VARCHAR(16) NOT NULL DEFAULT 'valid',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_infra_templates_owner_name
    ON infra_templates(owner_id, lower(name)) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_infra_templates_public ON infra_templates(is_public) WHERE deleted_at IS NULL;

-- 2. Managed databases. Credentials are stored AES-GCM encrypted
--    (password_ciphertext), never in plaintext and never in plans/events.
CREATE TABLE IF NOT EXISTS managed_databases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    engine VARCHAR(32) NOT NULL,
    engine_version VARCHAR(32) NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'provisioning',
    plan_name VARCHAR(32) NOT NULL DEFAULT 'starter',
    vcpus INTEGER NOT NULL DEFAULT 1,
    memory_mb INTEGER NOT NULL DEFAULT 1024,
    storage_gb INTEGER NOT NULL DEFAULT 10,
    host VARCHAR(255) NOT NULL DEFAULT '',
    port INTEGER NOT NULL DEFAULT 5432,
    database_name VARCHAR(100) NOT NULL DEFAULT '',
    username VARCHAR(100) NOT NULL DEFAULT '',
    password_ciphertext TEXT NOT NULL DEFAULT '',
    auto_backup BOOLEAN NOT NULL DEFAULT true,
    last_backup_at TIMESTAMP WITH TIME ZONE,
    last_health_check_at TIMESTAMP WITH TIME ZONE,
    cpu_percent DOUBLE PRECISION NOT NULL DEFAULT 0,
    memory_usage_percent DOUBLE PRECISION NOT NULL DEFAULT 0,
    storage_usage_percent DOUBLE PRECISION NOT NULL DEFAULT 0,
    connection_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_managed_databases_owner_name
    ON managed_databases(owner_id, lower(name)) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_managed_databases_owner_status ON managed_databases(owner_id, status) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS managed_database_backups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    database_id UUID NOT NULL REFERENCES managed_databases(id) ON DELETE CASCADE,
    name VARCHAR(150) NOT NULL,
    backup_reference VARCHAR(255) NOT NULL,
    size_bytes BIGINT NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL DEFAULT 'completed',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_managed_database_backups_db ON managed_database_backups(database_id, created_at DESC);

-- 3. Resource topology: a relationship row can now point at a second
--    resource (source -> target for uses/hosts/depends_on).
ALTER TABLE resource_relationships ADD COLUMN IF NOT EXISTS related_resource_type VARCHAR(32) NOT NULL DEFAULT '';
ALTER TABLE resource_relationships ADD COLUMN IF NOT EXISTS related_resource_id UUID;
ALTER TABLE resource_relationships ADD COLUMN IF NOT EXISTS related_resource_name VARCHAR(255) NOT NULL DEFAULT '';
DROP INDEX IF EXISTS uq_resource_relationships;
CREATE UNIQUE INDEX IF NOT EXISTS uq_resource_relationships
    ON resource_relationships(execution_id, relation, resource_type, resource_id, related_resource_type, related_resource_id);
CREATE INDEX IF NOT EXISTS idx_resource_relationships_owner ON resource_relationships(owner_id, created_at DESC);

-- 4. In-app notifications. dedupe_key makes repeated findings idempotent.
CREATE TABLE IF NOT EXISTS notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type VARCHAR(32) NOT NULL,
    severity VARCHAR(16) NOT NULL DEFAULT 'info',
    title VARCHAR(255) NOT NULL,
    message TEXT NOT NULL DEFAULT '',
    resource_type VARCHAR(32) NOT NULL DEFAULT '',
    resource_id UUID,
    action_url VARCHAR(255) NOT NULL DEFAULT '',
    dedupe_key VARCHAR(255) NOT NULL DEFAULT '',
    read_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_notifications_user_created ON notifications(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_notifications_unread ON notifications(user_id) WHERE read_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_notifications_dedupe ON notifications(user_id, dedupe_key) WHERE dedupe_key <> '';

-- 5. Scheduled infrastructure checks, claimed from Postgres by any replica.
CREATE TABLE IF NOT EXISTS infra_checks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(120) NOT NULL,
    resource_type VARCHAR(32) NOT NULL DEFAULT '',
    resource_id UUID NOT NULL,
    check_type VARCHAR(32) NOT NULL,
    interval_seconds INTEGER NOT NULL DEFAULT 300,
    enabled BOOLEAN NOT NULL DEFAULT true,
    status VARCHAR(16) NOT NULL DEFAULT 'unknown',
    last_result TEXT NOT NULL DEFAULT '{}',
    consecutive_failures INTEGER NOT NULL DEFAULT 0,
    last_run_at TIMESTAMP WITH TIME ZONE,
    next_run_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_infra_checks_resource
    ON infra_checks(owner_id, check_type, resource_type, resource_id);
CREATE INDEX IF NOT EXISTS idx_infra_checks_due ON infra_checks(next_run_at) WHERE enabled = true;

-- 6. Automation policies: deterministic rules evaluated on platform events.
CREATE TABLE IF NOT EXISTS automation_policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(120) NOT NULL,
    trigger VARCHAR(48) NOT NULL,
    conditions TEXT NOT NULL DEFAULT '{}',
    action VARCHAR(32) NOT NULL,
    action_config TEXT NOT NULL DEFAULT '{}',
    enabled BOOLEAN NOT NULL DEFAULT true,
    last_triggered_at TIMESTAMP WITH TIME ZONE,
    trigger_count BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_automation_policies_owner ON automation_policies(owner_id, enabled);
CREATE INDEX IF NOT EXISTS idx_automation_policies_trigger ON automation_policies(trigger) WHERE enabled = true;
