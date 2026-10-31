package services

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"gorm.io/gorm"
)

type PortService struct {
	db *gorm.DB
	mu sync.Mutex
}

func NewPortService(db *gorm.DB) *PortService {
	return &PortService{
		db: db,
	}
}

// Default base ports per game per PRD Section 54.2
const (
	DefaultPortMinecraftJava    = 25565
	DefaultPortMinecraftBedrock = 19132
	DefaultPortTerraria         = 7777
)

// FindAvailablePort scans for the next available public port on the target node
func (s *PortService) FindAvailablePort(gameType string, nodeName string) (int, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var defaultPort int
	var protocol string

	switch gameType {
	case models.GameTypeMinecraftJava:
		defaultPort = DefaultPortMinecraftJava
		protocol = "tcp"
	case models.GameTypeMinecraftBedrock:
		defaultPort = DefaultPortMinecraftBedrock
		protocol = "udp"
	case models.GameTypeTerraria:
		defaultPort = DefaultPortTerraria
		protocol = "tcp"
	default:
		return 0, "", fmt.Errorf("unsupported game type for port allocation: %s", gameType)
	}

	var allocatedPorts []int
	err := s.db.Model(&models.GameServerPort{}).
		Where("node_name = ? AND protocol = ? AND is_allocated = ?", nodeName, protocol, true).
		Pluck("port_number", &allocatedPorts).Error
	if err != nil {
		return 0, "", fmt.Errorf("failed to query allocated ports: %w", err)
	}

	allocatedMap := make(map[int]bool)
	for _, p := range allocatedPorts {
		allocatedMap[p] = true
	}

	chosenPort := -1
	for p := defaultPort; p < defaultPort+1000; p++ {
		if !allocatedMap[p] {
			chosenPort = p
			break
		}
	}

	if chosenPort == -1 {
		return 0, "", errors.New("no available ports in range on hypervisor node")
	}

	return chosenPort, protocol, nil
}

// RecordAllocation persists the allocated port once the game server record exists
func (s *PortService) RecordAllocation(serverID uuid.UUID, port int, protocol string, nodeName string) (*models.GameServerPort, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	portRecord := models.GameServerPort{
		ServerID:    serverID,
		PortNumber:  port,
		Protocol:    protocol,
		ServiceType: "game",
		NodeName:    nodeName,
		IsAllocated: true,
		AllocatedAt: time.Now(),
	}

	if err := s.db.Create(&portRecord).Error; err != nil {
		return nil, fmt.Errorf("failed to persist port allocation: %w", err)
	}

	return &portRecord, nil
}

// AllocatePort combines finding an available port and persisting it
func (s *PortService) AllocatePort(serverID uuid.UUID, gameType string, nodeName string) (*models.GameServerPort, error) {
	chosenPort, protocol, err := s.FindAvailablePort(gameType, nodeName)
	if err != nil {
		return nil, err
	}
	return s.RecordAllocation(serverID, chosenPort, protocol, nodeName)
}

// ReleasePort releases a specific port number on a node
func (s *PortService) ReleasePort(portNumber int, nodeName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	return s.db.Model(&models.GameServerPort{}).
		Where("port_number = ? AND node_name = ? AND is_allocated = ?", portNumber, nodeName, true).
		Updates(map[string]interface{}{
			"is_allocated": false,
			"released_at":  &now,
		}).Error
}

// ReleasePorts frees all ports assigned to a game server
func (s *PortService) ReleasePorts(serverID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	return s.db.Model(&models.GameServerPort{}).
		Where("server_id = ? AND is_allocated = ?", serverID, true).
		Updates(map[string]interface{}{
			"is_allocated": false,
			"released_at":  &now,
		}).Error
}
