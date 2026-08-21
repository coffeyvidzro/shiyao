package vmm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/coffeyvidzro/shiyao/internal/network"
	"github.com/coffeyvidzro/shiyao/internal/vsock"
)

var ErrBackpressure = errors.New("vmm admission limit reached")

type ManagerLimits struct {
	MaxVMs                 int
	MaxConcurrentProvision int
}

func DefaultManagerLimits() ManagerLimits {
	return ManagerLimits{MaxVMs: 256, MaxConcurrentProvision: 8}
}

type Manager struct {
	mu        sync.Mutex
	instances map[string]*Instance
	baseCfg   Config
	netCfg    network.Config
	vsockCfg  vsock.Config
	snapCfg   SnapshotConfig
	ipam      *network.IPAMPool
	limits    ManagerLimits
	provision chan struct{}
}

func NewManager(baseCfg Config, netCfg network.Config, vsockCfg vsock.Config, snapCfg SnapshotConfig) *Manager {
	return NewManagerWithLimits(baseCfg, netCfg, vsockCfg, snapCfg, DefaultManagerLimits())
}

func NewManagerWithLimits(baseCfg Config, netCfg network.Config, vsockCfg vsock.Config, snapCfg SnapshotConfig, limits ManagerLimits) *Manager {
	if limits.MaxVMs <= 0 {
		limits.MaxVMs = DefaultManagerLimits().MaxVMs
	}
	if limits.MaxConcurrentProvision <= 0 {
		limits.MaxConcurrentProvision = DefaultManagerLimits().MaxConcurrentProvision
	}
	return &Manager{
		instances: make(map[string]*Instance),
		baseCfg:   baseCfg,
		netCfg:    netCfg,
		vsockCfg:  vsockCfg,
		snapCfg:   snapCfg,
		ipam:      network.NewIPAMPool(),
		limits:    limits,
		provision: make(chan struct{}, limits.MaxConcurrentProvision),
	}
}

func (m *Manager) CreateVM(vmID string) (*Instance, error) {
	if err := validateVMID(vmID); err != nil {
		return nil, err
	}

	// 1. Fast path: Check limits and existence
	m.mu.Lock()
	if len(m.instances) >= m.limits.MaxVMs {
		m.mu.Unlock()
		return nil, fmt.Errorf("%w: maximum of %d resident VMs", ErrBackpressure, m.limits.MaxVMs)
	}
	if _, exists := m.instances[vmID]; exists {
		m.mu.Unlock()
		return nil, fmt.Errorf("vm %s already exists", vmID)
	}
	m.mu.Unlock()

	// 2. Allocate resources (Network/IPAM) WITHOUT holding the global lock.
	// This prevents IPAM latency from blocking all other VM creations/destructions.
	allocation, err := network.Acquire(vmID, m.netCfg, m.ipam)
	if err != nil {
		return nil, fmt.Errorf("allocate resources for vm %s: %w", vmID, err)
	}

	// 3. Slow path: Lock again to insert safely and prevent TOCTOU races.
	m.mu.Lock()

	// Re-check existence in case another goroutine created this vmID
	// while we were waiting on network.Acquire()
	if _, exists := m.instances[vmID]; exists {
		m.mu.Unlock()

		// CRITICAL: Cleanup the leaked resource before returning
		_ = allocation.Release(context.Background())

		return nil, fmt.Errorf("vm %s already exists", vmID)
	}

	// 4. Finalize initialization (Safe to do under lock, it's just memory assignment)
	vsockCfg := m.vsockCfg
	vsockCfg.GuestCID = allocation.CID()
	socketPath := filepath.Join(os.TempDir(), fmt.Sprintf("firecracker-%s.sock", vmID))

	inst := NewInstance(vmID, socketPath, m.baseCfg, allocation, vsockCfg, m.snapCfg)
	m.instances[vmID] = inst
	m.mu.Unlock()

	return inst, nil
}

func (m *Manager) ProvisionVM(ctx context.Context, vmID string) (*Instance, error) {
	select {
	case m.provision <- struct{}{}:
		defer func() { <-m.provision }()
	default:
		return nil, fmt.Errorf("%w: %d provisioning operations already running", ErrBackpressure, m.limits.MaxConcurrentProvision)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	inst, err := m.CreateVM(vmID)
	if err != nil {
		return nil, err
	}
	if err := inst.Configure(ctx); err != nil {
		if cleanupErr := m.removeFailedVM(ctx, vmID, inst); cleanupErr != nil {
			return nil, errors.Join(fmt.Errorf("configure vm %s: %w", vmID, err), fmt.Errorf("cleanup vm %s: %w", vmID, cleanupErr))
		}
		return nil, fmt.Errorf("configure vm %s: %w", vmID, err)
	}
	if err := inst.Start(ctx); err != nil {
		cleanupErr := m.removeFailedVM(ctx, vmID, inst)
		if cleanupErr != nil {
			return nil, errors.Join(fmt.Errorf("start vm %s: %w", vmID, err), fmt.Errorf("cleanup vm %s: %w", vmID, cleanupErr))
		}
		return nil, fmt.Errorf("start vm %s: %w", vmID, err)
	}
	return inst, nil
}

func (m *Manager) removeFailedVM(ctx context.Context, vmID string, inst *Instance) error {
	if err := inst.Stop(ctx); err != nil {
		return err
	}
	m.mu.Lock()
	if m.instances[vmID] == inst {
		delete(m.instances, vmID)
	}
	m.mu.Unlock()
	return nil
}

func (m *Manager) DestroyVM(ctx context.Context, vmID string) error {
	m.mu.Lock()
	inst, exists := m.instances[vmID]
	m.mu.Unlock()
	if !exists {
		return fmt.Errorf("vm %s not found", vmID)
	}
	if err := inst.Stop(ctx); err != nil {
		return fmt.Errorf("stop vm %s: %w", vmID, err)
	}
	m.mu.Lock()
	if m.instances[vmID] == inst {
		delete(m.instances, vmID)
	}
	m.mu.Unlock()
	return nil
}

func (m *Manager) GetVM(vmID string) (*Instance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	inst, exists := m.instances[vmID]
	if !exists {
		return nil, fmt.Errorf("vm %s not found", vmID)
	}
	return inst, nil
}

func (m *Manager) ListVMs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := make([]string, 0, len(m.instances))
	for id := range m.instances {
		ids = append(ids, id)
	}
	return ids
}
