-- Rollback of migration 000008.
DROP INDEX IF EXISTS idx_automation_policies_trigger;
DROP INDEX IF EXISTS idx_automation_policies_owner;
DROP TABLE IF EXISTS automation_policies;

DROP INDEX IF EXISTS idx_infra_checks_due;
DROP INDEX IF EXISTS uq_infra_checks_resource;
DROP TABLE IF EXISTS infra_checks;

DROP INDEX IF EXISTS uq_notifications_dedupe;
DROP INDEX IF EXISTS idx_notifications_unread;
DROP INDEX IF EXISTS idx_notifications_user_created;
DROP TABLE IF EXISTS notifications;

DROP INDEX IF EXISTS idx_resource_relationships_owner;
DROP INDEX IF EXISTS uq_resource_relationships;
ALTER TABLE resource_relationships DROP COLUMN IF EXISTS related_resource_name;
ALTER TABLE resource_relationships DROP COLUMN IF EXISTS related_resource_id;
ALTER TABLE resource_relationships DROP COLUMN IF EXISTS related_resource_type;
CREATE UNIQUE INDEX IF NOT EXISTS uq_resource_relationships
    ON resource_relationships(execution_id, relation, resource_type, resource_id);

DROP INDEX IF EXISTS idx_managed_database_backups_db;
DROP TABLE IF EXISTS managed_database_backups;
DROP INDEX IF EXISTS idx_managed_databases_owner_status;
DROP INDEX IF EXISTS uq_managed_databases_owner_name;
DROP TABLE IF EXISTS managed_databases;

DROP INDEX IF EXISTS idx_infra_templates_public;
DROP INDEX IF EXISTS uq_infra_templates_owner_name;
DROP TABLE IF EXISTS infra_templates;
