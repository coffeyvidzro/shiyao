package vm

import (
	"context"
	"fmt"
	"sync"
)

// Manager orchestrates multiple concurrent VM instances.
// It is safe for concurrent use by multiple goroutines.
type Manager struct {
	mu        sync.RWMutex
	instances map[string]*Instance // Keyed by VM ID
	baseCfg   Config
	netCfg    NetworkConfig
}

// NewManager initializes a new VM Manager with default base configurations.
func NewManager(baseCfg Config, netCfg NetworkConfig) *Manager {
	return &Manager{
		instances: make(map[string]*Instance),
		baseCfg:   baseCfg,
		netCfg:    netCfg,
	}
}

// CreateVM initializes a new VM instance in memory but does not start it yet.
// The caller must subsequently call instance.Configure() and instance.Start().
func (m *Manager) CreateVM(vmID string) (*Instance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.instances[vmID]; exists {
		return nil, fmt.Errorf("vm %s already exists", vmID)
	}

	// Generate a unique socket path for this specific VM
	socketPath := fmt.Sprintf("/tmp/firecracker-%s.sock", vmID)

	// Create the instance (passing the vmID for deterministic MAC generation)
	inst := NewInstance(vmID, socketPath, m.baseCfg, m.netCfg)
	m.instances[vmID] = inst

	return inst, nil
}

// DestroyVM stops and completely removes a VM instance, cleaning up all resources.
func (m *Manager) DestroyVM(ctx context.Context, vmID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	inst, exists := m.instances[vmID]
	if !exists {
		return fmt.Errorf("vm %s not found", vmID)
	}

	// Stop the VM and clean up TAP/iptables
	if err := inst.Stop(ctx); err != nil {
		return fmt.Errorf("stop vm %s: %w", vmID, err)
	}

	delete(m.instances, vmID)
	return nil
}

// GetVM retrieves a running or pending VM instance by its ID.
func (m *Manager) GetVM(vmID string) (*Instance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	inst, exists := m.instances[vmID]
	if !exists {
		return nil, fmt.Errorf("vm %s not found", vmID)
	}

	return inst, nil
}

// ListVMs returns a list of all currently tracked VM IDs.
func (m *Manager) ListVMs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.instances))
	for id := range m.instances {
		ids = append(ids, id)
	}

	return ids
}

// ActiveCount returns the number of VMs currently managed.
func (m *Manager) ActiveCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.instances)
}
