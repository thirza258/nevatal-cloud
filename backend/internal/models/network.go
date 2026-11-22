package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Network represents a tenant-isolated virtual private network.
type Network struct {
	ID                 uuid.UUID           `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	TenantID           uuid.UUID           `gorm:"type:uuid;not null;index" json:"tenant_id"`
	ProjectID          string              `gorm:"type:varchar(64);not null;default:'default';index" json:"project_id"`
	Name               string              `gorm:"type:varchar(120);not null" json:"name"`
	Description        string              `gorm:"type:text;not null;default:''" json:"description"`
	CIDR               string              `gorm:"type:varchar(64);not null" json:"cidr"`
	Gateway            string              `gorm:"type:varchar(64);not null;default:''" json:"gateway"`
	NetworkType        string              `gorm:"type:varchar(32);not null;default:'private'" json:"network_type"`
	Provider           string              `gorm:"type:varchar(32);not null;default:'proxmox_sdn'" json:"provider"`
	ProviderResourceID string              `gorm:"type:varchar(120);not null;default:''" json:"provider_resource_id"`
	Status             string              `gorm:"type:varchar(32);not null;default:'active'" json:"status"`
	CreatedAt          time.Time           `json:"created_at"`
	UpdatedAt          time.Time           `json:"updated_at"`
	Attachments        []NetworkAttachment `gorm:"foreignKey:NetworkID" json:"attachments,omitempty"`
}

func (n *Network) BeforeCreate(tx *gorm.DB) error {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	return nil
}

func (Network) TableName() string { return "networks" }

// NetworkAttachment represents an instance connected to a private network.
type NetworkAttachment struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	TenantID    uuid.UUID `gorm:"type:uuid;not null;index" json:"tenant_id"`
	NetworkID   uuid.UUID `gorm:"type:uuid;not null;index" json:"network_id"`
	InstanceID  uuid.UUID `gorm:"type:uuid;not null;index" json:"instance_id"`
	InterfaceID string    `gorm:"type:varchar(32);not null;default:'net1'" json:"interface_id"`
	IPAddress   string    `gorm:"type:varchar(64);not null;default:''" json:"ip_address"`
	Status      string    `gorm:"type:varchar(32);not null;default:'attached'" json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	Instance *Instance `gorm:"foreignKey:InstanceID" json:"instance,omitempty"`
}

func (na *NetworkAttachment) BeforeCreate(tx *gorm.DB) error {
	if na.ID == uuid.Nil {
		na.ID = uuid.New()
	}
	return nil
}

func (NetworkAttachment) TableName() string { return "network_attachments" }

