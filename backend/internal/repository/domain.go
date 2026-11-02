package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"gorm.io/gorm"
)

// ErrHostnameTaken is returned by DomainRepository.Create when another
// active domain already uses the hostname (unique index
// uq_domains_hostname_active).
var ErrHostnameTaken = errors.New("hostname is already in use")

// domainClaimLockClass namespaces the per-owner advisory locks taken by
// LockOwner (two-key form: class, hashtext(owner id)).
const domainClaimLockClass = 0x4e56_444d // "NVDM"

// DomainRepository persists Nevatal Domains (domains). A domain is active
// while released_at is NULL; released rows are kept for the quarantine
// window and history and are excluded from every list.
type DomainRepository interface {
	// ListActiveByOwner returns the owner's active domains (newest first)
	// with Owner and Instance preloaded. A deleted instance is preloaded too
	// (check Instance.DeletedAt).
	ListActiveByOwner(ctx context.Context, ownerID uuid.UUID) ([]models.Domain, error)
	// ListActive returns every active domain (newest first) with Owner and
	// Instance preloaded, as ListActiveByOwner.
	ListActive(ctx context.Context) ([]models.Domain, error)
	// FindActiveByID returns an active domain with Owner and Instance
	// preloaded. With ownerID set, domains of other owners are not found.
	FindActiveByID(ctx context.Context, id uuid.UUID, ownerID *uuid.UUID) (*models.Domain, error)
	CountActiveByOwner(ctx context.Context, ownerID uuid.UUID) (int64, error)
	// HostnameActive reports whether an active domain uses hostname
	// (case-insensitive).
	HostnameActive(ctx context.Context, hostname string) (bool, error)
	// HostnameQuarantined reports whether hostname was released at or after
	// since by an owner other than claimantID, or released while disabled
	// by an administrator (whoever the owner was).
	HostnameQuarantined(ctx context.Context, hostname string, claimantID uuid.UUID, since time.Time) (bool, error)
	// LockOwner takes a transaction-scoped advisory lock that serializes the
	// domain claims of one owner. Call it inside Transactor.WithinTransaction.
	LockOwner(ctx context.Context, ownerID uuid.UUID) error

	// Create inserts d. It returns ErrHostnameTaken when another active
	// domain has the same hostname.
	Create(ctx context.Context, d *models.Domain) error
	// UpdateSettings writes instance_id, target_port and enabled of d.
	UpdateSettings(ctx context.Context, d *models.Domain) error
	// UpdateAdminState writes admin_disabled and disabled_reason of d.
	UpdateAdminState(ctx context.Context, d *models.Domain) error
	// Release sets released_at of the active domain d. It returns
	// ErrNotFound when d is no longer active.
	Release(ctx context.Context, d *models.Domain, at time.Time) error

	// LoadEdgeSnapshot reads, in one read-only repeatable-read transaction,
	// every active domain with its owner status and non-deleted instance,
	// plus the private addresses of all non-deleted instances (for conflict
	// detection). Used by nevatal-edge.
	LoadEdgeSnapshot(ctx context.Context) (EdgeSnapshot, error)
}

// EdgeRoute is one active domain as nevatal-edge sees it.
type EdgeRoute struct {
	Hostname       string
	TargetPort     int
	Enabled        bool
	AdminDisabled  bool
	OwnerStatus    string
	InstanceFound  bool    // the target instance exists and is not deleted
	InstanceStatus *string // nil when the instance is missing
	PrivateIP      *string
}

// EdgeSnapshot is the routing data nevatal-edge works from.
type EdgeSnapshot struct {
	Routes     []EdgeRoute
	PrivateIPs []string // of every non-deleted instance; duplicates kept
}

type domainRepository struct{ base }

var _ DomainRepository = (*domainRepository)(nil)

func NewDomainRepository(db *gorm.DB) DomainRepository {
	return &domainRepository{base{db}}
}

// withRelations preloads the owner and the target instance, including a
// soft-deleted instance so its name can still be shown.
func withRelations(db *gorm.DB) *gorm.DB {
	return db.Preload("Owner").Preload("Instance", func(tx *gorm.DB) *gorm.DB { return tx.Unscoped() })
}

func (r *domainRepository) ListActiveByOwner(ctx context.Context, ownerID uuid.UUID) ([]models.Domain, error) {
	var domains []models.Domain
	err := withRelations(r.conn(ctx)).
		Where("owner_id = ? AND released_at IS NULL", ownerID).
		Order("created_at desc").
		Find(&domains).Error
	return domains, err
}

