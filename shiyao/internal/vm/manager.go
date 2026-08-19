package vm

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Manager orchestrates multiple concurrent VM instances.
type Manager struct {
	mu        sync.RWMutex
	instances map[string]*Instance

	baseCfg  Config
	netCfg   NetworkConfig
	vsockCfg VsockConfig

	// Allocation counters for per-VM resources.
	nextSubnet uint16
	nextCID    uint32
}

// NewManager creates a VM manager from the supplied base configuration.
func NewManager(
	baseCfg Config,
	netCfg NetworkConfig,
	vsockCfg VsockConfig,
) *Manager {
	return &Manager{
		instances: make(map[string]*Instance),

		baseCfg:  baseCfg,
		netCfg:   netCfg,
		vsockCfg: vsockCfg,

		// 172.16.0.0/12 contains private IPv4 space.
		// We allocate one /24 per VM.
		nextSubnet: 0,

		// CID 0 and 1 are reserved, and the host uses CID 2.
		nextCID: 3,
	}
}

// CreateVM initializes a new VM instance in memory and allocates
// unique VM-specific networking and VSOCK resources.
func (m *Manager) CreateVM(vmID string) (*Instance, error) {
	if err := validateVMID(vmID); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.instances[vmID]; exists {
		return nil, fmt.Errorf("vm %s already exists", vmID)
	}

	netCfg, err := m.allocateNetworkConfig(vmID)
	if err != nil {
		return nil, err
	}

	vsockCfg := m.vsockCfg
	vsockCfg.GuestCID = m.allocateCID()

	socketPath := filepath.Join(
		os.TempDir(),
		fmt.Sprintf("firecracker-%s.sock", vmID),
	)

	inst := NewInstance(
		vmID,
		socketPath,
		m.baseCfg,
		netCfg,
		vsockCfg,
	)

	m.instances[vmID] = inst

	return inst, nil
}

// DestroyVM stops and removes a VM instance.
func (m *Manager) DestroyVM(ctx context.Context, vmID string) error {
	m.mu.Lock()

	inst, exists := m.instances[vmID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("vm %s not found", vmID)
	}

	// Do not hold the manager mutex while Stop() performs potentially
	// slow VMM/network cleanup.
	delete(m.instances, vmID)

	m.mu.Unlock()

	if err := inst.Stop(ctx); err != nil {
		// Put the instance back if cleanup failed so callers can retry.
		m.mu.Lock()
		m.instances[vmID] = inst
		m.mu.Unlock()

		return fmt.Errorf("stop vm %s: %w", vmID, err)
	}

	return nil
}

// GetVM retrieves a VM instance by its ID.
func (m *Manager) GetVM(vmID string) (*Instance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	inst, exists := m.instances[vmID]
	if !exists {
		return nil, fmt.Errorf("vm %s not found", vmID)
	}

	return inst, nil
}

// ListVMs returns all currently tracked VM IDs.
func (m *Manager) ListVMs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.instances))

	for id := range m.instances {
		ids = append(ids, id)
	}

	return ids
}

// allocateNetworkConfig allocates a unique /24 network for a VM.
//
// Example:
//
//	VM 0 -> 172.16.0.1 / 172.16.0.2
//	VM 1 -> 172.16.1.1 / 172.16.1.2
//	VM 2 -> 172.16.2.1 / 172.16.2.2
func (m *Manager) allocateNetworkConfig(vmID string) (NetworkConfig, error) {
	if m.nextSubnet >= 256 {
		return NetworkConfig{}, fmt.Errorf("network subnet pool exhausted")
	}

	cfg := m.netCfg

	thirdOctet := byte(m.nextSubnet)

	cfg.HostIP = fmt.Sprintf("172.16.%d.1", thirdOctet)
	cfg.GuestIP = fmt.Sprintf("172.16.%d.2/24", thirdOctet)
	cfg.TapName = uniqueTapName(vmID)

	if cfg.UplinkInterface == "" {
		return NetworkConfig{}, fmt.Errorf(
			"uplink interface is required",
		)
	}

	m.nextSubnet++

	return cfg, nil
}

// allocateCID returns a unique guest VSOCK CID.
func (m *Manager) allocateCID() uint32 {
	cid := m.nextCID
	m.nextCID++
	return cid
}

// uniqueTapName creates a short deterministic TAP name.
//
// Linux interface names are limited to 15 characters, so don't directly
// use an arbitrarily long VM ID.
func uniqueTapName(vmID string) string {
	sum := sha256.Sum256([]byte(vmID))

	return fmt.Sprintf(
		"shy%02x%02x%02x%02x",
		sum[0],
		sum[1],
		sum[2],
		sum[3],
	)
}

func validateVMID(vmID string) error {
	if vmID == "" {
		return fmt.Errorf("vm ID is required")
	}

	if len(vmID) > 64 {
		return fmt.Errorf("vm ID is too long")
	}

	if strings.ContainsAny(vmID, `/\:*?"<>|`) {
		return fmt.Errorf("vm ID %q contains invalid characters", vmID)
	}

	return nil
}
