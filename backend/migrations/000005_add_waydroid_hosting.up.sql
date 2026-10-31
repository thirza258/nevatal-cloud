-- Migration 000005: Add Waydroid Android Cloud Hosting Tables per PRD v1.0 Section 79
-- Supports Managed Android Cloud environments powered by Waydroid on Linux hypervisors

-- Table: android_images
CREATE TABLE IF NOT EXISTS android_images (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(120) NOT NULL,
    slug VARCHAR(100) NOT NULL UNIQUE,
    android_version VARCHAR(32) NOT NULL,
    architecture VARCHAR(32) NOT NULL DEFAULT 'x86_64',
    image_type VARCHAR(32) NOT NULL DEFAULT 'standard', -- standard, developer, testing, custom
    system_image_url VARCHAR(255) NOT NULL,
    vendor_image_url VARCHAR(255) NOT NULL,
    image_source VARCHAR(64) NOT NULL DEFAULT 'official',
    checksum_sha256 VARCHAR(64) NOT NULL,
    min_cpu INTEGER NOT NULL DEFAULT 2,
    min_ram_mb INTEGER NOT NULL DEFAULT 4096,
    min_storage_gb INTEGER NOT NULL DEFAULT 32,
    is_published BOOLEAN NOT NULL DEFAULT true,
    is_verified BOOLEAN NOT NULL DEFAULT true,
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_android_images_version ON android_images(android_version);
CREATE INDEX IF NOT EXISTS idx_android_images_published ON android_images(is_published);

-- Table: android_image_versions
CREATE TABLE IF NOT EXISTS android_image_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    image_id UUID NOT NULL REFERENCES android_images(id) ON DELETE CASCADE,
    version_tag VARCHAR(50) NOT NULL,
    channel VARCHAR(32) NOT NULL DEFAULT 'stable', -- stable, beta, nightly
    release_notes TEXT,
    system_checksum VARCHAR(64),
    vendor_checksum VARCHAR(64),
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_android_img_ver_image ON android_image_versions(image_id);

-- Table: android_worker_nodes
CREATE TABLE IF NOT EXISTS android_worker_nodes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    proxmox_node_name VARCHAR(100) NOT NULL DEFAULT 'pve-dell-r640',
    ip_address VARCHAR(45) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'ready', -- ready, maintenance, offline
    kernel_version VARCHAR(64) NOT NULL DEFAULT '6.8.12-waydroid-pve',
    binder_status VARCHAR(32) NOT NULL DEFAULT 'active',
    total_cpu_cores INTEGER NOT NULL DEFAULT 32,
    total_memory_mb INTEGER NOT NULL DEFAULT 131072,
    total_storage_gb INTEGER NOT NULL DEFAULT 2048,
    active_instances_count INTEGER NOT NULL DEFAULT 0,
    last_health_check_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_android_worker_status ON android_worker_nodes(status);

-- Table: android_instances
CREATE TABLE IF NOT EXISTS android_instances (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    image_id UUID NOT NULL REFERENCES android_images(id),
    worker_node_id UUID REFERENCES android_worker_nodes(id) ON DELETE SET NULL,
    name VARCHAR(120) NOT NULL,
    slug VARCHAR(120) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'running', -- provisioning, running, stopped, suspended, reinstalling, error
    plan_name VARCHAR(50) NOT NULL DEFAULT 'standard',
    vcpus INTEGER NOT NULL DEFAULT 2,
    memory_mb INTEGER NOT NULL DEFAULT 4096,
    storage_gb INTEGER NOT NULL DEFAULT 32,
    display_resolution VARCHAR(32) NOT NULL DEFAULT '1280x720',
    display_density INTEGER NOT NULL DEFAULT 240,
    orientation VARCHAR(20) NOT NULL DEFAULT 'portrait', -- portrait, landscape
    remote_access_type VARCHAR(32) NOT NULL DEFAULT 'browser', -- browser, webrtc, vnc
    access_url VARCHAR(255) NOT NULL,
    session_token VARCHAR(128),
    ip_address VARCHAR(45) NOT NULL DEFAULT '10.192.0.10',
    gateway_port INTEGER NOT NULL DEFAULT 6080,
    adb_enabled BOOLEAN NOT NULL DEFAULT false,
    adb_port INTEGER DEFAULT 0,
    adb_auth_key VARCHAR(128),
    persistence_enabled BOOLEAN NOT NULL DEFAULT true,
    auto_backup_enabled BOOLEAN NOT NULL DEFAULT true,
    backup_schedule VARCHAR(50) NOT NULL DEFAULT 'daily',
    uptime_seconds BIGINT NOT NULL DEFAULT 0,
    cpu_percent NUMERIC(5,2) NOT NULL DEFAULT 0.00,
    memory_usage_mb INTEGER NOT NULL DEFAULT 0,
    storage_usage_gb NUMERIC(6,2) NOT NULL DEFAULT 0.00,
    last_backup_at TIMESTAMP WITH TIME ZONE,
    last_accessed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);
CREATE INDEX IF NOT EXISTS idx_android_instances_owner ON android_instances(owner_id);
CREATE INDEX IF NOT EXISTS idx_android_instances_status ON android_instances(status);
CREATE INDEX IF NOT EXISTS idx_android_instances_slug ON android_instances(slug);

-- Table: android_instance_resources
CREATE TABLE IF NOT EXISTS android_instance_resources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    instance_id UUID NOT NULL REFERENCES android_instances(id) ON DELETE CASCADE,
    cpu_limit INTEGER NOT NULL DEFAULT 2,
    memory_limit_mb INTEGER NOT NULL DEFAULT 4096,
    storage_limit_gb INTEGER NOT NULL DEFAULT 32,
    gpu_acceleration BOOLEAN NOT NULL DEFAULT false,
    io_priority VARCHAR(20) NOT NULL DEFAULT 'standard',
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_android_inst_res_inst ON android_instance_resources(instance_id);

-- Table: android_data_volumes
CREATE TABLE IF NOT EXISTS android_data_volumes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    instance_id UUID NOT NULL REFERENCES android_instances(id) ON DELETE CASCADE,
    volume_name VARCHAR(100) NOT NULL,
    mount_path VARCHAR(255) NOT NULL DEFAULT '/data',
    zfs_dataset VARCHAR(255) NOT NULL,
    size_bytes BIGINT NOT NULL DEFAULT 0,
    is_encrypted BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_android_volumes_inst ON android_data_volumes(instance_id);

-- Table: android_applications
CREATE TABLE IF NOT EXISTS android_applications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    instance_id UUID NOT NULL REFERENCES android_instances(id) ON DELETE CASCADE,
    package_name VARCHAR(150) NOT NULL,
    app_name VARCHAR(120) NOT NULL,
    version_name VARCHAR(64) NOT NULL DEFAULT '1.0.0',
    version_code INTEGER NOT NULL DEFAULT 1,
    file_size_bytes BIGINT NOT NULL DEFAULT 0,
    icon_url VARCHAR(255),
    is_system_app BOOLEAN NOT NULL DEFAULT false,
    is_running BOOLEAN NOT NULL DEFAULT false,
    installed_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    last_launched_at TIMESTAMP WITH TIME ZONE
);
CREATE INDEX IF NOT EXISTS idx_android_apps_instance ON android_applications(instance_id);
CREATE INDEX IF NOT EXISTS idx_android_apps_pkg ON android_applications(instance_id, package_name);

