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

// ManagerLimits bounds resident VMs and expensive host-side provisioning work.
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
	defer m.mu.Unlock()
	if len(m.instances) >= m.limits.MaxVMs {
		return nil, fmt.Errorf("%w: maximum of %d resident VMs", ErrBackpressure, m.limits.MaxVMs)
	}
	if _, exists := m.instances[vmID]; exists {
		return nil, fmt.Errorf("vm %s already exists", vmID)
	}
	netCfg, cid, err := m.ipam.Allocate(vmID, m.netCfg)
	if err != nil {
		return nil, fmt.Errorf("allocate resources for vm %s: %w", vmID, err)
	}
	vsockCfg := m.vsockCfg
	vsockCfg.GuestCID = cid
	socketPath := filepath.Join(os.TempDir(), fmt.Sprintf("firecracker-%s.sock", vmID))
	inst := NewInstance(vmID, socketPath, m.baseCfg, netCfg, vsockCfg, m.snapCfg)
	m.instances[vmID] = inst
	return inst, nil
}

// ProvisionVM performs configuration and boot under a bounded admission gate.
// It fails fast when the host is saturated rather than accumulating unbounded
// queued TAP, nftables, Firecracker, and snapshot operations.
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
		m.removeFailedVM(vmID, inst)
		return nil, fmt.Errorf("configure vm %s: %w", vmID, err)
	}
	if err := inst.Start(ctx); err != nil {
		_ = inst.Stop(ctx)
		m.removeFailedVM(vmID, inst)
		return nil, fmt.Errorf("start vm %s: %w", vmID, err)
	}
	return inst, nil
}

func (m *Manager) removeFailedVM(vmID string, inst *Instance) {
	m.mu.Lock()
	if m.instances[vmID] == inst {
		delete(m.instances, vmID)
	}
	m.mu.Unlock()
	m.ipam.Release(inst.netCfg.GuestIP, inst.vsockCfg.GuestCID)
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
	delete(m.instances, vmID)
	m.mu.Unlock()
	m.ipam.Release(inst.netCfg.GuestIP, inst.vsockCfg.GuestCID)
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