func (r *domainRepository) ListActive(ctx context.Context) ([]models.Domain, error) {
	var domains []models.Domain
	err := withRelations(r.conn(ctx)).
		Where("released_at IS NULL").
		Order("created_at desc").
		Find(&domains).Error
	return domains, err
}

func (r *domainRepository) FindActiveByID(ctx context.Context, id uuid.UUID, ownerID *uuid.UUID) (*models.Domain, error) {
	var d models.Domain
	query := withRelations(r.conn(ctx)).Where("id = ? AND released_at IS NULL", id)
	if ownerID != nil {
		query = query.Where("owner_id = ?", *ownerID)
	}
	if err := query.First(&d).Error; err != nil {
		return nil, mapErr(err)
	}
	return &d, nil
}

func (r *domainRepository) CountActiveByOwner(ctx context.Context, ownerID uuid.UUID) (int64, error) {
	var n int64
	err := r.conn(ctx).Model(&models.Domain{}).Where("owner_id = ? AND released_at IS NULL", ownerID).Count(&n).Error
	return n, err
}

func (r *domainRepository) HostnameActive(ctx context.Context, hostname string) (bool, error) {
	var n int64
	err := r.conn(ctx).Model(&models.Domain{}).
		Where("lower(hostname) = lower(?) AND released_at IS NULL", hostname).
		Count(&n).Error
	return n > 0, err
}

func (r *domainRepository) HostnameQuarantined(ctx context.Context, hostname string, claimantID uuid.UUID, since time.Time) (bool, error) {
	var n int64
	err := r.conn(ctx).Model(&models.Domain{}).
		Where("lower(hostname) = lower(?) AND released_at IS NOT NULL AND released_at >= ?", hostname, since).
		Where("owner_id <> ? OR admin_disabled", claimantID).
		Count(&n).Error
	return n > 0, err
}

func (r *domainRepository) LockOwner(ctx context.Context, ownerID uuid.UUID) error {
	return r.conn(ctx).Exec("SELECT pg_advisory_xact_lock(?, hashtext(?))", domainClaimLockClass, ownerID.String()).Error
}

func (r *domainRepository) Create(ctx context.Context, d *models.Domain) error {
	err := r.conn(ctx).Create(d).Error
	if r.isUniqueViolation(err) {
		return ErrHostnameTaken
	}
	return err
}

func (r *domainRepository) UpdateSettings(ctx context.Context, d *models.Domain) error {
	return r.conn(ctx).Model(d).Select("instance_id", "target_port", "enabled", "updated_at").Updates(d).Error
}

func (r *domainRepository) UpdateAdminState(ctx context.Context, d *models.Domain) error {
	return r.conn(ctx).Model(d).Select("admin_disabled", "disabled_reason", "updated_at").Updates(d).Error
}

func (r *domainRepository) Release(ctx context.Context, d *models.Domain, at time.Time) error {
	res := r.conn(ctx).Model(&models.Domain{}).
		Where("id = ? AND released_at IS NULL", d.ID).
		Updates(map[string]any{"released_at": at, "updated_at": at})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	d.ReleasedAt = &at
	return nil
}

func (r *domainRepository) LoadEdgeSnapshot(ctx context.Context) (EdgeSnapshot, error) {
	var snap EdgeSnapshot
	err := r.conn(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Table("domains AS d").
			Select(`d.hostname, d.target_port, d.enabled, d.admin_disabled, u.status AS owner_status,
				i.id IS NOT NULL AS instance_found, i.status AS instance_status, i.private_ip`).
			Joins("JOIN users u ON u.id = d.owner_id").
			Joins("LEFT JOIN instances i ON i.id = d.instance_id AND i.deleted_at IS NULL").
			Where("d.released_at IS NULL").
			Scan(&snap.Routes).Error
		if err != nil {
			return err
		}
		return tx.Model(&models.Instance{}).
			Where("deleted_at IS NULL AND private_ip IS NOT NULL AND private_ip <> ''").
			Pluck("private_ip", &snap.PrivateIPs).Error
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return snap, err
}

// isUniqueViolation reports whether err is a unique-constraint violation
// (SQLSTATE 23505). It translates through the dialect locally instead of
// enabling gorm.Config.TranslateError, which would change the error text
// other endpoints echo to clients.
func (r *domainRepository) isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	if translator, ok := r.db.Dialector.(gorm.ErrorTranslator); ok && errors.Is(translator.Translate(err), gorm.ErrDuplicatedKey) {
		return true
	}
	return strings.Contains(err.Error(), "SQLSTATE 23505")
}
