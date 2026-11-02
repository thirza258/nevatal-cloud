package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"gorm.io/gorm"
)

// PortRepository persists public port allocations of game servers
// (game_server_ports).
type PortRepository interface {
	// ListAllocated returns the allocated port numbers of a node and protocol.
	ListAllocated(ctx context.Context, nodeName, protocol string) ([]int, error)
	Create(ctx context.Context, port *models.GameServerPort) error
	// ReleaseByNumber frees an allocated port number on a node.
	ReleaseByNumber(ctx context.Context, portNumber int, nodeName string) error
	// ReleaseByServer frees every port allocated to a game server.
	ReleaseByServer(ctx context.Context, serverID uuid.UUID) error
}

type portRepository struct{ base }

var _ PortRepository = (*portRepository)(nil)

func NewPortRepository(db *gorm.DB) PortRepository {
	return &portRepository{base{db}}
}

func (r *portRepository) ListAllocated(ctx context.Context, nodeName, protocol string) ([]int, error) {
	var allocatedPorts []int
	err := r.conn(ctx).Model(&models.GameServerPort{}).
		Where("node_name = ? AND protocol = ? AND is_allocated = ?", nodeName, protocol, true).
		Pluck("port_number", &allocatedPorts).Error
	return allocatedPorts, err
}

func (r *portRepository) Create(ctx context.Context, port *models.GameServerPort) error {
	return r.conn(ctx).Create(port).Error
}

func (r *portRepository) ReleaseByNumber(ctx context.Context, portNumber int, nodeName string) error {
	now := time.Now()
	return r.conn(ctx).Model(&models.GameServerPort{}).
		Where("port_number = ? AND node_name = ? AND is_allocated = ?", portNumber, nodeName, true).
		Updates(map[string]interface{}{
			"is_allocated": false,
			"released_at":  &now,
		}).Error
}

func (r *portRepository) ReleaseByServer(ctx context.Context, serverID uuid.UUID) error {
	now := time.Now()
	return r.conn(ctx).Model(&models.GameServerPort{}).
		Where("server_id = ? AND is_allocated = ?", serverID, true).
		Updates(map[string]interface{}{
			"is_allocated": false,
			"released_at":  &now,
		}).Error
}
