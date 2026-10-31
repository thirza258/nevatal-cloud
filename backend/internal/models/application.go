package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Application struct {
	ID                   uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	OwnerID              uuid.UUID  `gorm:"type:uuid;not null;index" json:"owner_id"`
	Name                 string     `gorm:"type:varchar(100);not null" json:"name"`
	Slug                 string     `gorm:"type:varchar(100);not null;index" json:"slug"`
	ApplicationType      string     `gorm:"type:varchar(50);not null" json:"application_type"`       // static, react, nextjs, nodejs, go, python, docker
	DeploymentSourceType string     `gorm:"type:varchar(50);not null" json:"deployment_source_type"` // git, template, docker
	RepositoryURL        string     `gorm:"type:varchar(255)" json:"repository_url"`
	ProductionBranch     string     `gorm:"type:varchar(100);default:'main'" json:"production_branch"`
	RootDirectory        string     `gorm:"type:varchar(255);default:'/'" json:"root_directory"`
	BuildCommand         string     `gorm:"type:varchar(255)" json:"build_command"`
	OutputDirectory      string     `gorm:"type:varchar(100);default:'dist'" json:"output_directory"`
	StartCommand         string     `gorm:"type:varchar(255)" json:"start_command"`
	ApplicationPort      int        `gorm:"default:8080" json:"application_port"`
	HealthCheckPath      string     `gorm:"type:varchar(100);default:'/'" json:"health_check_path"`
	AssignedSubdomain    string     `gorm:"type:varchar(150);unique;not null" json:"assigned_subdomain"`
	CustomDomain         string     `gorm:"type:varchar(255)" json:"custom_domain,omitempty"`
	Status               string     `gorm:"type:varchar(32);not null;default:'draft'" json:"status"` // draft, queued, building, deploying, running, stopped, failed
	VCPUs                int        `gorm:"default:1" json:"vcpus"`
	MemoryMB             int        `gorm:"default:1024" json:"memory_mb"`
	DiskGB               int        `gorm:"default:10" json:"disk_gb"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
	DeletedAt            *time.Time `json:"deleted_at,omitempty"`

	Owner       *User               `gorm:"foreignKey:OwnerID" json:"owner,omitempty"`
	Deployments []Deployment        `gorm:"foreignKey:ApplicationID" json:"deployments,omitempty"`
	EnvVars     []ApplicationEnvVar `gorm:"foreignKey:ApplicationID" json:"env_vars,omitempty"`
}

func (a *Application) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

func (Application) TableName() string {
	return "applications"
}

type Deployment struct {
	ID                 uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ApplicationID      uuid.UUID  `gorm:"type:uuid;not null;index" json:"application_id"`
	SourceCommit       string     `gorm:"type:varchar(40)" json:"source_commit"`
	CommitMessage      string     `gorm:"type:varchar(255)" json:"commit_message"`
	SourceBranch       string     `gorm:"type:varchar(100);not null" json:"source_branch"`
	Status             string     `gorm:"type:varchar(32);not null;default:'queued'" json:"status"` // queued, building, deploying, active, failed, superseded
	BuildLogs          string     `gorm:"type:text" json:"build_logs"`
	ArtifactReference  string     `gorm:"type:varchar(255)" json:"artifact_reference"`
	RuntimeImageDigest string     `gorm:"type:varchar(255)" json:"runtime_image_digest"`
	ErrorCode          string     `gorm:"type:varchar(100)" json:"error_code,omitempty"`
	ErrorMessage       string     `gorm:"type:text" json:"error_message,omitempty"`
	StartedAt          *time.Time `json:"started_at"`
	CompletedAt        *time.Time `json:"completed_at"`
	CreatedAt          time.Time  `json:"created_at"`
}

func (d *Deployment) BeforeCreate(tx *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	return nil
}

func (Deployment) TableName() string {
	return "deployments"
}

type ApplicationEnvVar struct {
	ID            uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ApplicationID uuid.UUID `gorm:"type:uuid;not null;index" json:"application_id"`
	Key           string    `gorm:"type:varchar(100);not null" json:"key"`
	Value         string    `gorm:"type:text;not null" json:"value"` // masked if secret in API responses
	IsSecret      bool      `gorm:"not null;default:false" json:"is_secret"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (ApplicationEnvVar) TableName() string {
	return "application_environment_variables"
}

type GitConnection struct {
	ID                uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	OwnerID           uuid.UUID `gorm:"type:uuid;not null;index" json:"owner_id"`
	Provider          string    `gorm:"type:varchar(50);not null" json:"provider"` // github, gitlab
	ProviderAccountID string    `gorm:"type:varchar(100);not null" json:"provider_account_id"`
	DisplayName       string    `gorm:"type:varchar(100);not null" json:"display_name"`
	Username          string    `gorm:"type:varchar(100);not null" json:"username"`
	AvatarURL         string    `gorm:"type:varchar(512)" json:"avatar_url"`
	Status            string    `gorm:"type:varchar(32);default:'connected'" json:"status"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func (GitConnection) TableName() string {
	return "git_connections"
}
