-- Migration 000007: AI Infrastructure Orchestrator (Phase 1)
--
-- The planner proposes a plan, the policy engine validates a revision of it,
-- the user approves that exact revision (version + spec_hash) and the
-- execution engine runs its tasks from these tables. Tasks are leased by API
-- replicas (FOR UPDATE SKIP LOCKED + lease/heartbeat), so a crashed or
-- restarted replica's work is picked up by another one.
--
-- Revision 1 of plans created before this migration is written by the API at
-- startup (the spec hash is computed in Go, never in SQL).

-- 1. Immutable plan revisions. specification/tasks hold canonical JSON; the
--    spec_hash covers exactly these bytes.
CREATE TABLE IF NOT EXISTS ai_plan_revisions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_id UUID NOT NULL REFERENCES ai_resource_plans(id) ON DELETE CASCADE,
    version INTEGER NOT NULL,
    spec_hash VARCHAR(80) NOT NULL,
    created_by VARCHAR(16) NOT NULL DEFAULT 'planner',
    summary TEXT NOT NULL DEFAULT '',
    specification TEXT NOT NULL,
    tasks TEXT NOT NULL,
    estimated_resources TEXT NOT NULL DEFAULT '{}',
    risk_level VARCHAR(16) NOT NULL,
    requires_approval BOOLEAN NOT NULL DEFAULT true,
    confirmation_required TEXT,
    validation TEXT NOT NULL DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_ai_plan_revisions_version UNIQUE (plan_id, version)
);

-- 2. Approvals are bound to one revision, expire, and are consumed by one
--    execution. Approvals recorded before this migration have no binding:
--    the pending ones can never be used and are expired.
ALTER TABLE ai_approvals ADD COLUMN IF NOT EXISTS plan_version INTEGER;
ALTER TABLE ai_approvals ADD COLUMN IF NOT EXISTS spec_hash VARCHAR(80) NOT NULL DEFAULT '';
ALTER TABLE ai_approvals ADD COLUMN IF NOT EXISTS risk_level VARCHAR(16) NOT NULL DEFAULT '';
ALTER TABLE ai_approvals ADD COLUMN IF NOT EXISTS acknowledged_risks BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE ai_approvals ADD COLUMN IF NOT EXISTS consumed BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE ai_approvals ADD COLUMN IF NOT EXISTS consumed_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE ai_approvals ADD COLUMN IF NOT EXISTS execution_id UUID;
ALTER TABLE ai_approvals ADD COLUMN IF NOT EXISTS superseded_at TIMESTAMP WITH TIME ZONE;
UPDATE ai_approvals SET status = 'expired' WHERE status = 'pending' AND plan_version IS NULL;
CREATE INDEX IF NOT EXISTS idx_ai_approvals_plan ON ai_approvals(plan_id, created_at DESC);

-- 3. Plan status vocabulary of the orchestrator contract. Legacy game-server
--    and Android plans were created as 'draft' but were approvable.
--    'provisioning' plans were being provisioned synchronously by a request
--    that no longer exists: their outcome is unknown, they are failed.
UPDATE ai_resource_plans SET status = 'awaiting_approval' WHERE status IN ('awaiting_confirmation', 'draft');
UPDATE ai_resource_plans SET status = 'failed' WHERE status = 'provisioning';

-- 4. Executions of an approved revision.
CREATE TABLE IF NOT EXISTS ai_executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_id UUID NOT NULL REFERENCES ai_resource_plans(id) ON DELETE CASCADE,
    plan_version INTEGER NOT NULL,
    approval_id UUID REFERENCES ai_approvals(id) ON DELETE SET NULL,
    conversation_id UUID NOT NULL REFERENCES ai_conversations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status VARCHAR(32) NOT NULL DEFAULT 'queued',
    cancel_requested BOOLEAN NOT NULL DEFAULT false,
    -- Counter for the per-execution event sequence. Incremented with the
    -- event insert in one transaction: the row lock orders commits by seq.
    last_event_seq BIGINT NOT NULL DEFAULT 0,
    -- Lease for the verification/finalization phase.
    lease_owner VARCHAR(128),
    lease_token UUID,
    lease_expires_at TIMESTAMP WITH TIME ZONE,
    client_ip VARCHAR(64) NOT NULL DEFAULT '',
    report TEXT,
    recovery_options TEXT NOT NULL DEFAULT '[]',
    kept_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at TIMESTAMP WITH TIME ZONE,
    finished_at TIMESTAMP WITH TIME ZONE,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- At most one active execution per plan (EXECUTION_ACTIVE).
CREATE UNIQUE INDEX IF NOT EXISTS uq_ai_executions_plan_active ON ai_executions(plan_id)
    WHERE status IN ('queued', 'running', 'verifying', 'cancelling');
