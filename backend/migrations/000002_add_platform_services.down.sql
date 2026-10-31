-- Rollback Migration 000002: Add Platform Services

DROP TABLE IF EXISTS ai_assistant_messages CASCADE;
DROP TABLE IF EXISTS storage_shared_links CASCADE;
DROP TABLE IF EXISTS storage_access_keys CASCADE;
DROP TABLE IF EXISTS storage_objects CASCADE;
DROP TABLE IF EXISTS storage_buckets CASCADE;
DROP TABLE IF EXISTS os_images CASCADE;
DROP TABLE IF EXISTS openclaw_backups CASCADE;
DROP TABLE IF EXISTS openclaw_channel_integrations CASCADE;
DROP TABLE IF EXISTS openclaw_instances CASCADE;
DROP TABLE IF EXISTS automation_backups CASCADE;
DROP TABLE IF EXISTS automation_instances CASCADE;
DROP TABLE IF EXISTS git_connections CASCADE;
DROP TABLE IF EXISTS application_environment_variables CASCADE;
DROP TABLE IF EXISTS deployments CASCADE;
DROP TABLE IF EXISTS applications CASCADE;
