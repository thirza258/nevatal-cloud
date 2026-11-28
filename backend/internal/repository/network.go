package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"gorm.io/gorm"
)

type NetworkRepository interface {
	CreateNetwork(ctx context.Context, net *models.Network) error
	ListNetworksByTenant(ctx context.Context, tenantID uuid.UUID) ([]models.Network, error)
	GetNetworkByID(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) (*models.Network, error)
	UpdateNetwork(ctx context.Context, net *models.Network) error
	DeleteNetwork(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) error

	AttachInstance(ctx context.Context, att *models.NetworkAttachment) error
	DetachInstance(ctx context.Context, tenantID uuid.UUID, networkID uuid.UUID, attachmentID uuid.UUID) error
	ListAttachmentsByNetwork(ctx context.Context, tenantID uuid.UUID, networkID uuid.UUID) ([]models.NetworkAttachment, error)
	ListAttachmentsByTenant(ctx context.Context, tenantID uuid.UUID) ([]models.NetworkAttachment, error)

	CreateFirewall(ctx context.Context, fw *models.Firewall) error
	ListFirewallsByTenant(ctx context.Context, tenantID uuid.UUID) ([]models.Firewall, error)
	GetFirewallByID(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) (*models.Firewall, error)
	UpdateFirewall(ctx context.Context, fw *models.Firewall) error
	DeleteFirewall(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) error

	AddFirewallRule(ctx context.Context, rule *models.FirewallRule) error
	UpdateFirewallRule(ctx context.Context, rule *models.FirewallRule) error
	DeleteFirewallRule(ctx context.Context, tenantID uuid.UUID, firewallID uuid.UUID, ruleID uuid.UUID) error
	AttachFirewall(ctx context.Context, att *models.FirewallAttachment) error
	DetachFirewall(ctx context.Context, tenantID uuid.UUID, firewallID uuid.UUID, attachmentID uuid.UUID) error

	ListFloatingIPsByTenant(ctx context.Context, tenantID uuid.UUID) ([]models.FloatingIP, error)
	GetFloatingIPByID(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) (*models.FloatingIP, error)
	CreateFloatingIP(ctx context.Context, fip *models.FloatingIP) error
	UpdateFloatingIP(ctx context.Context, fip *models.FloatingIP) error
	DeleteFloatingIP(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) error

	ListDNSZonesByTenant(ctx context.Context, tenantID uuid.UUID) ([]models.DNSZone, error)
	GetDNSZoneByID(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) (*models.DNSZone, error)
	CreateDNSZone(ctx context.Context, zone *models.DNSZone) error
	DeleteDNSZone(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) error
	ListDNSRecordsByZone(ctx context.Context, tenantID uuid.UUID, zoneID uuid.UUID) ([]models.DNSRecord, error)
	CreateDNSRecord(ctx context.Context, rec *models.DNSRecord) error
	UpdateDNSRecord(ctx context.Context, rec *models.DNSRecord) error
	DeleteDNSRecord(ctx context.Context, tenantID uuid.UUID, zoneID uuid.UUID, recordID uuid.UUID) error
}

type networkRepository struct{ base }

var _ NetworkRepository = (*networkRepository)(nil)

func NewNetworkRepository(db *gorm.DB) NetworkRepository {
	return &networkRepository{base{db}}
}

func deleteChecked(res *gorm.DB) error {
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *networkRepository) CreateNetwork(ctx context.Context, net *models.Network) error {
	return r.conn(ctx).Create(net).Error
}

func (r *networkRepository) ListNetworksByTenant(ctx context.Context, tenantID uuid.UUID) ([]models.Network, error) {
	var networks []models.Network
	err := r.conn(ctx).
		Where("tenant_id = ?", tenantID).
		Preload("Attachments").
		Preload("Attachments.Instance").
		Order("created_at DESC").
		Find(&networks).Error
	return networks, err
}

