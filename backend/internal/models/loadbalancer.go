package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// LoadBalancer represents a managed HAProxy load balancer instance.
type LoadBalancer struct {
	ID                  uuid.UUID                    `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	TenantID            uuid.UUID                    `gorm:"type:uuid;not null;index" json:"tenant_id"`
	ProjectID           string                       `gorm:"type:varchar(64);not null;default:'default';index" json:"project_id"`
	Name                string                       `gorm:"type:varchar(120);not null" json:"name"`
	NetworkID           *uuid.UUID                   `gorm:"type:uuid" json:"network_id,omitempty"`
	Provider            string                       `gorm:"type:varchar(32);not null;default:'haproxy'" json:"provider"`
	ProviderResourceID  string                       `gorm:"type:varchar(120);not null;default:''" json:"provider_resource_id"`
	PublicIP            string                       `gorm:"type:varchar(64);not null;default:''" json:"public_ip"`
	Algorithm           string                       `gorm:"type:varchar(32);not null;default:'round_robin'" json:"algorithm"` // round_robin, least_conn, source_ip, weighted_round_robin
	Status              string                       `gorm:"type:varchar(32);not null;default:'active'" json:"status"`         // provisioning, active, degraded, failed
	ConfigVersion       int                          `gorm:"not null;default:1" json:"config_version"`
	ActiveConfigVersion int                          `gorm:"not null;default:1" json:"active_config_version"`
	CreatedAt           time.Time                    `json:"created_at"`
	UpdatedAt           time.Time                    `json:"updated_at"`
	Listeners           []LoadBalancerListener       `gorm:"foreignKey:LoadBalancerID" json:"listeners,omitempty"`
	Pools               []LoadBalancerPool           `gorm:"foreignKey:LoadBalancerID" json:"pools,omitempty"`
	Certificates        []LoadBalancerCertificate    `gorm:"foreignKey:LoadBalancerID" json:"certificates,omitempty"`
	ConfigRevisions     []LoadBalancerConfigRevision `gorm:"foreignKey:LoadBalancerID" json:"config_revisions,omitempty"`
}

func (lb *LoadBalancer) BeforeCreate(tx *gorm.DB) error {
	if lb.ID == uuid.Nil {
		lb.ID = uuid.New()
	}
	return nil
}

func (LoadBalancer) TableName() string { return "load_balancers" }

// LoadBalancerListener defines the ingress frontend port and protocol.
type LoadBalancerListener struct {
	ID             uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	TenantID       uuid.UUID  `gorm:"type:uuid;not null;index" json:"tenant_id"`
	LoadBalancerID uuid.UUID  `gorm:"type:uuid;not null;index" json:"load_balancer_id"`
	Protocol       string     `gorm:"type:varchar(16);not null;default:'http'" json:"protocol"` // http, https, tcp
	FrontendPort   int        `gorm:"not null;default:80" json:"frontend_port"`
	CertificateID  *uuid.UUID `gorm:"type:uuid" json:"certificate_id,omitempty"`
	DefaultPoolID  *uuid.UUID `gorm:"type:uuid" json:"default_pool_id,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func (lbl *LoadBalancerListener) BeforeCreate(tx *gorm.DB) error {
	if lbl.ID == uuid.Nil {
		lbl.ID = uuid.New()
	}
	return nil
}

func (LoadBalancerListener) TableName() string { return "load_balancer_listeners" }

