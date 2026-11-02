package usecase

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/repository"
)

// PortUsecase allocates public game ports per hypervisor node. Allocation is
// serialized in-process by mu.
type PortUsecase struct {
	ports repository.PortRepository
	mu    sync.Mutex
}

func NewPortUsecase(ports repository.PortRepository) *PortUsecase {
	return &PortUsecase{
		ports: ports,
	}
}

// Default base ports per game per PRD Section 54.2
const (
	DefaultPortMinecraftJava    = 25565
	DefaultPortMinecraftBedrock = 19132
	DefaultPortTerraria         = 7777
)

// FindAvailablePort scans for the next available public port on the target node
func (s *PortUsecase) FindAvailablePort(ctx context.Context, gameType string, nodeName string) (int, string, error) {
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

	allocatedPorts, err := s.ports.ListAllocated(ctx, nodeName, protocol)
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
func (s *PortUsecase) RecordAllocation(ctx context.Context, serverID uuid.UUID, port int, protocol string, nodeName string) (*models.GameServerPort, error) {
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

	if err := s.ports.Create(ctx, &portRecord); err != nil {
		return nil, fmt.Errorf("failed to persist port allocation: %w", err)
	}

	return &portRecord, nil
}

// AllocatePort combines finding an available port and persisting it
func (s *PortUsecase) AllocatePort(ctx context.Context, serverID uuid.UUID, gameType string, nodeName string) (*models.GameServerPort, error) {
	chosenPort, protocol, err := s.FindAvailablePort(ctx, gameType, nodeName)
	if err != nil {
		return nil, err
	}
	return s.RecordAllocation(ctx, serverID, chosenPort, protocol, nodeName)
}

// ReleasePort releases a specific port number on a node
func (s *PortUsecase) ReleasePort(ctx context.Context, portNumber int, nodeName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.ports.ReleaseByNumber(ctx, portNumber, nodeName)
}

// ReleasePorts frees all ports assigned to a game server
func (s *PortUsecase) ReleasePorts(ctx context.Context, serverID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.ports.ReleaseByServer(ctx, serverID)
}
