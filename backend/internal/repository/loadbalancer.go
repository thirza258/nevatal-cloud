package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"gorm.io/gorm"
)

type LoadBalancerRepository interface {
	Create(ctx context.Context, lb *models.LoadBalancer) error
	ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]models.LoadBalancer, error)
	GetByID(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) (*models.LoadBalancer, error)
	Update(ctx context.Context, lb *models.LoadBalancer) error
	Delete(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) error

	AddListener(ctx context.Context, l *models.LoadBalancerListener) error
	UpdateListener(ctx context.Context, l *models.LoadBalancerListener) error
	DeleteListener(ctx context.Context, tenantID uuid.UUID, lbID uuid.UUID, listenerID uuid.UUID) error

	AddPool(ctx context.Context, p *models.LoadBalancerPool) error
	GetPoolByID(ctx context.Context, tenantID uuid.UUID, poolID uuid.UUID) (*models.LoadBalancerPool, error)
	ListPoolsByLB(ctx context.Context, tenantID uuid.UUID, lbID uuid.UUID) ([]models.LoadBalancerPool, error)
	UpdatePool(ctx context.Context, p *models.LoadBalancerPool) error
	DeletePool(ctx context.Context, tenantID uuid.UUID, lbID uuid.UUID, poolID uuid.UUID) error

	AddMember(ctx context.Context, m *models.LoadBalancerMember) error
	UpdateMember(ctx context.Context, m *models.LoadBalancerMember) error
	DeleteMember(ctx context.Context, tenantID uuid.UUID, poolID uuid.UUID, memberID uuid.UUID) error

	AddCertificate(ctx context.Context, c *models.LoadBalancerCertificate) error
	GetCertificateByID(ctx context.Context, tenantID uuid.UUID, certID uuid.UUID) (*models.LoadBalancerCertificate, error)
	ListCertificatesByTenant(ctx context.Context, tenantID uuid.UUID) ([]models.LoadBalancerCertificate, error)
	DeleteCertificate(ctx context.Context, tenantID uuid.UUID, certID uuid.UUID) error

	SaveRevision(ctx context.Context, rev *models.LoadBalancerConfigRevision) error
	ListRevisions(ctx context.Context, tenantID uuid.UUID, lbID uuid.UUID) ([]models.LoadBalancerConfigRevision, error)
	GetActiveRevision(ctx context.Context, tenantID uuid.UUID, lbID uuid.UUID) (*models.LoadBalancerConfigRevision, error)
}

type loadBalancerRepository struct{ base }

var _ LoadBalancerRepository = (*loadBalancerRepository)(nil)

func NewLoadBalancerRepository(db *gorm.DB) LoadBalancerRepository {
	return &loadBalancerRepository{base{db}}
}

func (r *loadBalancerRepository) Create(ctx context.Context, lb *models.LoadBalancer) error {
	return r.conn(ctx).Create(lb).Error
}

func (r *loadBalancerRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]models.LoadBalancer, error) {
	var lbs []models.LoadBalancer
	err := r.conn(ctx).
		Where("tenant_id = ?", tenantID).
		Preload("Listeners").
		Preload("Pools").
		Preload("Pools.Members").
		Preload("Pools.Members.Instance").
		Preload("Certificates").
		Order("created_at DESC").
		Find(&lbs).Error
	return lbs, err
}

