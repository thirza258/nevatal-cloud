-- Rollback Migration 000007: AI Infrastructure Orchestrator (Phase 1)
-- Plan statuses written in the new vocabulary are kept; job idempotency keys
-- stay VARCHAR(255) (shrinking could fail on orchestrator keys).

DROP INDEX IF EXISTS idx_jobs_active;
DROP INDEX IF EXISTS uq_jobs_owner_idempotency;
CREATE INDEX IF NOT EXISTS idx_jobs_idempotency ON provisioning_jobs(owner_id, idempotency_key);
ALTER TABLE provisioning_jobs DROP COLUMN IF EXISTS heartbeat_at;

DROP TABLE IF EXISTS resource_relationships CASCADE;
DROP TABLE IF EXISTS ai_execution_events CASCADE;
DROP TABLE IF EXISTS ai_execution_tasks CASCADE;
DROP TABLE IF EXISTS ai_executions CASCADE;
DROP TABLE IF EXISTS ai_plan_revisions CASCADE;

DROP INDEX IF EXISTS idx_ai_approvals_plan;
ALTER TABLE ai_approvals DROP COLUMN IF EXISTS superseded_at;
ALTER TABLE ai_approvals DROP COLUMN IF EXISTS execution_id;
ALTER TABLE ai_approvals DROP COLUMN IF EXISTS consumed_at;
ALTER TABLE ai_approvals DROP COLUMN IF EXISTS consumed;
ALTER TABLE ai_approvals DROP COLUMN IF EXISTS acknowledged_risks;
ALTER TABLE ai_approvals DROP COLUMN IF EXISTS risk_level;
ALTER TABLE ai_approvals DROP COLUMN IF EXISTS spec_hash;
ALTER TABLE ai_approvals DROP COLUMN IF EXISTS plan_version;
