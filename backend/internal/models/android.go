package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Android Status constants per PRD Section 72
const (
	AndroidStatusProvisioning = "provisioning"
	AndroidStatusRunning      = "running"
	AndroidStatusStopped      = "stopped"
	AndroidStatusSuspended    = "suspended"
	AndroidStatusReinstalling = "reinstalling"
	AndroidStatusError        = "error"
)

// Android Image Type constants per PRD Section 69.3
const (
	AndroidImageTypeStandard  = "standard"
	AndroidImageTypeDeveloper = "developer"
	AndroidImageTypeTesting   = "testing"
	AndroidImageTypeCustom    = "custom"
)

// Android Architecture constants
const (
	AndroidArchX86_64 = "x86_64"
	AndroidArchARM64  = "arm64"
)

// AndroidImage represents an approved Waydroid system and vendor image
type AndroidImage struct {
	ID             uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Name           string    `gorm:"type:varchar(120);not null" json:"name"`
	Slug           string    `gorm:"type:varchar(100);not null;uniqueIndex" json:"slug"`
	AndroidVersion string    `gorm:"type:varchar(32);not null;index" json:"android_version"`
	Architecture   string    `gorm:"type:varchar(32);not null;default:'x86_64'" json:"architecture"`
	ImageType      string    `gorm:"type:varchar(32);not null;default:'standard'" json:"image_type"` // standard, developer, testing, custom
	SystemImageURL string    `gorm:"type:varchar(255);not null" json:"system_image_url"`
	VendorImageURL string    `gorm:"type:varchar(255);not null" json:"vendor_image_url"`
	ImageSource    string    `gorm:"type:varchar(64);not null;default:'official'" json:"image_source"`
	ChecksumSHA256 string    `gorm:"type:varchar(64);not null" json:"checksum_sha256"`
	MinCPU         int       `gorm:"default:2" json:"min_cpu"`
	MinRAMMB       int       `gorm:"default:4096" json:"min_ram_mb"`
	MinStorageGB   int       `gorm:"default:32" json:"min_storage_gb"`
	IsPublished    bool      `gorm:"default:true;index" json:"is_published"`
	IsVerified     bool      `gorm:"default:true" json:"is_verified"`
	Description    string    `gorm:"type:text" json:"description"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`

	Versions []AndroidImageVersion `gorm:"foreignKey:ImageID" json:"versions,omitempty"`
}

func (AndroidImage) TableName() string {
	return "android_images"
}

