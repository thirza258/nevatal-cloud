-- Migration 000006: Nevatal Domains
-- Customers publish one port of an instance at https://<subdomain>.<PUBLIC_DOMAIN_ROOT>.
-- nevatal-edge reads this table to route requests. Releasing a domain sets
-- released_at; the row is kept so the hostname can be quarantined for other
-- accounts and for history.

CREATE TABLE IF NOT EXISTS domains (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    instance_id UUID REFERENCES instances(id) ON DELETE SET NULL,
    subdomain VARCHAR(63) NOT NULL,
    hostname VARCHAR(253) NOT NULL,
    target_port INTEGER NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    admin_disabled BOOLEAN NOT NULL DEFAULT false,
    disabled_reason VARCHAR(255) NOT NULL DEFAULT '',
    released_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_domains_target_port CHECK (target_port BETWEEN 1 AND 65535),
    CONSTRAINT chk_domains_lowercase CHECK (hostname = lower(hostname) AND subdomain = lower(subdomain))
);

-- At most one active (unreleased) domain per hostname. This index, not an
-- application check, is what makes a claim unique under concurrency.
CREATE UNIQUE INDEX IF NOT EXISTS uq_domains_hostname_active ON domains (lower(hostname)) WHERE released_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_domains_owner ON domains(owner_id);
CREATE INDEX IF NOT EXISTS idx_domains_instance ON domains(instance_id);
-- Quarantine lookups: recent releases of a hostname.
CREATE INDEX IF NOT EXISTS idx_domains_hostname_released ON domains (lower(hostname), released_at) WHERE released_at IS NOT NULL;