CREATE INDEX IF NOT EXISTS idx_ai_executions_user_status ON ai_executions(user_id, status);
CREATE INDEX IF NOT EXISTS idx_ai_executions_conversation ON ai_executions(conversation_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ai_executions_plan ON ai_executions(plan_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ai_executions_active ON ai_executions(status)
    WHERE status IN ('queued', 'running', 'verifying', 'cancelling');

-- 5. Tasks (DAG nodes). idempotency_key = orch:<execution_id>:<task_key>:<attempt>.
CREATE TABLE IF NOT EXISTS ai_execution_tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id UUID NOT NULL REFERENCES ai_executions(id) ON DELETE CASCADE,
    position INTEGER NOT NULL DEFAULT 0,
    task_key VARCHAR(160) NOT NULL,
    tool VARCHAR(64) NOT NULL,
    title VARCHAR(255) NOT NULL DEFAULT '',
    risk VARCHAR(16) NOT NULL,
    depends_on TEXT NOT NULL DEFAULT '[]',
    input TEXT NOT NULL DEFAULT '{}',
    status VARCHAR(16) NOT NULL,
    attempt INTEGER NOT NULL DEFAULT 1,
    max_attempts INTEGER NOT NULL DEFAULT 3,
    next_attempt_at TIMESTAMP WITH TIME ZONE,
    timeout_seconds INTEGER NOT NULL DEFAULT 300,
    idempotency_key VARCHAR(255) NOT NULL,
    lease_owner VARCHAR(128),
    lease_token UUID,
    lease_expires_at TIMESTAMP WITH TIME ZONE,
    heartbeat_at TIMESTAMP WITH TIME ZONE,
    claim_count INTEGER NOT NULL DEFAULT 0,
    output TEXT,
    error TEXT,
    verification TEXT,
    started_at TIMESTAMP WITH TIME ZONE,
    finished_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_ai_execution_tasks_key UNIQUE (execution_id, task_key)
);
CREATE INDEX IF NOT EXISTS idx_ai_execution_tasks_execution ON ai_execution_tasks(execution_id, position);
CREATE INDEX IF NOT EXISTS idx_ai_execution_tasks_claimable ON ai_execution_tasks(status, next_attempt_at)
    WHERE status IN ('queued', 'retrying');
CREATE INDEX IF NOT EXISTS idx_ai_execution_tasks_lease ON ai_execution_tasks(lease_expires_at)
    WHERE status = 'running';

-- 6. Append-only execution events.
CREATE TABLE IF NOT EXISTS ai_execution_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id UUID NOT NULL REFERENCES ai_executions(id) ON DELETE CASCADE,
    seq BIGINT NOT NULL,
    type VARCHAR(48) NOT NULL,
    severity VARCHAR(16) NOT NULL DEFAULT 'info',
    message TEXT NOT NULL DEFAULT '',
    task_key VARCHAR(160),
    metadata TEXT NOT NULL DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_ai_execution_events_seq UNIQUE (execution_id, seq)
);

-- 7. Which execution created, changed or deleted which resource
--    (recovery "delete_created", reports).
CREATE TABLE IF NOT EXISTS resource_relationships (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    execution_id UUID REFERENCES ai_executions(id) ON DELETE CASCADE,
    task_id UUID REFERENCES ai_execution_tasks(id) ON DELETE SET NULL,
    plan_id UUID REFERENCES ai_resource_plans(id) ON DELETE SET NULL,
    relation VARCHAR(32) NOT NULL,
    resource_type VARCHAR(32) NOT NULL,
    resource_id UUID NOT NULL,
    resource_name VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_resource_relationships ON resource_relationships(execution_id, relation, resource_type, resource_id);
CREATE INDEX IF NOT EXISTS idx_resource_relationships_resource ON resource_relationships(resource_type, resource_id);

-- 8. Provisioning jobs: durable idempotency and heartbeats.
-- Orchestrator keys (orch:<uuid>:<task_key>:<attempt>) exceed 128 characters.
ALTER TABLE provisioning_jobs ALTER COLUMN idempotency_key TYPE VARCHAR(255);
ALTER TABLE provisioning_jobs ADD COLUMN IF NOT EXISTS heartbeat_at TIMESTAMP WITH TIME ZONE;

-- The old index was not unique, so concurrent creates with one key could both
-- insert. Keep the newest job of each (owner, key) and rename the older ones
-- before the unique index is built.
WITH ranked AS (
    SELECT id, ROW_NUMBER() OVER (PARTITION BY owner_id, idempotency_key ORDER BY created_at DESC, id DESC) AS rn
    FROM provisioning_jobs
    WHERE idempotency_key <> ''
)
UPDATE provisioning_jobs j
SET idempotency_key = left(j.idempotency_key, 200) || ':dup:' || j.id::text
FROM ranked r
WHERE j.id = r.id AND r.rn > 1;

DROP INDEX IF EXISTS idx_jobs_idempotency;
CREATE UNIQUE INDEX IF NOT EXISTS uq_jobs_owner_idempotency ON provisioning_jobs(owner_id, idempotency_key)
    WHERE idempotency_key <> '';
-- Stale-job reaper scan.
CREATE INDEX IF NOT EXISTS idx_jobs_active ON provisioning_jobs(status, heartbeat_at)
    WHERE status IN ('queued', 'in_progress');
