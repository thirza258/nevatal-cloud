-- Nevatal Cloud Database Schema
-- Matches Product Requirements Document (PRD) v1.0 Section 19

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Table: users
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    display_name VARCHAR(100) NOT NULL,
    role VARCHAR(32) NOT NULL DEFAULT 'customer', -- 'customer', 'admin'
    status VARCHAR(32) NOT NULL DEFAULT 'active', -- 'active', 'suspended'
    email_verified_at TIMESTAMP WITH TIME ZONE,
    last_login_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Table: resource_quotas
CREATE TABLE IF NOT EXISTS resource_quotas (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    max_vcpus INTEGER NOT NULL DEFAULT 4,
    max_memory_mb INTEGER NOT NULL DEFAULT 8192,
    max_storage_gb INTEGER NOT NULL DEFAULT 100,
    max_instances INTEGER NOT NULL DEFAULT 3,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_resource_quotas_user UNIQUE (user_id)
);

-- Table: nodes (Hypervisor nodes, e.g. Dell PowerEdge R640)
CREATE TABLE IF NOT EXISTS nodes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    proxmox_node_name VARCHAR(100) NOT NULL UNIQUE,
    status VARCHAR(32) NOT NULL DEFAULT 'online', -- 'online', 'maintenance', 'offline'
    total_cpu_threads INTEGER NOT NULL DEFAULT 64,
    total_memory_mb INTEGER NOT NULL DEFAULT 262144, -- 256 GB
    total_disk_gb INTEGER NOT NULL DEFAULT 3840,     -- ~3.8 TB
    last_health_check_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Table: os_templates
CREATE TABLE IF NOT EXISTS os_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    distribution VARCHAR(50) NOT NULL,
    version VARCHAR(50) NOT NULL,
    description TEXT,
    proxmox_template_vmid INTEGER NOT NULL,
    architecture VARCHAR(32) NOT NULL DEFAULT 'x86_64',
    cloud_init_supported BOOLEAN NOT NULL DEFAULT true,
    default_username VARCHAR(50) NOT NULL DEFAULT 'ubuntu',
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Table: resource_plans
CREATE TABLE IF NOT EXISTS resource_plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    description TEXT,
    vcpus INTEGER NOT NULL,
    memory_mb INTEGER NOT NULL,
    disk_gb INTEGER NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Table: ssh_keys
CREATE TABLE IF NOT EXISTS ssh_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    public_key TEXT NOT NULL,
    fingerprint VARCHAR(100) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_ssh_keys_owner ON ssh_keys(owner_id);

-- Table: instances
CREATE TABLE IF NOT EXISTS instances (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    node_id UUID NOT NULL REFERENCES nodes(id) ON DELETE RESTRICT,
    os_template_id UUID NOT NULL REFERENCES os_templates(id) ON DELETE RESTRICT,
    proxmox_vmid INTEGER NOT NULL,
    name VARCHAR(100) NOT NULL,
    hostname VARCHAR(100) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'queued', -- queued, provisioning, starting, running, stopping, stopped, restarting, deleting, failed, unknown
    vcpus INTEGER NOT NULL,
    memory_mb INTEGER NOT NULL,
    disk_gb INTEGER NOT NULL,
    private_ip VARCHAR(45),
    public_ip VARCHAR(45),
    ssh_username VARCHAR(50) NOT NULL DEFAULT 'ubuntu',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);
CREATE INDEX IF NOT EXISTS idx_instances_owner ON instances(owner_id);
CREATE INDEX IF NOT EXISTS idx_instances_node ON instances(node_id);
CREATE INDEX IF NOT EXISTS idx_instances_status ON instances(status);
CREATE INDEX IF NOT EXISTS idx_instances_proxmox_vmid ON instances(proxmox_vmid);

-- Table: instance_snapshots
CREATE TABLE IF NOT EXISTS instance_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    instance_id UUID NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    includes_ram BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_snapshots_instance ON instance_snapshots(instance_id);

-- Table: provisioning_jobs
CREATE TABLE IF NOT EXISTS provisioning_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    instance_id UUID REFERENCES instances(id) ON DELETE SET NULL,
    idempotency_key VARCHAR(128) NOT NULL,
    operation_type VARCHAR(32) NOT NULL, -- create, delete, start, shutdown, restart
    status VARCHAR(32) NOT NULL DEFAULT 'queued', -- queued, in_progress, completed, failed
    current_step VARCHAR(64) DEFAULT 'pending',
    progress_percent INTEGER DEFAULT 0,
    error_code VARCHAR(64),
    error_message TEXT,
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_jobs_owner ON provisioning_jobs(owner_id);
CREATE INDEX IF NOT EXISTS idx_jobs_status ON provisioning_jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_idempotency ON provisioning_jobs(owner_id, idempotency_key);

