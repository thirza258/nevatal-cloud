-- 000011_networking_and_developer_platform.down.sql

DROP TABLE IF EXISTS event_outbox;
DROP TABLE IF EXISTS webhook_delivery_attempts;
DROP TABLE IF EXISTS webhook_deliveries;
DROP TABLE IF EXISTS webhook_subscriptions;
DROP TABLE IF EXISTS webhook_endpoints;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS load_balancer_config_revisions;
DROP TABLE IF EXISTS load_balancer_certificates;
DROP TABLE IF EXISTS load_balancer_members;
DROP TABLE IF EXISTS load_balancer_pools;
DROP TABLE IF EXISTS load_balancer_listeners;
DROP TABLE IF EXISTS load_balancers;
DROP TABLE IF EXISTS dns_records;
DROP TABLE IF EXISTS dns_zones;
DROP TABLE IF EXISTS floating_ips;
DROP TABLE IF EXISTS firewall_attachments;
DROP TABLE IF EXISTS firewall_rules;
DROP TABLE IF EXISTS firewalls;
DROP TABLE IF EXISTS network_attachments;
DROP TABLE IF EXISTS networks;
