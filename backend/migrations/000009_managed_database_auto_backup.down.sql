-- Migration 000009 (down): remove the automatic backup scheduling column.

DROP INDEX IF EXISTS idx_managed_databases_auto_backup_due;
ALTER TABLE managed_databases DROP COLUMN IF EXISTS next_backup_at;
