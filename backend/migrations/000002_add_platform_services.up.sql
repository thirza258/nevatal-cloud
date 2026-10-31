-- Migration 000002: Add Platform Services
-- Includes: Applications, Deployments, Git Connections, Automations (n8n), OpenClaw AI, OS Images, S3 Object Storage, AI Assistant Messages

-- Table: applications
CREATE TABLE IF NOT EXISTS applications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    slug VARCHAR(100) NOT NULL,
    application_type VARCHAR(50) NOT NULL,
    deployment_source_type VARCHAR(50) NOT NULL,
    repository_url VARCHAR(255),
    production_branch VARCHAR(100) DEFAULT 'main',
    root_directory VARCHAR(255) DEFAULT '/',
    build_command VARCHAR(255),
    output_directory VARCHAR(100) DEFAULT 'dist',
    start_command VARCHAR(255),
    application_port INTEGER DEFAULT 8080,
    health_check_path VARCHAR(100) DEFAULT '/',
    assigned_subdomain VARCHAR(150) NOT NULL UNIQUE,
    custom_domain VARCHAR(255),
    status VARCHAR(32) NOT NULL DEFAULT 'draft',
    v_cpus INTEGER DEFAULT 1,
    memory_mb INTEGER DEFAULT 1024,
    disk_gb INTEGER DEFAULT 10,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);
CREATE INDEX IF NOT EXISTS idx_applications_owner_id ON applications(owner_id);
CREATE INDEX IF NOT EXISTS idx_applications_slug ON applications(slug);

-- Table: deployments
CREATE TABLE IF NOT EXISTS deployments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    source_commit VARCHAR(40),
    commit_message VARCHAR(255),
    source_branch VARCHAR(100) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'queued',
    build_logs TEXT,
    artifact_reference VARCHAR(255),
    runtime_image_digest VARCHAR(255),
    error_code VARCHAR(100),
    error_message TEXT,
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_deployments_application_id ON deployments(application_id);

