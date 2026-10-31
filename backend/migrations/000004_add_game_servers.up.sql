-- Migration 000004: Add Game Server Hosting Tables per PRD v1.0 Section 60
-- Supports Minecraft Java, Minecraft Bedrock, and Terraria

-- Table: game_servers
CREATE TABLE IF NOT EXISTS game_servers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    slug VARCHAR(100) NOT NULL,
    game_type VARCHAR(50) NOT NULL,
    game_version VARCHAR(50) NOT NULL,
    server_software VARCHAR(50) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'installing',
    plan_name VARCHAR(50) DEFAULT 'basic',
    cpu_cores INTEGER NOT NULL DEFAULT 2,
    memory_mb INTEGER NOT NULL DEFAULT 4096,
    storage_gb INTEGER NOT NULL DEFAULT 20,
    player_slots INTEGER NOT NULL DEFAULT 10,
    assigned_port INTEGER NOT NULL,
    query_port INTEGER DEFAULT 0,
    rcon_port INTEGER DEFAULT 0,
    rcon_password VARCHAR(100),
    public_address VARCHAR(255) NOT NULL,
    custom_domain VARCHAR(255),
    motd VARCHAR(255) DEFAULT 'Welcome to Nevatal Game Server',
    proxmox_vmid INTEGER,
    proxmox_vm_id INTEGER,
    node_name VARCHAR(100) DEFAULT 'pve-dell-r640',
    uptime_seconds BIGINT DEFAULT 0,
    tps NUMERIC(4,2) DEFAULT 20.00,
    mspt NUMERIC(5,2) DEFAULT 12.50,
    online_player_count INTEGER DEFAULT 0,
    auto_backup_enabled BOOLEAN DEFAULT true,
    last_backup_at TIMESTAMP WITH TIME ZONE,
    last_health_check_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);
CREATE INDEX IF NOT EXISTS idx_game_servers_owner ON game_servers(owner_id);
CREATE INDEX IF NOT EXISTS idx_game_servers_game_type ON game_servers(game_type);
CREATE INDEX IF NOT EXISTS idx_game_servers_slug ON game_servers(slug);

-- Table: game_server_templates
CREATE TABLE IF NOT EXISTS game_server_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    game_type VARCHAR(50) NOT NULL,
    name VARCHAR(100) NOT NULL,
    slug VARCHAR(100) NOT NULL UNIQUE,
    server_software VARCHAR(50) NOT NULL,
    description TEXT,
    min_cpu INTEGER DEFAULT 1,
    min_ram_mb INTEGER DEFAULT 2048,
    min_storage_gb INTEGER DEFAULT 10,
    default_config TEXT,
    supported_versions VARCHAR(255),
    is_modded BOOLEAN DEFAULT false,
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_game_server_templates_type ON game_server_templates(game_type);

-- Table: game_server_versions
CREATE TABLE IF NOT EXISTS game_server_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    game_type VARCHAR(50) NOT NULL,
    server_software VARCHAR(50) NOT NULL,
    version_string VARCHAR(50) NOT NULL,
    is_latest_stable BOOLEAN DEFAULT false,
    release_date VARCHAR(50),
    java_version VARCHAR(50) DEFAULT '21',
    download_url VARCHAR(255),
    enabled BOOLEAN DEFAULT true
);
CREATE INDEX IF NOT EXISTS idx_game_server_versions_type ON game_server_versions(game_type, server_software);

-- Table: game_server_ports
CREATE TABLE IF NOT EXISTS game_server_ports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id UUID NOT NULL REFERENCES game_servers(id) ON DELETE CASCADE,
    port_number INTEGER NOT NULL,
    protocol VARCHAR(10) NOT NULL,
    service_type VARCHAR(20) NOT NULL,
    node_name VARCHAR(100) NOT NULL,
    is_allocated BOOLEAN DEFAULT true,
    allocated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    released_at TIMESTAMP WITH TIME ZONE
);
CREATE INDEX IF NOT EXISTS idx_game_server_ports_server ON game_server_ports(server_id);
CREATE INDEX IF NOT EXISTS idx_game_server_ports_num ON game_server_ports(port_number, node_name, protocol);

