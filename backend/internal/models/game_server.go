package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// GameType constants per PRD Section 49.1
const (
	GameTypeMinecraftJava    = "minecraft_java"
	GameTypeMinecraftBedrock = "minecraft_bedrock"
	GameTypeTerraria         = "terraria"
)

// ServerSoftware constants per PRD Section 51.1 & 52.1
const (
	SoftwareVanilla       = "vanilla"
	SoftwarePaper         = "paper"
	SoftwarePurpur        = "purpur"
	SoftwareFabric        = "fabric"
	SoftwareForge         = "forge"
	SoftwareNeoForge      = "neoforge"
	SoftwareBedrock       = "bedrock"
	SoftwareTerraria      = "terraria_vanilla"
	SoftwareTModLoader    = "tmodloader"
)

// GameServerStatus constants per PRD Section 53
const (
	GameServerStatusInstalling = "installing"
	GameServerStatusStarting   = "starting"
	GameServerStatusRunning    = "running"
	GameServerStatusStopping   = "stopping"
	GameServerStatusStopped    = "stopped"
	GameServerStatusError      = "error"
	GameServerStatusRebuilding = "rebuilding"
)

// GameServer represents a customer managed game server instance
type GameServer struct {
	ID                 uuid.UUID               `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	OwnerID            uuid.UUID               `gorm:"type:uuid;not null;index" json:"owner_id"`
	Name               string                  `gorm:"type:varchar(100);not null" json:"name"`
	Slug               string                  `gorm:"type:varchar(100);not null;index" json:"slug"`
	GameType           string                  `gorm:"type:varchar(50);not null;index" json:"game_type"` // minecraft_java, minecraft_bedrock, terraria
	GameVersion        string                  `gorm:"type:varchar(50);not null" json:"game_version"`
	ServerSoftware     string                  `gorm:"type:varchar(50);not null" json:"server_software"`
	Status             string                  `gorm:"type:varchar(32);not null;default:'installing'" json:"status"`
	PlanName           string                  `gorm:"type:varchar(50);default:'basic'" json:"plan_name"` // starter, basic, standard, advanced, custom
	CPUCores           int                     `gorm:"default:2" json:"cpu_cores"`
	MemoryMB           int                     `gorm:"default:4096" json:"memory_mb"`
	StorageGB          int                     `gorm:"default:20" json:"storage_gb"`
	PlayerSlots        int                     `gorm:"default:10" json:"player_slots"`
	AssignedPort       int                     `gorm:"not null" json:"assigned_port"`
	QueryPort          int                     `gorm:"default:0" json:"query_port,omitempty"`
	RCONPort           int                     `gorm:"default:0" json:"rcon_port,omitempty"`
	RCONPassword       string                  `gorm:"type:varchar(100)" json:"-"`
	PublicAddress      string                  `gorm:"type:varchar(255);not null" json:"public_address"`
	CustomDomain       string                  `gorm:"type:varchar(255)" json:"custom_domain,omitempty"`
	MOTD               string                  `gorm:"type:varchar(255);default:'Welcome to Nevatal Game Server'" json:"motd"`
	ProxmoxVMID        *int                    `gorm:"column:proxmox_vmid" json:"proxmox_vmid,omitempty"`
	NodeName           string                  `gorm:"type:varchar(100);default:'pve-dell-r640'" json:"node_name"`
	UptimeSeconds      int64                   `gorm:"default:0" json:"uptime_seconds"`
	TPS                float64                 `gorm:"default:20.0" json:"tps"`
	MSPT               float64                 `gorm:"default:12.5" json:"mspt"`
	OnlinePlayerCount  int                     `gorm:"default:0" json:"online_player_count"`
	AutoBackupEnabled  bool                    `gorm:"default:true" json:"auto_backup_enabled"`
	LastBackupAt       *time.Time              `json:"last_backup_at,omitempty"`
	LastHealthCheckAt  *time.Time              `json:"last_health_check_at,omitempty"`
	CreatedAt          time.Time               `json:"created_at"`
	UpdatedAt          time.Time               `json:"updated_at"`
	DeletedAt          gorm.DeletedAt          `gorm:"index" json:"-"`

	// Relations
	Ports          []GameServerPort          `gorm:"foreignKey:ServerID" json:"ports,omitempty"`
	Configurations []GameServerConfiguration `gorm:"foreignKey:ServerID" json:"configurations,omitempty"`
	Players        []GameServerPlayer        `gorm:"foreignKey:ServerID" json:"players,omitempty"`
	Plugins        []GameServerPlugin        `gorm:"foreignKey:ServerID" json:"plugins,omitempty"`
	Mods           []GameServerMod           `gorm:"foreignKey:ServerID" json:"mods,omitempty"`
	Worlds         []GameServerWorld         `gorm:"foreignKey:ServerID" json:"worlds,omitempty"`
	Backups        []GameServerBackup        `gorm:"foreignKey:ServerID" json:"backups,omitempty"`
}

// GameServerTemplate represents catalog templates for one-click setup
type GameServerTemplate struct {
	ID                uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	GameType          string    `gorm:"type:varchar(50);not null;index" json:"game_type"`
	Name              string    `gorm:"type:varchar(100);not null" json:"name"`
	Slug              string    `gorm:"type:varchar(100);not null;unique" json:"slug"`
	ServerSoftware    string    `gorm:"type:varchar(50);not null" json:"server_software"`
	Description       string    `gorm:"type:text" json:"description"`
	MinCPU            int       `gorm:"default:1" json:"min_cpu"`
	MinRAMMB          int       `gorm:"default:2048" json:"min_ram_mb"`
	MinStorageGB      int       `gorm:"default:10" json:"min_storage_gb"`
	DefaultConfig     string    `gorm:"type:text" json:"default_config"`
	SupportedVersions string    `gorm:"type:varchar(255)" json:"supported_versions"`
	IsModded          bool      `gorm:"default:false" json:"is_modded"`
	Enabled           bool      `gorm:"default:true" json:"enabled"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// GameServerVersion tracks supported game server releases
type GameServerVersion struct {
	ID             uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	GameType       string    `gorm:"type:varchar(50);not null;index" json:"game_type"`
	ServerSoftware string    `gorm:"type:varchar(50);not null" json:"server_software"`
	VersionString  string    `gorm:"type:varchar(50);not null" json:"version_string"`
	IsLatestStable bool      `gorm:"default:false" json:"is_latest_stable"`
	ReleaseDate    string    `gorm:"type:varchar(50)" json:"release_date"`
	JavaVersion    string    `gorm:"type:varchar(50);default:'21'" json:"java_version,omitempty"`
	DownloadURL    string    `gorm:"type:varchar(255)" json:"download_url,omitempty"`
	Enabled        bool      `gorm:"default:true" json:"enabled"`
}

// GameServerPort tracks assigned ports and prevents collision
type GameServerPort struct {
	ID           uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ServerID     uuid.UUID  `gorm:"type:uuid;not null;index" json:"server_id"`
	PortNumber   int        `gorm:"not null;index" json:"port_number"`
	Protocol     string     `gorm:"type:varchar(10);not null" json:"protocol"` // tcp, udp
	ServiceType  string     `gorm:"type:varchar(20);not null" json:"service_type"` // game, query, rcon
	NodeName     string     `gorm:"type:varchar(100);not null" json:"node_name"`
	IsAllocated  bool       `gorm:"default:true" json:"is_allocated"`
	AllocatedAt  time.Time  `json:"allocated_at"`
	ReleasedAt   *time.Time `json:"released_at,omitempty"`
}

// GameServerConfiguration holds server-specific property settings
type GameServerConfiguration struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ServerID  uuid.UUID `gorm:"type:uuid;not null;index" json:"server_id"`
	Key       string    `gorm:"type:varchar(100);not null" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	Category  string    `gorm:"type:varchar(50);default:'general'" json:"category"`
	DataType  string    `gorm:"type:varchar(20);default:'string'" json:"data_type"`
	UpdatedAt time.Time `json:"updated_at"`
}

// GameServerPlayer represents player list, whitelist, operators, and ban records
type GameServerPlayer struct {
	ID            uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ServerID      uuid.UUID  `gorm:"type:uuid;not null;index" json:"server_id"`
	PlayerName    string     `gorm:"type:varchar(100);not null" json:"player_name"`
	PlayerUUID    string     `gorm:"type:varchar(100)" json:"player_uuid,omitempty"`
	IsOnline      bool       `gorm:"default:false" json:"is_online"`
	IsWhitelisted bool       `gorm:"default:false" json:"is_whitelisted"`
	IsOperator    bool       `gorm:"default:false" json:"is_operator"`
	IsBanned      bool       `gorm:"default:false" json:"is_banned"`
	BanReason     string     `gorm:"type:varchar(255)" json:"ban_reason,omitempty"`
	FirstJoinedAt *time.Time `json:"first_joined_at,omitempty"`
	LastSeenAt    *time.Time `json:"last_seen_at,omitempty"`
}

// GameServerPlugin represents installed Minecraft plugins (Paper/Purpur)
type GameServerPlugin struct {
	ID            uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ServerID      uuid.UUID `gorm:"type:uuid;not null;index" json:"server_id"`
	Name          string    `gorm:"type:varchar(100);not null" json:"name"`
	FileName      string    `gorm:"type:varchar(255);not null" json:"file_name"`
	Version       string    `gorm:"type:varchar(50)" json:"version"`
	Author        string    `gorm:"type:varchar(100)" json:"author,omitempty"`
	IsEnabled     bool      `gorm:"default:true" json:"is_enabled"`
	IsCompatible  bool      `gorm:"default:true" json:"is_compatible"`
	FileSizeBytes int64     `gorm:"default:0" json:"file_size_bytes"`
	CreatedAt     time.Time `json:"created_at"`
}

// GameServerMod represents installed mod files (Fabric, Forge, tModLoader)
type GameServerMod struct {
	ID            uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ServerID      uuid.UUID `gorm:"type:uuid;not null;index" json:"server_id"`
	Name          string    `gorm:"type:varchar(100);not null" json:"name"`
	FileName      string    `gorm:"type:varchar(255);not null" json:"file_name"`
	Version       string    `gorm:"type:varchar(50)" json:"version"`
	ModLoader     string    `gorm:"type:varchar(50)" json:"mod_loader"` // fabric, forge, neoforge, tmodloader
	IsEnabled     bool      `gorm:"default:true" json:"is_enabled"`
	FileSizeBytes int64     `gorm:"default:0" json:"file_size_bytes"`
	CreatedAt     time.Time `json:"created_at"`
}

// GameServerWorld represents world storage entries
type GameServerWorld struct {
	ID         uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ServerID   uuid.UUID `gorm:"type:uuid;not null;index" json:"server_id"`
	WorldName  string    `gorm:"type:varchar(100);not null" json:"world_name"`
	IsActive   bool      `gorm:"default:true" json:"is_active"`
	SizeBytes  int64     `gorm:"default:0" json:"size_bytes"`
	Seed       string    `gorm:"type:varchar(100)" json:"seed,omitempty"`
	Difficulty string    `gorm:"type:varchar(50);default:'normal'" json:"difficulty"`
	WorldSize  string    `gorm:"type:varchar(50);default:'medium'" json:"world_size"` // small, medium, large (Terraria)
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// GameServerBackup represents persistent world backup archives
type GameServerBackup struct {
	ID            uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ServerID      uuid.UUID `gorm:"type:uuid;not null;index" json:"server_id"`
	UserID        uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	BackupName    string    `gorm:"type:varchar(255);not null" json:"backup_name"`
	FileSizeBytes int64     `gorm:"default:0" json:"file_size_bytes"`
	StoragePath   string    `gorm:"type:varchar(255);not null" json:"storage_path"`
	WorldName     string    `gorm:"type:varchar(100)" json:"world_name"`
	IsAutomated   bool      `gorm:"default:false" json:"is_automated"`
	Checksum      string    `gorm:"type:varchar(64)" json:"checksum"`
	Status        string    `gorm:"type:varchar(32);default:'completed'" json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

// GameServerAuditLog records sensitive operations per PRD Section 60
type GameServerAuditLog struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ServerID    uuid.UUID `gorm:"type:uuid;not null;index" json:"server_id"`
	UserID      uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	Action      string    `gorm:"type:varchar(100);not null" json:"action"`
	ActorEmail  string    `gorm:"type:varchar(255)" json:"actor_email"`
	IPAddress   string    `gorm:"type:varchar(64)" json:"ip_address"`
	DetailsJSON string    `gorm:"type:text" json:"details_json"`
	CreatedAt   time.Time `json:"created_at"`
}
