-- Migration 000010 Rollback: Observability Platform
DROP TABLE IF EXISTS observability_incident_resources CASCADE;
DROP TABLE IF EXISTS observability_alerts CASCADE;
DROP TABLE IF EXISTS observability_incidents CASCADE;
DROP TABLE IF EXISTS observability_alert_rules CASCADE;
DROP TABLE IF EXISTS observability_logs CASCADE;
DROP TABLE IF EXISTS observability_events CASCADE;
DROP TABLE IF EXISTS observability_metrics CASCADE;