-- Table: application_environment_variables
CREATE TABLE IF NOT EXISTS application_environment_variables (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    key VARCHAR(100) NOT NULL,
    value TEXT NOT NULL,
    is_secret BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_app_env_vars_app_id ON application_environment_variables(application_id);

-- Table: git_connections
CREATE TABLE IF NOT EXISTS git_connections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider VARCHAR(50) NOT NULL,
    provider_account_id VARCHAR(100) NOT NULL,
    display_name VARCHAR(100) NOT NULL,
    username VARCHAR(100) NOT NULL,
    avatar_url VARCHAR(512),
    status VARCHAR(32) DEFAULT 'connected',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_git_connections_owner_id ON git_connections(owner_id);

-- Table: automation_instances (n8n)
CREATE TABLE IF NOT EXISTS automation_instances (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    n8n_version VARCHAR(50) NOT NULL DEFAULT '1.71.0',
    status VARCHAR(32) NOT NULL DEFAULT 'running',
    hostname VARCHAR(150) NOT NULL UNIQUE,
    dashboard_url VARCHAR(255) NOT NULL,
    encryption_key VARCHAR(255) NOT NULL,
    database_name VARCHAR(100) NOT NULL,
    cpu_limit INTEGER DEFAULT 2,
    memory_limit_mb INTEGER DEFAULT 2048,
    storage_limit_gb INTEGER DEFAULT 20,
    storage_used_gb NUMERIC(10,2) DEFAULT 1.2,
    active_workflows_count INTEGER DEFAULT 0,
    total_executions BIGINT DEFAULT 0,
    last_health_check_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);
CREATE INDEX IF NOT EXISTS idx_automation_instances_owner_id ON automation_instances(owner_id);

-- Table: automation_backups
CREATE TABLE IF NOT EXISTS automation_backups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    automation_instance_id UUID NOT NULL REFERENCES automation_instances(id) ON DELETE CASCADE,
    name VARCHAR(150) NOT NULL,
    backup_reference VARCHAR(255) NOT NULL,
    size_bytes BIGINT,
    status VARCHAR(32) DEFAULT 'completed',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_automation_backups_instance_id ON automation_backups(automation_instance_id);

-- Table: openclaw_instances (Autonomous AI Agents)
CREATE TABLE IF NOT EXISTS openclaw_instances (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    version VARCHAR(50) NOT NULL DEFAULT '2.4.0',
    status VARCHAR(32) NOT NULL DEFAULT 'running',
    hostname VARCHAR(150) NOT NULL UNIQUE,
    gateway_endpoint VARCHAR(255) NOT NULL,
    gateway_auth_token VARCHAR(255) NOT NULL,
    workspace_path VARCHAR(255) NOT NULL,
    ai_provider VARCHAR(50) NOT NULL DEFAULT 'openai',
    default_model VARCHAR(100) NOT NULL DEFAULT 'gpt-4o',
    encrypted_api_key VARCHAR(512),
    masked_api_key VARCHAR(50),
    cpu_limit INTEGER DEFAULT 2,
    memory_limit_mb INTEGER DEFAULT 4096,
    storage_limit_gb INTEGER DEFAULT 30,
    storage_used_gb NUMERIC(10,2) DEFAULT 2.4,
    active_skills_count INTEGER DEFAULT 5,
    total_interactions BIGINT DEFAULT 0,
    last_health_check_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);
CREATE INDEX IF NOT EXISTS idx_openclaw_instances_owner_id ON openclaw_instances(owner_id);

-- Table: openclaw_channel_integrations
CREATE TABLE IF NOT EXISTS openclaw_channel_integrations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    open_claw_instance_id UUID NOT NULL REFERENCES openclaw_instances(id) ON DELETE CASCADE,
    channel VARCHAR(50) NOT NULL,
    identifier VARCHAR(100) NOT NULL,
    status VARCHAR(32) DEFAULT 'connected',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_openclaw_integrations_instance_id ON openclaw_channel_integrations(open_claw_instance_id);

-- Table: openclaw_backups
CREATE TABLE IF NOT EXISTS openclaw_backups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    open_claw_instance_id UUID NOT NULL REFERENCES openclaw_instances(id) ON DELETE CASCADE,
    name VARCHAR(150) NOT NULL,
    backup_reference VARCHAR(255) NOT NULL,
    size_bytes BIGINT,
    status VARCHAR(32) DEFAULT 'completed',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_openclaw_backups_instance_id ON openclaw_backups(open_claw_instance_id);

-- Table: os_images (Custom ISOs)
CREATE TABLE IF NOT EXISTS os_images (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    version VARCHAR(50) NOT NULL,
    architecture VARCHAR(32) NOT NULL DEFAULT 'x86_64',
    category VARCHAR(50) NOT NULL DEFAULT 'Linux',
    description TEXT,
    filename VARCHAR(255) NOT NULL,
    file_size_bytes BIGINT NOT NULL,
    sha256 VARCHAR(64) NOT NULL,
    proxmox_storage VARCHAR(100) NOT NULL DEFAULT 'local',
    proxmox_volume VARCHAR(255) NOT NULL,
    visibility VARCHAR(32) NOT NULL DEFAULT 'published',
    status VARCHAR(32) NOT NULL DEFAULT 'available',
    created_by_id UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Table: storage_buckets (S3 Compatible Storage)
CREATE TABLE IF NOT EXISTS storage_buckets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(63) NOT NULL UNIQUE,
    description VARCHAR(255),
    visibility VARCHAR(32) NOT NULL DEFAULT 'private',
    versioning_enabled BOOLEAN NOT NULL DEFAULT false,
    region VARCHAR(50) NOT NULL DEFAULT 'id-central-1',
    total_objects BIGINT DEFAULT 0,
    total_size_bytes BIGINT DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);
CREATE INDEX IF NOT EXISTS idx_storage_buckets_owner_id ON storage_buckets(owner_id);

-- Table: storage_objects
CREATE TABLE IF NOT EXISTS storage_objects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bucket_id UUID NOT NULL REFERENCES storage_buckets(id) ON DELETE CASCADE,
    key VARCHAR(1024) NOT NULL,
    filename VARCHAR(255) NOT NULL,
    folder_prefix VARCHAR(512) DEFAULT '',
    content_type VARCHAR(150) NOT NULL DEFAULT 'application/octet-stream',
    size_bytes BIGINT NOT NULL,
    etag VARCHAR(64),
    is_folder BOOLEAN NOT NULL DEFAULT false,
    public_url VARCHAR(512),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_storage_objects_bucket_id ON storage_objects(bucket_id);
CREATE INDEX IF NOT EXISTS idx_storage_objects_key ON storage_objects(key);

-- Table: storage_access_keys
CREATE TABLE IF NOT EXISTS storage_access_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    access_key_id VARCHAR(64) NOT NULL UNIQUE,
    secret_access_key VARCHAR(128) NOT NULL,
    description VARCHAR(255),
    permissions VARCHAR(50) NOT NULL DEFAULT 'read_write',
    bucket_scope VARCHAR(255) DEFAULT '*',
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    last_used_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_storage_access_keys_owner_id ON storage_access_keys(owner_id);

-- Table: storage_shared_links
CREATE TABLE IF NOT EXISTS storage_shared_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    object_id UUID NOT NULL REFERENCES storage_objects(id) ON DELETE CASCADE,
    share_token VARCHAR(64) NOT NULL UNIQUE,
    share_url VARCHAR(512) NOT NULL,
    access_type VARCHAR(32) DEFAULT 'download',
    password VARCHAR(255),
    has_password BOOLEAN DEFAULT false,
    expires_at TIMESTAMP WITH TIME ZONE,
    downloads BIGINT DEFAULT 0,
    max_downloads BIGINT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_storage_shared_links_owner_id ON storage_shared_links(owner_id);
CREATE INDEX IF NOT EXISTS idx_storage_shared_links_object_id ON storage_shared_links(object_id);

-- Table: ai_assistant_messages
CREATE TABLE IF NOT EXISTS ai_assistant_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(32) NOT NULL,
    content TEXT NOT NULL,
    model VARCHAR(100) NOT NULL DEFAULT 'openai/gpt-4o-mini',
    action_type VARCHAR(50),
    action_payload TEXT,
    action_executed BOOLEAN DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_ai_assistant_messages_user_id ON ai_assistant_messages(user_id);

-- Schema migrations for OAuth & integrations
ALTER TABLE users ADD COLUMN IF NOT EXISTS auth_provider VARCHAR(32) NOT NULL DEFAULT 'local';
ALTER TABLE users ADD COLUMN IF NOT EXISTS auth_id VARCHAR(255);
ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar_url VARCHAR(512);
ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL;

-- Schema additions for resource_plans sort_order
ALTER TABLE resource_plans ADD COLUMN IF NOT EXISTS sort_order INTEGER NOT NULL DEFAULT 0;
