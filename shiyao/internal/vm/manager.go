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
	snapCfg  SnapshotConfig

	ipam *IPAMPool
}

// NewManager creates a VM manager with an active IPAM pool.
func NewManager(
	baseCfg Config,
	netCfg NetworkConfig,
	vsockCfg VsockConfig,
	snapCfg SnapshotConfig,
) *Manager {
	return &Manager{
		instances: make(map[string]*Instance),
		baseCfg:   baseCfg,
		netCfg:    netCfg,
		vsockCfg:  vsockCfg,
		snapCfg:   snapCfg,
		ipam:      NewIPAMPool(),
	}
}

// CreateVM initializes a new VM instance with safely allocated resources.
func (m *Manager) CreateVM(vmID string) (*Instance, error) {
	if err := validateVMID(vmID); err != nil {
		return nil, err
	}

	m.mu.Lock()
	if _, exists := m.instances[vmID]; exists {
		m.mu.Unlock()
		return nil, fmt.Errorf("vm %s already exists", vmID)
	}
	m.mu.Unlock()

	// Atomically allocate subnet and CID from pool
	netCfg, cid, err := m.ipam.Allocate(vmID, m.netCfg)
	if err != nil {
		return nil, fmt.Errorf("allocate resources for vm %s: %w", vmID, err)
	}

	vsockCfg := m.vsockCfg
	vsockCfg.GuestCID = cid

	socketPath := filepath.Join(
		os.TempDir(),
		fmt.Sprintf("firecracker-%s.sock", vmID),
	)

	// Fixed: Passing all 6 arguments required by NewInstance
	inst := NewInstance(
		vmID,
		socketPath,
		m.baseCfg,
		netCfg,
		vsockCfg,
		m.snapCfg,
	)

	m.mu.Lock()
	m.instances[vmID] = inst
	m.mu.Unlock()

	return inst, nil
}

// DestroyVM stops a VM instance and recycles its network/vsock resources.
func (m *Manager) DestroyVM(ctx context.Context, vmID string) error {
	m.mu.Lock()
	inst, exists := m.instances[vmID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("vm %s not found", vmID)
	}

	// Remove tracking to prevent concurrent actions during stop
	delete(m.instances, vmID)
	m.mu.Unlock()

	// Perform physical stop and teardown outside manager lock
	err := inst.Stop(ctx)

	// Always release IPAM resources back to the pool regardless of cleanup warning
	m.ipam.Release(inst.netCfg.GuestIP, inst.vsockCfg.GuestCID)

	if err != nil {
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

// uniqueTapName creates a short deterministic TAP name.
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