func (r *loadBalancerRepository) GetByID(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) (*models.LoadBalancer, error) {
	var lb models.LoadBalancer
	err := r.conn(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Preload("Listeners").
		Preload("Pools").
		Preload("Pools.Members").
		Preload("Pools.Members.Instance").
		Preload("Certificates").
		Preload("ConfigRevisions", func(db *gorm.DB) *gorm.DB {
			return db.Order("version DESC")
		}).
		First(&lb).Error
	if err != nil {
		return nil, mapErr(err)
	}
	return &lb, nil
}

func (r *loadBalancerRepository) Update(ctx context.Context, lb *models.LoadBalancer) error {
	return r.conn(ctx).Save(lb).Error
}

func (r *loadBalancerRepository) Delete(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) error {
	return deleteChecked(r.conn(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&models.LoadBalancer{}))
}

func (r *loadBalancerRepository) AddListener(ctx context.Context, l *models.LoadBalancerListener) error {
	return r.conn(ctx).Create(l).Error
}

func (r *loadBalancerRepository) UpdateListener(ctx context.Context, l *models.LoadBalancerListener) error {
	return r.conn(ctx).Save(l).Error
}

func (r *loadBalancerRepository) DeleteListener(ctx context.Context, tenantID uuid.UUID, lbID uuid.UUID, listenerID uuid.UUID) error {
	return deleteChecked(r.conn(ctx).
		Where("id = ? AND load_balancer_id = ? AND tenant_id = ?", listenerID, lbID, tenantID).
		Delete(&models.LoadBalancerListener{}))
}

func (r *loadBalancerRepository) AddPool(ctx context.Context, p *models.LoadBalancerPool) error {
	return r.conn(ctx).Create(p).Error
}

func (r *loadBalancerRepository) GetPoolByID(ctx context.Context, tenantID uuid.UUID, poolID uuid.UUID) (*models.LoadBalancerPool, error) {
	var pool models.LoadBalancerPool
	err := r.conn(ctx).
		Where("id = ? AND tenant_id = ?", poolID, tenantID).
		Preload("Members").
		Preload("Members.Instance").
		First(&pool).Error
	if err != nil {
		return nil, mapErr(err)
	}
	return &pool, nil
}

func (r *loadBalancerRepository) ListPoolsByLB(ctx context.Context, tenantID uuid.UUID, lbID uuid.UUID) ([]models.LoadBalancerPool, error) {
	var pools []models.LoadBalancerPool
	err := r.conn(ctx).
		Where("load_balancer_id = ? AND tenant_id = ?", lbID, tenantID).
		Preload("Members").
		Preload("Members.Instance").
		Find(&pools).Error
	return pools, err
}

func (r *loadBalancerRepository) UpdatePool(ctx context.Context, p *models.LoadBalancerPool) error {
	return r.conn(ctx).Save(p).Error
}

func (r *loadBalancerRepository) DeletePool(ctx context.Context, tenantID uuid.UUID, lbID uuid.UUID, poolID uuid.UUID) error {
	return deleteChecked(r.conn(ctx).
		Where("id = ? AND load_balancer_id = ? AND tenant_id = ?", poolID, lbID, tenantID).
		Delete(&models.LoadBalancerPool{}))
}

func (r *loadBalancerRepository) AddMember(ctx context.Context, m *models.LoadBalancerMember) error {
	return r.conn(ctx).Create(m).Error
}

func (r *loadBalancerRepository) UpdateMember(ctx context.Context, m *models.LoadBalancerMember) error {
	return r.conn(ctx).Save(m).Error
}

func (r *loadBalancerRepository) DeleteMember(ctx context.Context, tenantID uuid.UUID, poolID uuid.UUID, memberID uuid.UUID) error {
	return deleteChecked(r.conn(ctx).
		Where("id = ? AND pool_id = ? AND tenant_id = ?", memberID, poolID, tenantID).
		Delete(&models.LoadBalancerMember{}))
}

func (r *loadBalancerRepository) AddCertificate(ctx context.Context, c *models.LoadBalancerCertificate) error {
	return r.conn(ctx).Create(c).Error
}

func (r *loadBalancerRepository) GetCertificateByID(ctx context.Context, tenantID uuid.UUID, certID uuid.UUID) (*models.LoadBalancerCertificate, error) {
	var cert models.LoadBalancerCertificate
	if err := r.conn(ctx).Where("id = ? AND tenant_id = ?", certID, tenantID).First(&cert).Error; err != nil {
		return nil, mapErr(err)
	}
	return &cert, nil
}

func (r *loadBalancerRepository) ListCertificatesByTenant(ctx context.Context, tenantID uuid.UUID) ([]models.LoadBalancerCertificate, error) {
	var certs []models.LoadBalancerCertificate
	err := r.conn(ctx).
		Where("tenant_id = ?", tenantID).
		Order("created_at DESC").
		Find(&certs).Error
	return certs, err
}

func (r *loadBalancerRepository) DeleteCertificate(ctx context.Context, tenantID uuid.UUID, certID uuid.UUID) error {
	return deleteChecked(r.conn(ctx).
		Where("id = ? AND tenant_id = ?", certID, tenantID).
		Delete(&models.LoadBalancerCertificate{}))
}

func (r *loadBalancerRepository) SaveRevision(ctx context.Context, rev *models.LoadBalancerConfigRevision) error {
	return r.conn(ctx).Create(rev).Error
}

func (r *loadBalancerRepository) ListRevisions(ctx context.Context, tenantID uuid.UUID, lbID uuid.UUID) ([]models.LoadBalancerConfigRevision, error) {
	var revs []models.LoadBalancerConfigRevision
	err := r.conn(ctx).
		Where("load_balancer_id = ? AND tenant_id = ?", lbID, tenantID).
		Order("version DESC").
		Find(&revs).Error
	return revs, err
}

func (r *loadBalancerRepository) GetActiveRevision(ctx context.Context, tenantID uuid.UUID, lbID uuid.UUID) (*models.LoadBalancerConfigRevision, error) {
	var rev models.LoadBalancerConfigRevision
	err := r.conn(ctx).
		Where("load_balancer_id = ? AND tenant_id = ? AND deployment_status = 'applied'", lbID, tenantID).
		Order("version DESC").
		First(&rev).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &rev, nil
}