-- Table: game_server_configurations
CREATE TABLE IF NOT EXISTS game_server_configurations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id UUID NOT NULL REFERENCES game_servers(id) ON DELETE CASCADE,
    key VARCHAR(100) NOT NULL,
    value TEXT,
    category VARCHAR(50) DEFAULT 'general',
    data_type VARCHAR(20) DEFAULT 'string',
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_game_server_configurations_server ON game_server_configurations(server_id);

-- Table: game_server_players
CREATE TABLE IF NOT EXISTS game_server_players (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id UUID NOT NULL REFERENCES game_servers(id) ON DELETE CASCADE,
    player_name VARCHAR(100) NOT NULL,
    player_uuid VARCHAR(100),
    is_online BOOLEAN DEFAULT false,
    is_whitelisted BOOLEAN DEFAULT false,
    is_operator BOOLEAN DEFAULT false,
    is_banned BOOLEAN DEFAULT false,
    ban_reason VARCHAR(255),
    first_joined_at TIMESTAMP WITH TIME ZONE,
    last_seen_at TIMESTAMP WITH TIME ZONE
);
CREATE INDEX IF NOT EXISTS idx_game_server_players_server ON game_server_players(server_id);

-- Table: game_server_plugins
CREATE TABLE IF NOT EXISTS game_server_plugins (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id UUID NOT NULL REFERENCES game_servers(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    file_name VARCHAR(255) NOT NULL,
    version VARCHAR(50),
    author VARCHAR(100),
    is_enabled BOOLEAN DEFAULT true,
    is_compatible BOOLEAN DEFAULT true,
    file_size_bytes BIGINT DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_game_server_plugins_server ON game_server_plugins(server_id);

-- Table: game_server_mods
CREATE TABLE IF NOT EXISTS game_server_mods (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id UUID NOT NULL REFERENCES game_servers(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    file_name VARCHAR(255) NOT NULL,
    version VARCHAR(50),
    mod_loader VARCHAR(50) NOT NULL,
    is_enabled BOOLEAN DEFAULT true,
    file_size_bytes BIGINT DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_game_server_mods_server ON game_server_mods(server_id);

-- Table: game_server_worlds
CREATE TABLE IF NOT EXISTS game_server_worlds (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id UUID NOT NULL REFERENCES game_servers(id) ON DELETE CASCADE,
    world_name VARCHAR(100) NOT NULL,
    is_active BOOLEAN DEFAULT true,
    size_bytes BIGINT DEFAULT 0,
    seed VARCHAR(100),
    difficulty VARCHAR(50) DEFAULT 'normal',
    world_size VARCHAR(50) DEFAULT 'medium',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_game_server_worlds_server ON game_server_worlds(server_id);

-- Table: game_server_backups
CREATE TABLE IF NOT EXISTS game_server_backups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id UUID NOT NULL REFERENCES game_servers(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    backup_name VARCHAR(255) NOT NULL,
    file_size_bytes BIGINT DEFAULT 0,
    storage_path VARCHAR(255) NOT NULL,
    world_name VARCHAR(100),
    is_automated BOOLEAN DEFAULT false,
    checksum VARCHAR(64),
    status VARCHAR(32) DEFAULT 'completed',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_game_server_backups_server ON game_server_backups(server_id);

-- Table: game_server_audit_logs
CREATE TABLE IF NOT EXISTS game_server_audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id UUID NOT NULL REFERENCES game_servers(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    action VARCHAR(100) NOT NULL,
    actor_email VARCHAR(255),
    ip_address VARCHAR(64),
    details_json TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_game_server_audit_server ON game_server_audit_logs(server_id);
