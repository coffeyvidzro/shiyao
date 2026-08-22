package vmm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/coffeyvidzro/shiyao/internal/core/network"
	"github.com/coffeyvidzro/shiyao/internal/core/vsock"
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

	allocation, err := network.Acquire(vmID, m.netCfg, m.ipam)
	if err != nil {
		return nil, fmt.Errorf("allocate resources for vm %s: %w", vmID, err)
	}

	m.mu.Lock()
	if _, exists := m.instances[vmID]; exists {
		m.mu.Unlock()
		_ = allocation.Release(context.Background())
		return nil, fmt.Errorf("vm %s already exists", vmID)
	}
	if len(m.instances) >= m.limits.MaxVMs {
		m.mu.Unlock()
		_ = allocation.Release(context.Background())
		return nil, fmt.Errorf("%w: maximum of %d resident VMs", ErrBackpressure, m.limits.MaxVMs)
	}

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
		if cleanupErr := m.removeFailedVM(ctx, vmID, inst); cleanupErr != nil {
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

// Reconcile retries cleanup for instances that could not be fully torn down.
// Cleanup failures remain visible in the manager until a later reconciliation succeeds.
func (m *Manager) Reconcile(ctx context.Context) error {
	m.mu.Lock()
	instances := make([]struct {
		id   string
		inst *Instance
	}, 0)
	for id, inst := range m.instances {
		if inst.State() == StateCleanupFailed {
			instances = append(instances, struct {
				id   string
				inst *Instance
			}{id: id, inst: inst})
		}
	}
	m.mu.Unlock()

	var errs []error
	for _, item := range instances {
		if err := item.inst.Stop(ctx); err != nil {
			errs = append(errs, fmt.Errorf("reconcile vm %s: %w", item.id, err))
			continue
		}
		m.mu.Lock()
		if m.instances[item.id] == item.inst {
			delete(m.instances, item.id)
		}
		m.mu.Unlock()
	}
	return errors.Join(errs...)
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
