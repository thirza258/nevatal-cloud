package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"gorm.io/gorm"
)

// NodeRepository persists Proxmox hypervisor nodes (nodes).
type NodeRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (*models.Node, error)
	// First returns the node with the lowest id (the "primary" node). On error
	// it still returns a zero Node, which callers render as is.
	First(ctx context.Context) (*models.Node, error)
	// FirstOnline returns the oldest node whose status is online.
	FirstOnline(ctx context.Context) (*models.Node, error)
	// FirstOnlineByID returns the online node with the lowest id.
	FirstOnlineByID(ctx context.Context) (*models.Node, error)
	List(ctx context.Context) ([]models.Node, error)
}

type nodeRepository struct{ base }

var _ NodeRepository = (*nodeRepository)(nil)

func NewNodeRepository(db *gorm.DB) NodeRepository {
	return &nodeRepository{base{db}}
}

func (r *nodeRepository) FindByID(ctx context.Context, id uuid.UUID) (*models.Node, error) {
	var node models.Node
	if err := r.conn(ctx).First(&node, "id = ?", id).Error; err != nil {
		return nil, mapErr(err)
	}
	return &node, nil
}

func (r *nodeRepository) First(ctx context.Context) (*models.Node, error) {
	var node models.Node
	if err := r.conn(ctx).First(&node).Error; err != nil {
		return &node, mapErr(err)
	}
	return &node, nil
}

func (r *nodeRepository) FirstOnline(ctx context.Context) (*models.Node, error) {
	var node models.Node
	if err := r.conn(ctx).Where("status = ?", "online").Order("created_at asc").First(&node).Error; err != nil {
		return nil, mapErr(err)
	}
	return &node, nil
}

func (r *nodeRepository) List(ctx context.Context) ([]models.Node, error) {
	var nodes []models.Node
	err := r.conn(ctx).Find(&nodes).Error
	return nodes, err
}

func (r *nodeRepository) FirstOnlineByID(ctx context.Context) (*models.Node, error) {
	var node models.Node
	if err := r.conn(ctx).First(&node, "status = ?", "online").Error; err != nil {
		return nil, mapErr(err)
	}
	return &node, nil
}