// LoadBalancerPool represents a backend target pool with health check parameters.
type LoadBalancerPool struct {
	ID                uuid.UUID            `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	TenantID          uuid.UUID            `gorm:"type:uuid;not null;index" json:"tenant_id"`
	LoadBalancerID    uuid.UUID            `gorm:"type:uuid;not null;index" json:"load_balancer_id"`
	Name              string               `gorm:"type:varchar(120);not null" json:"name"`
	Protocol          string               `gorm:"type:varchar(16);not null;default:'http'" json:"protocol"` // http, https, tcp
	Port              int                  `gorm:"not null;default:8080" json:"port"`
	Algorithm         string               `gorm:"type:varchar(32);not null;default:'round_robin'" json:"algorithm"`
	HealthCheckConfig string               `gorm:"type:text;not null;default:'{}'" json:"health_check_config"`
	CreatedAt         time.Time            `json:"created_at"`
	UpdatedAt         time.Time            `json:"updated_at"`
	Members           []LoadBalancerMember `gorm:"foreignKey:PoolID" json:"members,omitempty"`
}

func (lbp *LoadBalancerPool) BeforeCreate(tx *gorm.DB) error {
	if lbp.ID == uuid.Nil {
		lbp.ID = uuid.New()
	}
	return nil
}

func (LoadBalancerPool) TableName() string { return "load_balancer_pools" }

// LoadBalancerMember represents a VM backend receiving traffic.
type LoadBalancerMember struct {
	ID                uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	TenantID          uuid.UUID  `gorm:"type:uuid;not null;index" json:"tenant_id"`
	PoolID            uuid.UUID  `gorm:"type:uuid;not null;index" json:"pool_id"`
	InstanceID        uuid.UUID  `gorm:"type:uuid;not null;index" json:"instance_id"`
	Address           string     `gorm:"type:varchar(64);not null;default:''" json:"address"`
	Port              int        `gorm:"not null;default:8080" json:"port"`
	Weight            int        `gorm:"not null;default:100" json:"weight"`
	Enabled           bool       `gorm:"not null;default:true" json:"enabled"`
	HealthStatus      string     `gorm:"type:varchar(32);not null;default:'healthy'" json:"health_status"` // healthy, unhealthy, draining
	LastHealthCheckAt *time.Time `json:"last_health_check_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`

	Instance *Instance `gorm:"foreignKey:InstanceID" json:"instance,omitempty"`
}

func (lbm *LoadBalancerMember) BeforeCreate(tx *gorm.DB) error {
	if lbm.ID == uuid.Nil {
		lbm.ID = uuid.New()
	}
	return nil
}

func (LoadBalancerMember) TableName() string { return "load_balancer_members" }

// LoadBalancerCertificate stores TLS certificates and encrypted private keys for TLS termination.
type LoadBalancerCertificate struct {
	ID                   uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	TenantID             uuid.UUID  `gorm:"type:uuid;not null;index" json:"tenant_id"`
	LoadBalancerID       *uuid.UUID `gorm:"type:uuid" json:"load_balancer_id,omitempty"`
	Domain               string     `gorm:"type:varchar(255);not null" json:"domain"`
	CertificateEncrypted string     `gorm:"type:text;not null" json:"certificate"`
	PrivateKeyEncrypted  string     `gorm:"type:text;not null" json:"-"`
	ExpiresAt            time.Time  `json:"expires_at"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

func (lbc *LoadBalancerCertificate) BeforeCreate(tx *gorm.DB) error {
	if lbc.ID == uuid.Nil {
		lbc.ID = uuid.New()
	}
	return nil
}

func (LoadBalancerCertificate) TableName() string { return "load_balancer_certificates" }

// LoadBalancerConfigRevision maintains versioned HAProxy configs for last-known-good rollback.
type LoadBalancerConfigRevision struct {
	ID               uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	TenantID         uuid.UUID `gorm:"type:uuid;not null;index" json:"tenant_id"`
	LoadBalancerID   uuid.UUID `gorm:"type:uuid;not null;index" json:"load_balancer_id"`
	Version          int       `gorm:"not null" json:"version"`
	ConfigHash       string    `gorm:"type:varchar(64);not null" json:"config_hash"`
	Configuration    string    `gorm:"type:text;not null" json:"configuration"`
	DeploymentStatus string    `gorm:"type:varchar(32);not null;default:'applied'" json:"deployment_status"` // applied, failed, rolled_back
	ErrorMessage     string    `gorm:"type:text;not null;default:''" json:"error_message"`
	CreatedAt        time.Time `json:"created_at"`
}

func (cr *LoadBalancerConfigRevision) BeforeCreate(tx *gorm.DB) error {
	if cr.ID == uuid.Nil {
		cr.ID = uuid.New()
	}
	return nil
}

func (LoadBalancerConfigRevision) TableName() string { return "load_balancer_config_revisions" }