// AndroidImageVersion represents version releases for an Android image
type AndroidImageVersion struct {
	ID             uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ImageID        uuid.UUID `gorm:"type:uuid;not null;index" json:"image_id"`
	VersionTag     string    `gorm:"type:varchar(50);not null" json:"version_tag"`
	Channel        string    `gorm:"type:varchar(32);not null;default:'stable'" json:"channel"`
	ReleaseNotes   string    `gorm:"type:text" json:"release_notes,omitempty"`
	SystemChecksum string    `gorm:"type:varchar(64)" json:"system_checksum,omitempty"`
	VendorChecksum string    `gorm:"type:varchar(64)" json:"vendor_checksum,omitempty"`
	IsActive       bool      `gorm:"default:true" json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
}

func (AndroidImageVersion) TableName() string {
	return "android_image_versions"
}

// AndroidWorkerNode represents a dedicated Linux hypervisor node running Waydroid
type AndroidWorkerNode struct {
	ID                   uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Name                 string    `gorm:"type:varchar(100);not null" json:"name"`
	ProxmoxNodeName      string    `gorm:"type:varchar(100);not null;default:'pve-dell-r640'" json:"proxmox_node_name"`
	IPAddress            string    `gorm:"type:varchar(45);not null" json:"ip_address"`
	Status               string    `gorm:"type:varchar(32);not null;default:'ready';index" json:"status"`
	KernelVersion        string    `gorm:"type:varchar(64);not null;default:'6.8.12-waydroid-pve'" json:"kernel_version"`
	BinderStatus         string    `gorm:"type:varchar(32);not null;default:'active'" json:"binder_status"`
	TotalCPUCores        int       `gorm:"not null;default:32" json:"total_cpu_cores"`
	TotalMemoryMB        int       `gorm:"not null;default:131072" json:"total_memory_mb"`
	TotalStorageGB       int       `gorm:"not null;default:2048" json:"total_storage_gb"`
	ActiveInstancesCount int       `gorm:"not null;default:0" json:"active_instances_count"`
	LastHealthCheckAt    time.Time `json:"last_health_check_at"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

func (AndroidWorkerNode) TableName() string {
	return "android_worker_nodes"
}

// AndroidInstance represents a customer managed Waydroid container environment
type AndroidInstance struct {
	ID                 uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	OwnerID            uuid.UUID      `gorm:"type:uuid;not null;index" json:"owner_id"`
	ImageID            uuid.UUID      `gorm:"type:uuid;not null" json:"image_id"`
	WorkerNodeID       *uuid.UUID     `gorm:"type:uuid" json:"worker_node_id,omitempty"`
	Name               string         `gorm:"type:varchar(120);not null" json:"name"`
	Slug               string         `gorm:"type:varchar(120);not null;index" json:"slug"`
	Status             string         `gorm:"type:varchar(32);not null;default:'running';index" json:"status"`
	PlanName           string         `gorm:"type:varchar(50);not null;default:'standard'" json:"plan_name"`
	VCPUs              int            `gorm:"column:vcpus;not null;default:2" json:"vcpus"`
	MemoryMB           int            `gorm:"column:memory_mb;not null;default:4096" json:"memory_mb"`
	StorageGB          int            `gorm:"column:storage_gb;not null;default:32" json:"storage_gb"`
	DisplayResolution  string         `gorm:"type:varchar(32);not null;default:'1280x720'" json:"display_resolution"`
	DisplayDensity     int            `gorm:"not null;default:240" json:"display_density"`
	Orientation        string         `gorm:"type:varchar(20);not null;default:'portrait'" json:"orientation"` // portrait, landscape
	RemoteAccessType   string         `gorm:"type:varchar(32);not null;default:'browser'" json:"remote_access_type"`
	AccessURL          string         `gorm:"type:varchar(255);not null" json:"access_url"`
	SessionToken       string         `gorm:"type:varchar(128)" json:"session_token,omitempty"`
	IPAddress          string         `gorm:"type:varchar(45);not null;default:'10.192.0.10'" json:"ip_address"`
	GatewayPort        int            `gorm:"not null;default:6080" json:"gateway_port"`
	ADBEnabled         bool           `gorm:"not null;default:false" json:"adb_enabled"`
	ADBPort            int            `gorm:"default:0" json:"adb_port,omitempty"`
	ADBAuthKey         string         `gorm:"type:varchar(128)" json:"adb_auth_key,omitempty"`
	PersistenceEnabled bool           `gorm:"not null;default:true" json:"persistence_enabled"`
	AutoBackupEnabled  bool           `gorm:"not null;default:true" json:"auto_backup_enabled"`
	BackupSchedule     string         `gorm:"type:varchar(50);not null;default:'daily'" json:"backup_schedule"`
	UptimeSeconds      int64          `gorm:"not null;default:0" json:"uptime_seconds"`
	CPUPercent         float64        `gorm:"type:numeric(5,2);not null;default:0.00" json:"cpu_percent"`
	MemoryUsageMB      int            `gorm:"not null;default:0" json:"memory_usage_mb"`
	StorageUsageGB     float64        `gorm:"type:numeric(6,2);not null;default:0.00" json:"storage_usage_gb"`
	LastBackupAt       *time.Time     `json:"last_backup_at,omitempty"`
	LastAccessedAt     *time.Time     `json:"last_accessed_at,omitempty"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	Image        *AndroidImage            `gorm:"foreignKey:ImageID" json:"image,omitempty"`
	WorkerNode   *AndroidWorkerNode       `gorm:"foreignKey:WorkerNodeID" json:"worker_node,omitempty"`
	Resources    *AndroidInstanceResource `gorm:"foreignKey:InstanceID" json:"resources,omitempty"`
	Applications []AndroidApplication     `gorm:"foreignKey:InstanceID" json:"applications,omitempty"`
	Backups      []AndroidBackup          `gorm:"foreignKey:InstanceID" json:"backups,omitempty"`
	Sessions     []AndroidSession         `gorm:"foreignKey:InstanceID" json:"sessions,omitempty"`
}

func (AndroidInstance) TableName() string {
	return "android_instances"
}

// AndroidInstanceResource details limits and priorities
type AndroidInstanceResource struct {
	ID              uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	InstanceID      uuid.UUID `gorm:"type:uuid;not null;index" json:"instance_id"`
	CPULimit        int       `gorm:"not null;default:2" json:"cpu_limit"`
	MemoryLimitMB   int       `gorm:"not null;default:4096" json:"memory_limit_mb"`
	StorageLimitGB  int       `gorm:"not null;default:32" json:"storage_limit_gb"`
	GPUAcceleration bool      `gorm:"not null;default:false" json:"gpu_acceleration"`
	IOPriority      string    `gorm:"type:varchar(20);not null;default:'standard'" json:"io_priority"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (AndroidInstanceResource) TableName() string {
	return "android_instance_resources"
}

// AndroidDataVolume represents persistent data storage on ZFS
type AndroidDataVolume struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	InstanceID  uuid.UUID `gorm:"type:uuid;not null;index" json:"instance_id"`
	VolumeName  string    `gorm:"type:varchar(100);not null" json:"volume_name"`
	MountPath   string    `gorm:"type:varchar(255);not null;default:'/data'" json:"mount_path"`
	ZFSDataset  string    `gorm:"type:varchar(255);not null" json:"zfs_dataset"`
	SizeBytes   int64     `gorm:"not null;default:0" json:"size_bytes"`
	IsEncrypted bool      `gorm:"not null;default:false" json:"is_encrypted"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (AndroidDataVolume) TableName() string {
	return "android_data_volumes"
}

// AndroidApplication represents installed APKs inside an instance
type AndroidApplication struct {
	ID             uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	InstanceID     uuid.UUID  `gorm:"type:uuid;not null;index" json:"instance_id"`
	PackageName    string     `gorm:"type:varchar(150);not null;index" json:"package_name"`
	AppName        string     `gorm:"type:varchar(120);not null" json:"app_name"`
	VersionName    string     `gorm:"type:varchar(64);not null;default:'1.0.0'" json:"version_name"`
	VersionCode    int        `gorm:"not null;default:1" json:"version_code"`
	FileSizeBytes  int64      `gorm:"not null;default:0" json:"file_size_bytes"`
	IconURL        string     `gorm:"type:varchar(255)" json:"icon_url,omitempty"`
	IsSystemApp    bool       `gorm:"not null;default:false" json:"is_system_app"`
	IsRunning      bool       `gorm:"not null;default:false" json:"is_running"`
	InstalledAt    time.Time  `json:"installed_at"`
	LastLaunchedAt *time.Time `json:"last_launched_at,omitempty"`
}

func (AndroidApplication) TableName() string {
	return "android_applications"
}

// AndroidSession tracks authenticated remote browser sessions
type AndroidSession struct {
	ID           uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	InstanceID   uuid.UUID  `gorm:"type:uuid;not null;index" json:"instance_id"`
	UserID       uuid.UUID  `gorm:"type:uuid;not null" json:"user_id"`
	SessionToken string     `gorm:"type:varchar(128);not null;uniqueIndex" json:"session_token"`
	AccessType   string     `gorm:"type:varchar(32);not null;default:'browser'" json:"access_type"`
	ClientIP     string     `gorm:"type:varchar(45);not null" json:"client_ip"`
	UserAgent    string     `gorm:"type:text" json:"user_agent,omitempty"`
	IsActive     bool       `gorm:"not null;default:true" json:"is_active"`
	ExpiresAt    time.Time  `gorm:"not null" json:"expires_at"`
	StartedAt    time.Time  `json:"started_at"`
	EndedAt      *time.Time `json:"ended_at,omitempty"`
}

func (AndroidSession) TableName() string {
	return "android_sessions"
}

// AndroidBackup represents persistent world / app data snapshots
type AndroidBackup struct {
	ID             uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	InstanceID     uuid.UUID `gorm:"type:uuid;not null;index" json:"instance_id"`
	UserID         uuid.UUID `gorm:"type:uuid;not null" json:"user_id"`
	BackupName     string    `gorm:"type:varchar(120);not null" json:"backup_name"`
	BackupType     string    `gorm:"type:varchar(32);not null;default:'snapshot'" json:"backup_type"`
	FileSizeBytes  int64     `gorm:"not null;default:0" json:"file_size_bytes"`
	StoragePath    string    `gorm:"type:varchar(255);not null" json:"storage_path"`
	ChecksumSHA256 string    `gorm:"type:varchar(64);not null" json:"checksum_sha256"`
	IsAutomated    bool      `gorm:"not null;default:false" json:"is_automated"`
	Status         string    `gorm:"type:varchar(32);not null;default:'completed'" json:"status"`
	RetentionDays  int       `gorm:"not null;default:30" json:"retention_days"`
	CreatedAt      time.Time `json:"created_at"`
}

func (AndroidBackup) TableName() string {
	return "android_backups"
}

// AndroidNetworkProfile represents isolated IP/bridge routing
type AndroidNetworkProfile struct {
	ID              uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	InstanceID      uuid.UUID `gorm:"type:uuid;not null;index" json:"instance_id"`
	NetworkBridge   string    `gorm:"type:varchar(50);not null;default:'waydroid0'" json:"network_bridge"`
	IPAddress       string    `gorm:"type:varchar(45);not null" json:"ip_address"`
	MACAddress      string    `gorm:"type:varchar(20);not null" json:"mac_address"`
	Gateway         string    `gorm:"type:varchar(45);not null;default:'10.192.0.1'" json:"gateway"`
	DNSServers      string    `gorm:"type:varchar(100);not null;default:'1.1.1.1,8.8.8.8'" json:"dns_servers"`
	FirewallProfile string    `gorm:"type:varchar(50);not null;default:'isolated_customer'" json:"firewall_profile"`
	CreatedAt       time.Time `json:"created_at"`
}

func (AndroidNetworkProfile) TableName() string {
	return "android_network_profiles"
}

// AndroidProvisioningJob tracks asynchronous instance actions
type AndroidProvisioningJob struct {
	ID              uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	InstanceID      uuid.UUID `gorm:"type:uuid;not null;index" json:"instance_id"`
	UserID          uuid.UUID `gorm:"type:uuid;not null" json:"user_id"`
	JobType         string    `gorm:"type:varchar(50);not null;default:'provision'" json:"job_type"`
	CurrentStep     string    `gorm:"type:varchar(64);not null;default:'init'" json:"current_step"`
	ProgressPercent int       `gorm:"not null;default:0" json:"progress_percent"`
	Status          string    `gorm:"type:varchar(32);not null;default:'pending'" json:"status"`
	ErrorMessage    string    `gorm:"type:text" json:"error_message,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (AndroidProvisioningJob) TableName() string {
	return "android_provisioning_jobs"
}

// AndroidAuditLog records all sensitive operations
type AndroidAuditLog struct {
	ID          uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	InstanceID  *uuid.UUID `gorm:"type:uuid;index" json:"instance_id,omitempty"`
	ActorUserID *uuid.UUID `gorm:"type:uuid" json:"actor_user_id,omitempty"`
	Action      string     `gorm:"type:varchar(64);not null" json:"action"`
	ClientIP    string     `gorm:"type:varchar(45)" json:"client_ip,omitempty"`
	DetailsJSON string     `gorm:"type:text" json:"details_json,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

func (AndroidAuditLog) TableName() string {
	return "android_audit_logs"
}