-- Table: resource_reservations
CREATE TABLE IF NOT EXISTS resource_reservations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    job_id UUID NOT NULL REFERENCES provisioning_jobs(id) ON DELETE CASCADE,
    node_id UUID REFERENCES nodes(id) ON DELETE SET NULL,
    reserved_vcpus INTEGER NOT NULL,
    reserved_memory_mb INTEGER NOT NULL,
    reserved_storage_gb INTEGER NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'active', -- active, committed, released
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP WITH TIME ZONE,
    released_at TIMESTAMP WITH TIME ZONE
);
CREATE INDEX IF NOT EXISTS idx_reservations_owner ON resource_reservations(owner_id);
CREATE INDEX IF NOT EXISTS idx_reservations_status ON resource_reservations(status);

-- Table: audit_logs
CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(64) NOT NULL,
    resource_type VARCHAR(64) NOT NULL,
    resource_id UUID,
    result VARCHAR(32) NOT NULL, -- success, failure
    ip_address VARCHAR(45),
    metadata JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_audit_logs_actor ON audit_logs(actor_user_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at DESC);

-- AI Agent & Conversational Infrastructure Tables (PRD Section 42)
CREATE TABLE IF NOT EXISTS ai_conversations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    state VARCHAR(64) NOT NULL DEFAULT 'UnderstandingRequest',
    current_intent VARCHAR(64),
    summary TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);
CREATE INDEX IF NOT EXISTS idx_ai_conversations_user ON ai_conversations(user_id);

CREATE TABLE IF NOT EXISTS ai_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES ai_conversations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(32) NOT NULL,
    content TEXT NOT NULL,
    ui_components TEXT,
    raw_input TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_ai_messages_conv ON ai_messages(conversation_id);

CREATE TABLE IF NOT EXISTS ai_conversation_state (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES ai_conversations(id) ON DELETE CASCADE UNIQUE,
    state VARCHAR(64) NOT NULL DEFAULT 'UnderstandingRequest',
    current_intent VARCHAR(64),
    pending_questions TEXT,
    gathered_requirements TEXT,
    active_plan_id UUID,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS ai_resource_plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES ai_conversations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    version INTEGER NOT NULL DEFAULT 1,
    intent VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'draft',
    title VARCHAR(255) NOT NULL,
    description TEXT,
    plan_json TEXT NOT NULL,
    estimated_costs TEXT,
    validation_errors TEXT,
    requires_confirmation BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_ai_plans_conv ON ai_resource_plans(conversation_id);

CREATE TABLE IF NOT EXISTS ai_plan_resources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_id UUID NOT NULL REFERENCES ai_resource_plans(id) ON DELETE CASCADE,
    resource_type VARCHAR(64) NOT NULL,
    resource_name VARCHAR(128) NOT NULL,
    config_json TEXT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    provisioned_id VARCHAR(128),
    execution_order INTEGER NOT NULL DEFAULT 1,
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_ai_plan_res_plan ON ai_plan_resources(plan_id);

CREATE TABLE IF NOT EXISTS ai_approvals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_id UUID NOT NULL REFERENCES ai_resource_plans(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    version INTEGER NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    approved_at TIMESTAMP WITH TIME ZONE,
    cancelled_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS ai_tool_invocations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID REFERENCES ai_conversations(id) ON DELETE SET NULL,
    run_id UUID,
    tool_name VARCHAR(128) NOT NULL,
    server_name VARCHAR(128) NOT NULL,
    input_json TEXT,
    output_json TEXT,
    status VARCHAR(32) NOT NULL,
    risk_level VARCHAR(32) NOT NULL DEFAULT 'read',
    execution_time_ms BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS ai_agent_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES ai_conversations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status VARCHAR(32) NOT NULL DEFAULT 'running',
    model VARCHAR(128),
    prompt_tokens INTEGER DEFAULT 0,
    completion_tokens INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP WITH TIME ZONE
);

CREATE TABLE IF NOT EXISTS mcp_servers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(64) NOT NULL UNIQUE,
    display_name VARCHAR(128) NOT NULL,
    description TEXT,
    transport VARCHAR(32) NOT NULL DEFAULT 'in_process',
    endpoint_url VARCHAR(512),
    is_first_party BOOLEAN NOT NULL DEFAULT false,
    enabled BOOLEAN NOT NULL DEFAULT true,
    auth_type VARCHAR(32) DEFAULT 'none',
    icon VARCHAR(64),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS mcp_connections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    server_id UUID NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    status VARCHAR(32) NOT NULL DEFAULT 'connected',
    config_json TEXT,
    last_health_check TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS mcp_tools (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id UUID NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    name VARCHAR(128) NOT NULL,
    description TEXT,
    risk_level VARCHAR(32) NOT NULL DEFAULT 'read',
    input_schema TEXT,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS mcp_permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    connection_id UUID NOT NULL REFERENCES mcp_connections(id) ON DELETE CASCADE,
    tool_id UUID NOT NULL REFERENCES mcp_tools(id) ON DELETE CASCADE,
    granted BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS mcp_audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    server_id UUID NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    tool_name VARCHAR(128) NOT NULL,
    risk_level VARCHAR(32) NOT NULL,
    confirmed BOOLEAN NOT NULL DEFAULT false,
    input_summary TEXT,
    result_status VARCHAR(32) NOT NULL,
    ip_address VARCHAR(45),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

