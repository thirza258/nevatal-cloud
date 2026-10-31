-- Rollback Migration 000005: Drop Waydroid Android Cloud Hosting Tables

DROP TABLE IF EXISTS android_audit_logs CASCADE;
DROP TABLE IF EXISTS android_provisioning_jobs CASCADE;
DROP TABLE IF EXISTS android_network_profiles CASCADE;
DROP TABLE IF EXISTS android_backups CASCADE;
DROP TABLE IF EXISTS android_sessions CASCADE;
DROP TABLE IF EXISTS android_applications CASCADE;
DROP TABLE IF EXISTS android_data_volumes CASCADE;
DROP TABLE IF EXISTS android_instance_resources CASCADE;
DROP TABLE IF EXISTS android_instances CASCADE;
DROP TABLE IF EXISTS android_worker_nodes CASCADE;
DROP TABLE IF EXISTS android_image_versions CASCADE;
DROP TABLE IF EXISTS android_images CASCADE;
