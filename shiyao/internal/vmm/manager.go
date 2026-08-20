package vmm

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/coffeyvidzro/shiyao/internal/network"
	"github.com/coffeyvidzro/shiyao/internal/vsock"
)

type Manager struct {
	mu sync.Mutex
	instances map[string]*Instance
	baseCfg Config
	netCfg network.Config
	vsockCfg vsock.Config
	snapCfg SnapshotConfig
	ipam *network.IPAMPool
}

func NewManager(baseCfg Config, netCfg network.Config, vsockCfg vsock.Config, snapCfg SnapshotConfig) *Manager {
	return &Manager{instances: make(map[string]*Instance), baseCfg: baseCfg, netCfg: netCfg, vsockCfg: vsockCfg, snapCfg: snapCfg, ipam: network.NewIPAMPool()}
}

func (m *Manager) CreateVM(vmID string) (*Instance, error) {
	if err := validateVMID(vmID); err != nil { return nil, err }
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.instances[vmID]; exists { return nil, fmt.Errorf("vm %s already exists", vmID) }
	netCfg, cid, err := m.ipam.Allocate(vmID, m.netCfg)
	if err != nil { return nil, fmt.Errorf("allocate resources for vm %s: %w", vmID, err) }
	vsockCfg := m.vsockCfg
	vsockCfg.GuestCID = cid
	socketPath := filepath.Join(os.TempDir(), fmt.Sprintf("firecracker-%s.sock", vmID))
	inst := NewInstance(vmID, socketPath, m.baseCfg, netCfg, vsockCfg, m.snapCfg)
	m.instances[vmID] = inst
	return inst, nil
}

func (m *Manager) DestroyVM(ctx context.Context, vmID string) error {
	m.mu.Lock()
	inst, exists := m.instances[vmID]
	m.mu.Unlock()
	if !exists { return fmt.Errorf("vm %s not found", vmID) }
	if err := inst.Stop(ctx); err != nil { return fmt.Errorf("stop vm %s: %w", vmID, err) }
	m.mu.Lock()
	delete(m.instances, vmID)
	m.mu.Unlock()
	m.ipam.Release(inst.netCfg.GuestIP, inst.vsockCfg.GuestCID)
	return nil
}

func (m *Manager) GetVM(vmID string) (*Instance, error) {
	m.mu.Lock(); defer m.mu.Unlock()
	inst, exists := m.instances[vmID]
	if !exists { return nil, fmt.Errorf("vm %s not found", vmID) }
	return inst, nil
}

func (m *Manager) ListVMs() []string {
	m.mu.Lock(); defer m.mu.Unlock()
	ids := make([]string, 0, len(m.instances))
	for id := range m.instances { ids = append(ids, id) }
	return ids
}

func uniqueTapNameWithInstanceID(instanceID string) string {
	sum := sha256.Sum256([]byte(instanceID))
	return fmt.Sprintf("shy%02x%02x%02x%02x", sum[0], sum[1], sum[2], sum[3])
}

func validateVMID(vmID string) error {
	if vmID == "" { return fmt.Errorf("vm ID is required") }
	if len(vmID) > 64 { return fmt.Errorf("vm ID is too long") }
	if strings.ContainsAny(vmID, `/\\:*?"<>|`) { return fmt.Errorf("vm ID %q contains invalid characters", vmID) }
	return nil
}
