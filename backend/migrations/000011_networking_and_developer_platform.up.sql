-- 000011_networking_and_developer_platform.up.sql
-- Nevatal Cloud Networking & Developer Platform

-- 1. Private Networks
CREATE TABLE IF NOT EXISTS networks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    project_id VARCHAR(64) NOT NULL DEFAULT 'default',
    name VARCHAR(120) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    cidr VARCHAR(64) NOT NULL,
    gateway VARCHAR(64) NOT NULL DEFAULT '',
    network_type VARCHAR(32) NOT NULL DEFAULT 'private',
    provider VARCHAR(32) NOT NULL DEFAULT 'proxmox_sdn',
    provider_resource_id VARCHAR(120) NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_networks_tenant ON networks(tenant_id);
CREATE INDEX IF NOT EXISTS idx_networks_project ON networks(project_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_networks_tenant_name ON networks(tenant_id, name);

-- 2. Network Attachments
CREATE TABLE IF NOT EXISTS network_attachments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    network_id UUID NOT NULL REFERENCES networks(id) ON DELETE CASCADE,
    instance_id UUID NOT NULL,
    interface_id VARCHAR(32) NOT NULL DEFAULT 'net1',
    ip_address VARCHAR(64) NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'attached',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_network_attachments_tenant ON network_attachments(tenant_id);
CREATE INDEX IF NOT EXISTS idx_network_attachments_network ON network_attachments(network_id);
CREATE INDEX IF NOT EXISTS idx_network_attachments_instance ON network_attachments(instance_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_network_attachments_unique ON network_attachments(network_id, instance_id);

-- 3. Cloud Firewalls
CREATE TABLE IF NOT EXISTS firewalls (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    project_id VARCHAR(64) NOT NULL DEFAULT 'default',
    name VARCHAR(120) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_firewalls_tenant ON firewalls(tenant_id);
CREATE INDEX IF NOT EXISTS idx_firewalls_project ON firewalls(project_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_firewalls_tenant_name ON firewalls(tenant_id, name);

-- 4. Firewall Rules
CREATE TABLE IF NOT EXISTS firewall_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    firewall_id UUID NOT NULL REFERENCES firewalls(id) ON DELETE CASCADE,
    direction VARCHAR(16) NOT NULL DEFAULT 'inbound',
    action VARCHAR(16) NOT NULL DEFAULT 'allow',
    protocol VARCHAR(16) NOT NULL DEFAULT 'tcp',
    source_cidr VARCHAR(64) NOT NULL DEFAULT '0.0.0.0/0',
    destination_cidr VARCHAR(64) NOT NULL DEFAULT '0.0.0.0/0',
    port_min INT NOT NULL DEFAULT 1,
    port_max INT NOT NULL DEFAULT 65535,
    priority INT NOT NULL DEFAULT 100,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_firewall_rules_tenant ON firewall_rules(tenant_id);
CREATE INDEX IF NOT EXISTS idx_firewall_rules_firewall ON firewall_rules(firewall_id);

-- 5. Firewall Attachments
CREATE TABLE IF NOT EXISTS firewall_attachments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    firewall_id UUID NOT NULL REFERENCES firewalls(id) ON DELETE CASCADE,
    resource_type VARCHAR(64) NOT NULL DEFAULT 'vm',
    resource_id UUID NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_firewall_attachments_tenant ON firewall_attachments(tenant_id);
CREATE INDEX IF NOT EXISTS idx_firewall_attachments_firewall ON firewall_attachments(firewall_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_firewall_attachments_unique ON firewall_attachments(firewall_id, resource_type, resource_id);

-- 6. Floating IPs
CREATE TABLE IF NOT EXISTS floating_ips (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    project_id VARCHAR(64) NOT NULL DEFAULT 'default',
    address VARCHAR(64) NOT NULL,
    ip_version INT NOT NULL DEFAULT 4,
    status VARCHAR(32) NOT NULL DEFAULT 'available',
    instance_id UUID,
    provider VARCHAR(32) NOT NULL DEFAULT 'upstream',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_floating_ips_tenant ON floating_ips(tenant_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_floating_ips_address ON floating_ips(address);

-- 7. DNS Zones
CREATE TABLE IF NOT EXISTS dns_zones (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    project_id VARCHAR(64) NOT NULL DEFAULT 'default',
    domain VARCHAR(255) NOT NULL,
    provider VARCHAR(32) NOT NULL DEFAULT 'cloudflare',
    provider_zone_id VARCHAR(120) NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_dns_zones_tenant ON dns_zones(tenant_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_dns_zones_tenant_domain ON dns_zones(tenant_id, domain);

-- 8. DNS Records
CREATE TABLE IF NOT EXISTS dns_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    zone_id UUID NOT NULL REFERENCES dns_zones(id) ON DELETE CASCADE,
    record_type VARCHAR(16) NOT NULL DEFAULT 'A',
    name VARCHAR(255) NOT NULL,
    value TEXT NOT NULL,
    ttl INT NOT NULL DEFAULT 300,
    provider_record_id VARCHAR(120) NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_dns_records_tenant ON dns_records(tenant_id);
CREATE INDEX IF NOT EXISTS idx_dns_records_zone ON dns_records(zone_id);

-- 9. Load Balancers
CREATE TABLE IF NOT EXISTS load_balancers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    project_id VARCHAR(64) NOT NULL DEFAULT 'default',
    name VARCHAR(120) NOT NULL,
    network_id UUID REFERENCES networks(id) ON DELETE SET NULL,
    provider VARCHAR(32) NOT NULL DEFAULT 'haproxy',
    provider_resource_id VARCHAR(120) NOT NULL DEFAULT '',
    public_ip VARCHAR(64) NOT NULL DEFAULT '',
    algorithm VARCHAR(32) NOT NULL DEFAULT 'round_robin',
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    config_version INT NOT NULL DEFAULT 1,
    active_config_version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_load_balancers_tenant ON load_balancers(tenant_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_load_balancers_tenant_name ON load_balancers(tenant_id, name);

-- 10. Load Balancer Listeners
CREATE TABLE IF NOT EXISTS load_balancer_listeners (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    load_balancer_id UUID NOT NULL REFERENCES load_balancers(id) ON DELETE CASCADE,
    protocol VARCHAR(16) NOT NULL DEFAULT 'http',
    frontend_port INT NOT NULL DEFAULT 80,
    certificate_id UUID,
    default_pool_id UUID,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_lb_listeners_lb ON load_balancer_listeners(load_balancer_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_lb_listeners_port ON load_balancer_listeners(load_balancer_id, frontend_port);

-- 11. Load Balancer Pools
CREATE TABLE IF NOT EXISTS load_balancer_pools (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    load_balancer_id UUID NOT NULL REFERENCES load_balancers(id) ON DELETE CASCADE,
    name VARCHAR(120) NOT NULL,
    protocol VARCHAR(16) NOT NULL DEFAULT 'http',
    port INT NOT NULL DEFAULT 8080,
    algorithm VARCHAR(32) NOT NULL DEFAULT 'round_robin',
    health_check_config TEXT NOT NULL DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_lb_pools_lb ON load_balancer_pools(load_balancer_id);

-- 12. Load Balancer Members
CREATE TABLE IF NOT EXISTS load_balancer_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    pool_id UUID NOT NULL REFERENCES load_balancer_pools(id) ON DELETE CASCADE,
    instance_id UUID NOT NULL,
    address VARCHAR(64) NOT NULL DEFAULT '',
    port INT NOT NULL DEFAULT 8080,
    weight INT NOT NULL DEFAULT 100,
    enabled BOOLEAN NOT NULL DEFAULT true,
    health_status VARCHAR(32) NOT NULL DEFAULT 'healthy',
    last_health_check_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_lb_members_pool ON load_balancer_members(pool_id);
CREATE INDEX IF NOT EXISTS idx_lb_members_instance ON load_balancer_members(instance_id);

-- 13. Load Balancer Certificates
CREATE TABLE IF NOT EXISTS load_balancer_certificates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    load_balancer_id UUID REFERENCES load_balancers(id) ON DELETE CASCADE,
    domain VARCHAR(255) NOT NULL,
    certificate_encrypted TEXT NOT NULL,
    private_key_encrypted TEXT NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_lb_certs_tenant ON load_balancer_certificates(tenant_id);

-- 14. Load Balancer Config Revisions
CREATE TABLE IF NOT EXISTS load_balancer_config_revisions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    load_balancer_id UUID NOT NULL REFERENCES load_balancers(id) ON DELETE CASCADE,
    version INT NOT NULL,
    config_hash VARCHAR(64) NOT NULL,
    configuration TEXT NOT NULL,
    deployment_status VARCHAR(32) NOT NULL DEFAULT 'applied',
    error_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_lb_revisions_lb ON load_balancer_config_revisions(load_balancer_id);

-- 15. API Keys
CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    project_id VARCHAR(64) NOT NULL DEFAULT 'default',
    name VARCHAR(120) NOT NULL,
    key_prefix VARCHAR(32) NOT NULL,
    secret_hash VARCHAR(128) NOT NULL,
    scopes TEXT NOT NULL DEFAULT '[]',
    expires_at TIMESTAMP WITH TIME ZONE,
    last_used_at TIMESTAMP WITH TIME ZONE,
    revoked_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_api_keys_tenant ON api_keys(tenant_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_api_keys_hash ON api_keys(secret_hash);

-- 16. Webhook Endpoints
CREATE TABLE IF NOT EXISTS webhook_endpoints (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    project_id VARCHAR(64) NOT NULL DEFAULT 'default',
    name VARCHAR(120) NOT NULL,
    url VARCHAR(512) NOT NULL,
    secret_encrypted TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_webhook_endpoints_tenant ON webhook_endpoints(tenant_id);

-- 17. Webhook Subscriptions
CREATE TABLE IF NOT EXISTS webhook_subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    endpoint_id UUID NOT NULL REFERENCES webhook_endpoints(id) ON DELETE CASCADE,
    event_type VARCHAR(120) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_webhook_sub_endpoint ON webhook_subscriptions(endpoint_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_webhook_sub_unique ON webhook_subscriptions(endpoint_id, event_type);

-- 18. Webhook Deliveries
CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    endpoint_id UUID NOT NULL REFERENCES webhook_endpoints(id) ON DELETE CASCADE,
    event_id VARCHAR(64) NOT NULL,
    event_type VARCHAR(120) NOT NULL,
    payload TEXT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    attempt_count INT NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMP WITH TIME ZONE,
    last_attempt_at TIMESTAMP WITH TIME ZONE,
    last_http_status INT NOT NULL DEFAULT 0,
    last_error TEXT NOT NULL DEFAULT '',
    delivered_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_tenant ON webhook_deliveries(tenant_id);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_endpoint ON webhook_deliveries(endpoint_id);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_status ON webhook_deliveries(status);

-- 19. Webhook Delivery Attempts
CREATE TABLE IF NOT EXISTS webhook_delivery_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    delivery_id UUID NOT NULL REFERENCES webhook_deliveries(id) ON DELETE CASCADE,
    attempt_number INT NOT NULL,
    http_status INT NOT NULL DEFAULT 0,
    response_time_ms INT NOT NULL DEFAULT 0,
    error_message TEXT NOT NULL DEFAULT '',
    attempted_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_webhook_attempts_delivery ON webhook_delivery_attempts(delivery_id);

-- 20. Event Outbox
CREATE TABLE IF NOT EXISTS event_outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    project_id VARCHAR(64) NOT NULL DEFAULT 'default',
    event_type VARCHAR(120) NOT NULL,
    aggregate_type VARCHAR(64) NOT NULL DEFAULT '',
    aggregate_id VARCHAR(64) NOT NULL DEFAULT '',
    payload TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    published_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_event_outbox_tenant ON event_outbox(tenant_id);
CREATE INDEX IF NOT EXISTS idx_event_outbox_unpublished ON event_outbox(published_at) WHERE published_at IS NULL;