func (r *networkRepository) GetNetworkByID(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) (*models.Network, error) {
	var net models.Network
	err := r.conn(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Preload("Attachments").
		Preload("Attachments.Instance").
		First(&net).Error
	if err != nil {
		return nil, mapErr(err)
	}
	return &net, nil
}

func (r *networkRepository) UpdateNetwork(ctx context.Context, net *models.Network) error {
	return r.conn(ctx).Save(net).Error
}

func (r *networkRepository) DeleteNetwork(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) error {
	return deleteChecked(r.conn(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&models.Network{}))
}

func (r *networkRepository) AttachInstance(ctx context.Context, att *models.NetworkAttachment) error {
	return r.conn(ctx).Create(att).Error
}

func (r *networkRepository) DetachInstance(ctx context.Context, tenantID uuid.UUID, networkID uuid.UUID, attachmentID uuid.UUID) error {
	return deleteChecked(r.conn(ctx).
		Where("id = ? AND network_id = ? AND tenant_id = ?", attachmentID, networkID, tenantID).
		Delete(&models.NetworkAttachment{}))
}

func (r *networkRepository) ListAttachmentsByNetwork(ctx context.Context, tenantID uuid.UUID, networkID uuid.UUID) ([]models.NetworkAttachment, error) {
	var atts []models.NetworkAttachment
	err := r.conn(ctx).
		Where("network_id = ? AND tenant_id = ?", networkID, tenantID).
		Preload("Instance").
		Find(&atts).Error
	return atts, err
}

func (r *networkRepository) ListAttachmentsByTenant(ctx context.Context, tenantID uuid.UUID) ([]models.NetworkAttachment, error) {
	var atts []models.NetworkAttachment
	err := r.conn(ctx).
		Where("tenant_id = ?", tenantID).
		Preload("Instance").
		Find(&atts).Error
	return atts, err
}

func (r *networkRepository) CreateFirewall(ctx context.Context, fw *models.Firewall) error {
	return r.conn(ctx).Create(fw).Error
}

func (r *networkRepository) ListFirewallsByTenant(ctx context.Context, tenantID uuid.UUID) ([]models.Firewall, error) {
	var firewalls []models.Firewall
	err := r.conn(ctx).
		Where("tenant_id = ?", tenantID).
		Preload("Rules", func(db *gorm.DB) *gorm.DB {
			return db.Order("priority ASC")
		}).
		Preload("Attachments").
		Order("created_at DESC").
		Find(&firewalls).Error
	return firewalls, err
}

func (r *networkRepository) GetFirewallByID(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) (*models.Firewall, error) {
	var fw models.Firewall
	err := r.conn(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Preload("Rules", func(db *gorm.DB) *gorm.DB {
			return db.Order("priority ASC")
		}).
		Preload("Attachments").
		First(&fw).Error
	if err != nil {
		return nil, mapErr(err)
	}
	return &fw, nil
}

func (r *networkRepository) UpdateFirewall(ctx context.Context, fw *models.Firewall) error {
	return r.conn(ctx).Save(fw).Error
}

func (r *networkRepository) DeleteFirewall(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) error {
	return deleteChecked(r.conn(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&models.Firewall{}))
}

func (r *networkRepository) AddFirewallRule(ctx context.Context, rule *models.FirewallRule) error {
	return r.conn(ctx).Create(rule).Error
}

func (r *networkRepository) UpdateFirewallRule(ctx context.Context, rule *models.FirewallRule) error {
	return r.conn(ctx).Save(rule).Error
}

func (r *networkRepository) DeleteFirewallRule(ctx context.Context, tenantID uuid.UUID, firewallID uuid.UUID, ruleID uuid.UUID) error {
	return deleteChecked(r.conn(ctx).
		Where("id = ? AND firewall_id = ? AND tenant_id = ?", ruleID, firewallID, tenantID).
		Delete(&models.FirewallRule{}))
}

func (r *networkRepository) AttachFirewall(ctx context.Context, att *models.FirewallAttachment) error {
	return r.conn(ctx).Create(att).Error
}

func (r *networkRepository) DetachFirewall(ctx context.Context, tenantID uuid.UUID, firewallID uuid.UUID, attachmentID uuid.UUID) error {
	return deleteChecked(r.conn(ctx).
		Where("id = ? AND firewall_id = ? AND tenant_id = ?", attachmentID, firewallID, tenantID).
		Delete(&models.FirewallAttachment{}))
}

func (r *networkRepository) ListFloatingIPsByTenant(ctx context.Context, tenantID uuid.UUID) ([]models.FloatingIP, error) {
	var fips []models.FloatingIP
	err := r.conn(ctx).
		Where("tenant_id = ?", tenantID).
		Preload("Instance").
		Order("created_at DESC").
		Find(&fips).Error
	return fips, err
}

func (r *networkRepository) GetFloatingIPByID(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) (*models.FloatingIP, error) {
	var fip models.FloatingIP
	err := r.conn(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Preload("Instance").
		First(&fip).Error
	if err != nil {
		return nil, mapErr(err)
	}
	return &fip, nil
}

func (r *networkRepository) CreateFloatingIP(ctx context.Context, fip *models.FloatingIP) error {
	return r.conn(ctx).Create(fip).Error
}

func (r *networkRepository) UpdateFloatingIP(ctx context.Context, fip *models.FloatingIP) error {
	return r.conn(ctx).Save(fip).Error
}

func (r *networkRepository) DeleteFloatingIP(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) error {
	return deleteChecked(r.conn(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&models.FloatingIP{}))
}

func (r *networkRepository) ListDNSZonesByTenant(ctx context.Context, tenantID uuid.UUID) ([]models.DNSZone, error) {
	var zones []models.DNSZone
	err := r.conn(ctx).
		Where("tenant_id = ?", tenantID).
		Preload("Records").
		Order("created_at DESC").
		Find(&zones).Error
	return zones, err
}

func (r *networkRepository) GetDNSZoneByID(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) (*models.DNSZone, error) {
	var zone models.DNSZone
	err := r.conn(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Preload("Records").
		First(&zone).Error
	if err != nil {
		return nil, mapErr(err)
	}
	return &zone, nil
}

func (r *networkRepository) CreateDNSZone(ctx context.Context, zone *models.DNSZone) error {
	return r.conn(ctx).Create(zone).Error
}

func (r *networkRepository) DeleteDNSZone(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) error {
	return deleteChecked(r.conn(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&models.DNSZone{}))
}

func (r *networkRepository) ListDNSRecordsByZone(ctx context.Context, tenantID uuid.UUID, zoneID uuid.UUID) ([]models.DNSRecord, error) {
	var records []models.DNSRecord
	err := r.conn(ctx).
		Where("zone_id = ? AND tenant_id = ?", zoneID, tenantID).
		Order("created_at DESC").
		Find(&records).Error
	return records, err
}

func (r *networkRepository) CreateDNSRecord(ctx context.Context, rec *models.DNSRecord) error {
	return r.conn(ctx).Create(rec).Error
}

func (r *networkRepository) UpdateDNSRecord(ctx context.Context, rec *models.DNSRecord) error {
	return r.conn(ctx).Save(rec).Error
}

func (r *networkRepository) DeleteDNSRecord(ctx context.Context, tenantID uuid.UUID, zoneID uuid.UUID, recordID uuid.UUID) error {
	return deleteChecked(r.conn(ctx).
		Where("id = ? AND zone_id = ? AND tenant_id = ?", recordID, zoneID, tenantID).
		Delete(&models.DNSRecord{}))
}