-- Table: android_sessions
CREATE TABLE IF NOT EXISTS android_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    instance_id UUID NOT NULL REFERENCES android_instances(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    session_token VARCHAR(128) NOT NULL UNIQUE,
    access_type VARCHAR(32) NOT NULL DEFAULT 'browser',
    client_ip VARCHAR(45) NOT NULL,
    user_agent TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    started_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    ended_at TIMESTAMP WITH TIME ZONE
);
CREATE INDEX IF NOT EXISTS idx_android_sessions_inst ON android_sessions(instance_id);
CREATE INDEX IF NOT EXISTS idx_android_sessions_token ON android_sessions(session_token);

-- Table: android_backups
CREATE TABLE IF NOT EXISTS android_backups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    instance_id UUID NOT NULL REFERENCES android_instances(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    backup_name VARCHAR(120) NOT NULL,
    backup_type VARCHAR(32) NOT NULL DEFAULT 'snapshot', -- snapshot, full, app_data
    file_size_bytes BIGINT NOT NULL DEFAULT 0,
    storage_path VARCHAR(255) NOT NULL,
    checksum_sha256 VARCHAR(64) NOT NULL,
    is_automated BOOLEAN NOT NULL DEFAULT false,
    status VARCHAR(32) NOT NULL DEFAULT 'completed', -- creating, completed, failed, restoring
    retention_days INTEGER NOT NULL DEFAULT 30,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_android_backups_inst ON android_backups(instance_id);

-- Table: android_network_profiles
CREATE TABLE IF NOT EXISTS android_network_profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    instance_id UUID NOT NULL REFERENCES android_instances(id) ON DELETE CASCADE,
    network_bridge VARCHAR(50) NOT NULL DEFAULT 'waydroid0',
    ip_address VARCHAR(45) NOT NULL,
    mac_address VARCHAR(20) NOT NULL,
    gateway VARCHAR(45) NOT NULL DEFAULT '10.192.0.1',
    dns_servers VARCHAR(100) NOT NULL DEFAULT '1.1.1.1,8.8.8.8',
    firewall_profile VARCHAR(50) NOT NULL DEFAULT 'isolated_customer',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_android_net_inst ON android_network_profiles(instance_id);

-- Table: android_provisioning_jobs
CREATE TABLE IF NOT EXISTS android_provisioning_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    instance_id UUID NOT NULL REFERENCES android_instances(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    job_type VARCHAR(50) NOT NULL DEFAULT 'provision', -- provision, reinstall, backup, restore, apk_install
    current_step VARCHAR(64) NOT NULL DEFAULT 'init',
    progress_percent INTEGER NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL DEFAULT 'pending', -- pending, running, completed, failed
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_android_jobs_inst ON android_provisioning_jobs(instance_id);

-- Table: android_audit_logs
CREATE TABLE IF NOT EXISTS android_audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    instance_id UUID REFERENCES android_instances(id) ON DELETE SET NULL,
    actor_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(64) NOT NULL,
    client_ip VARCHAR(45),
    details_json TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_android_audit_inst ON android_audit_logs(instance_id);
