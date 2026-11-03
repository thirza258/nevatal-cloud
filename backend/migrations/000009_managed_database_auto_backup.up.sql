-- Migration 000009: automatic managed-database backups
--
-- The backup scheduler claims due databases atomically (UPDATE ... FOR UPDATE
-- SKIP LOCKED) and advances next_backup_at by the configured interval, so any
-- number of API replicas can run the loop without taking duplicate backups.

ALTER TABLE managed_databases
    ADD COLUMN IF NOT EXISTS next_backup_at TIMESTAMP WITH TIME ZONE;

-- The claim query filters on deleted_at IS NULL, status and auto_backup and
-- orders by next_backup_at; this partial index keeps it cheap for large
-- fleets while excluding soft-deleted rows.
CREATE INDEX IF NOT EXISTS idx_managed_databases_auto_backup_due
    ON managed_databases(next_backup_at)
    WHERE deleted_at IS NULL AND auto_backup = true AND status = 'running';
