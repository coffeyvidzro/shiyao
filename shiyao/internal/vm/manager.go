package vm

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// State represents the lifecycle state of a VM instance.
type State uint8

const (
	StateCreated State = iota
	StateConfiguring
	StateConfigured
	StateRunning
	StateStopping
	StateStopped
	StateCleanupFailed
)

func (s State) String() string {
	switch s {
	case StateCreated:
		return "created"
	case StateConfiguring:
		return "configuring"
	case StateConfigured:
		return "configured"
	case StateRunning:
		return "running"
	case StateStopping:
		return "stopping"
	case StateStopped:
		return "stopped"
	case StateCleanupFailed:
		return "cleanup-failed"
	default:
		return "unknown"
	}
}

// Manager orchestrates multiple concurrent VM instances.
type Manager struct {
	mu        sync.Mutex
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
	defer m.mu.Unlock()

	if _, exists := m.instances[vmID]; exists {
		return nil, fmt.Errorf("vm %s already exists", vmID)
	}

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

	inst := NewInstance(
		vmID,
		socketPath,
		m.baseCfg,
		netCfg,
		vsockCfg,
		m.snapCfg,
	)

	m.instances[vmID] = inst

	return inst, nil
}

// DestroyVM stops a VM instance and recycles its network/vsock resources.
// The instance remains tracked while teardown is in progress so a failed
// cleanup cannot race with a new VM using the same ID or IP/CID resources.
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

// GetVM retrieves a VM instance by its ID.
func (m *Manager) GetVM(vmID string) (*Instance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	inst, exists := m.instances[vmID]
	if !exists {
		return nil, fmt.Errorf("vm %s not found", vmID)
	}

	return inst, nil
}

// ListVMs returns all currently tracked VM IDs.
func (m *Manager) ListVMs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	ids := make([]string, 0, len(m.instances))

	for id := range m.instances {
		ids = append(ids, id)
	}

	return ids
}

// generateInstanceID creates a cryptographically random instance identifier
// for host resource naming to avoid predictable namespace collisions.
func generateInstanceID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// uniqueTapName creates a short deterministic TAP name based on VM ID.
// Deprecated: Use uniqueTapNameWithInstanceID for new code to avoid predictable names.
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

// uniqueTapNameWithInstanceID creates a TAP name using a random instance ID
// to avoid predictable firewall chain names and resource collisions.
func uniqueTapNameWithInstanceID(instanceID string) string {
	sum := sha256.Sum256([]byte(instanceID))

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

	if strings.ContainsAny(vmID, `/\\:*?"<>|`) {
		return fmt.Errorf("vm ID %q contains invalid characters", vmID)
	}

	return nil
}