// Firewall represents a cloud firewall policy.
type Firewall struct {
	ID          uuid.UUID            `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	TenantID    uuid.UUID            `gorm:"type:uuid;not null;index" json:"tenant_id"`
	ProjectID   string               `gorm:"type:varchar(64);not null;default:'default';index" json:"project_id"`
	Name        string               `gorm:"type:varchar(120);not null" json:"name"`
	Description string               `gorm:"type:text;not null;default:''" json:"description"`
	Status      string               `gorm:"type:varchar(32);not null;default:'active'" json:"status"`
	CreatedAt   time.Time            `json:"created_at"`
	UpdatedAt   time.Time            `json:"updated_at"`
	Rules       []FirewallRule       `gorm:"foreignKey:FirewallID" json:"rules,omitempty"`
	Attachments []FirewallAttachment `gorm:"foreignKey:FirewallID" json:"attachments,omitempty"`
}

func (f *Firewall) BeforeCreate(tx *gorm.DB) error {
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	return nil
}

func (Firewall) TableName() string { return "firewalls" }

// FirewallRule represents an individual inbound/outbound filter.
type FirewallRule struct {
	ID              uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	TenantID        uuid.UUID `gorm:"type:uuid;not null;index" json:"tenant_id"`
	FirewallID      uuid.UUID `gorm:"type:uuid;not null;index" json:"firewall_id"`
	Direction       string    `gorm:"type:varchar(16);not null;default:'inbound'" json:"direction"` // inbound, outbound
	Action          string    `gorm:"type:varchar(16);not null;default:'allow'" json:"action"`      // allow, deny
	Protocol        string    `gorm:"type:varchar(16);not null;default:'tcp'" json:"protocol"`      // tcp, udp, icmp, any
	SourceCIDR      string    `gorm:"type:varchar(64);not null;default:'0.0.0.0/0'" json:"source_cidr"`
	DestinationCIDR string    `gorm:"type:varchar(64);not null;default:'0.0.0.0/0'" json:"destination_cidr"`
	PortMin         int       `gorm:"not null;default:1" json:"port_min"`
	PortMax         int       `gorm:"not null;default:65535" json:"port_max"`
	Priority        int       `gorm:"not null;default:100" json:"priority"`
	Enabled         bool      `gorm:"not null;default:true" json:"enabled"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (fr *FirewallRule) BeforeCreate(tx *gorm.DB) error {
	if fr.ID == uuid.Nil {
		fr.ID = uuid.New()
	}
	return nil
}

func (FirewallRule) TableName() string { return "firewall_rules" }

// FirewallAttachment links a firewall to an instance or network.
type FirewallAttachment struct {
	ID           uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	TenantID     uuid.UUID `gorm:"type:uuid;not null;index" json:"tenant_id"`
	FirewallID   uuid.UUID `gorm:"type:uuid;not null;index" json:"firewall_id"`
	ResourceType string    `gorm:"type:varchar(64);not null;default:'vm'" json:"resource_type"`
	ResourceID   uuid.UUID `gorm:"type:uuid;not null" json:"resource_id"`
	CreatedAt    time.Time `json:"created_at"`
}

func (fa *FirewallAttachment) BeforeCreate(tx *gorm.DB) error {
	if fa.ID == uuid.Nil {
		fa.ID = uuid.New()
	}
	return nil
}

func (FirewallAttachment) TableName() string { return "firewall_attachments" }

// FloatingIP represents an allocatable public IPv4 address.
type FloatingIP struct {
	ID         uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	TenantID   uuid.UUID  `gorm:"type:uuid;not null;index" json:"tenant_id"`
	ProjectID  string     `gorm:"type:varchar(64);not null;default:'default';index" json:"project_id"`
	Address    string     `gorm:"type:varchar(64);not null;unique" json:"address"`
	IPVersion  int        `gorm:"not null;default:4" json:"ip_version"`
	Status     string     `gorm:"type:varchar(32);not null;default:'available'" json:"status"` // available, allocated, assigned, reassigning
	InstanceID *uuid.UUID `gorm:"type:uuid" json:"instance_id,omitempty"`
	Provider   string     `gorm:"type:varchar(32);not null;default:'upstream'" json:"provider"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`

	Instance *Instance `gorm:"foreignKey:InstanceID" json:"instance,omitempty"`
}

func (fip *FloatingIP) BeforeCreate(tx *gorm.DB) error {
	if fip.ID == uuid.Nil {
		fip.ID = uuid.New()
	}
	return nil
}

func (FloatingIP) TableName() string { return "floating_ips" }

// DNSZone represents a managed DNS zone.
type DNSZone struct {
	ID             uuid.UUID   `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	TenantID       uuid.UUID   `gorm:"type:uuid;not null;index" json:"tenant_id"`
	ProjectID      string      `gorm:"type:varchar(64);not null;default:'default';index" json:"project_id"`
	Domain         string      `gorm:"type:varchar(255);not null" json:"domain"`
	Provider       string      `gorm:"type:varchar(32);not null;default:'cloudflare'" json:"provider"`
	ProviderZoneID string      `gorm:"type:varchar(120);not null;default:''" json:"provider_zone_id"`
	Status         string      `gorm:"type:varchar(32);not null;default:'active'" json:"status"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
	Records        []DNSRecord `gorm:"foreignKey:ZoneID" json:"records,omitempty"`
}

func (z *DNSZone) BeforeCreate(tx *gorm.DB) error {
	if z.ID == uuid.Nil {
		z.ID = uuid.New()
	}
	return nil
}

func (DNSZone) TableName() string { return "dns_zones" }

// DNSRecord represents an entry within a DNS zone.
type DNSRecord struct {
	ID               uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	TenantID         uuid.UUID `gorm:"type:uuid;not null;index" json:"tenant_id"`
	ZoneID           uuid.UUID `gorm:"type:uuid;not null;index" json:"zone_id"`
	RecordType       string    `gorm:"type:varchar(16);not null;default:'A'" json:"record_type"` // A, AAAA, CNAME, MX, TXT, SRV, PTR
	Name             string    `gorm:"type:varchar(255);not null" json:"name"`
	Value            string    `gorm:"type:text;not null" json:"value"`
	TTL              int       `gorm:"not null;default:300" json:"ttl"`
	ProviderRecordID string    `gorm:"type:varchar(120);not null;default:''" json:"provider_record_id"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func (r *DNSRecord) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

func (DNSRecord) TableName() string { return "dns_records" }
