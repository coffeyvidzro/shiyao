package vm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"

	fc "github.com/firecracker-microvm/firecracker-go-sdk"
	models "github.com/firecracker-microvm/firecracker-go-sdk/client/models"
)

// SnapshotConfig specifies paths required for resuming a pre-warmed microVM.
type SnapshotConfig struct {
	MemFilePath   string
	StateFilePath string
	EnableResume  bool
}

// SnapshotIntegrity holds validation metadata for snapshot files.
type SnapshotIntegrity struct {
	ExpectedMemHash   string
	ExpectedStateHash string
	KernelHash        string
	RootfsHash        string
}

type Instance struct {
	mu         sync.Mutex
	ID         string
	InstanceID string // Random instance ID for host resource naming
	SocketPath string
	cfg        Config
	netCfg     NetworkConfig
	vsockCfg   VsockConfig
	snapCfg    SnapshotConfig
	snapInteg  SnapshotIntegrity // Snapshot integrity validation metadata
	machine    *fc.Machine
	cleanups   []func() error
	state      State
}

func NewInstance(
	id, socketPath string,
	cfg Config,
	netCfg NetworkConfig,
	vsockCfg VsockConfig,
	snapCfg SnapshotConfig,
) *Instance {
	return &Instance{
		ID:         id,
		InstanceID: "", // Will be set during Configure if needed
		SocketPath: socketPath,
		cfg:        cfg,
		netCfg:     netCfg,
		vsockCfg:   vsockCfg,
		snapCfg:    snapCfg,
		state:      StateCreated,
	}
}

func generateMAC(vmID string) string {
	hash := sha256.Sum256([]byte(vmID))
	mac := net.HardwareAddr{0x02, hash[0] & 0xFE, hash[1], hash[2], hash[3], hash[4]}
	return mac.String()
}

// validateSnapshotIntegrity checks snapshot file hashes if expected values are provided.
// This prevents loading tampered or corrupted snapshot files.
func (i *Instance) validateSnapshotIntegrity() error {
	// If no hashes are provided, skip validation (allow unverified snapshots).
	if i.snapInteg.ExpectedMemHash == "" && i.snapInteg.ExpectedStateHash == "" {
		return nil
	}

	if i.snapInteg.ExpectedMemHash != "" {
		memHash, err := computeFileHash(i.snapCfg.MemFilePath)
		if err != nil {
			return fmt.Errorf("compute memory file hash: %w", err)
		}
		if memHash != i.snapInteg.ExpectedMemHash {
			return fmt.Errorf("memory file hash mismatch: expected %s, got %s", i.snapInteg.ExpectedMemHash, memHash)
		}
	}

	if i.snapInteg.ExpectedStateHash != "" {
		stateHash, err := computeFileHash(i.snapCfg.StateFilePath)
		if err != nil {
			return fmt.Errorf("compute state file hash: %w", err)
		}
		if stateHash != i.snapInteg.ExpectedStateHash {
			return fmt.Errorf("state file hash mismatch: expected %s, got %s", i.snapInteg.ExpectedStateHash, stateHash)
		}
	}

	return nil
}

// computeFileHash computes SHA256 hash of a file.
func computeFileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func guestKernelArgs(bootArgs, guestAgentPath string) string {
	bootArgs = strings.TrimSpace(bootArgs)
	if guestAgentPath == "" {
		return bootArgs
	}

	args := strings.Fields(bootArgs)
	for _, arg := range args {
		if strings.HasPrefix(arg, "init=") {
			return bootArgs
		}
	}

	return strings.TrimSpace(bootArgs + " init=" + guestAgentPath)
}

func (i *Instance) Configure(ctx context.Context) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.state != StateCreated {
		return fmt.Errorf("instance %s cannot be configured in state %s", i.ID, i.state)
	}
	i.state = StateConfiguring

	var err error
	i.InstanceID, err = generateInstanceID()
	if err != nil {
		i.state = StateCreated
		return fmt.Errorf("generate instance ID: %w", err)
	}

	i.netCfg.TapName = uniqueTapNameWithInstanceID(i.InstanceID)

	if err := i.cfg.Validate(); err != nil {
		i.state = StateCreated
		return fmt.Errorf("invalid config: %w", err)
	}

	configured := false
	defer func() {
		if !configured {
			for j := len(i.cleanups) - 1; j >= 0; j-- {
				if cleanupErr := i.cleanups[j](); cleanupErr != nil {
					fmt.Printf("warning: rollback cleanup failed for vm %s: %v\n", i.ID, cleanupErr)
				}
			}
			i.cleanups = nil
		}
	}()

	if err := SetupTAP(ctx, i.netCfg); err != nil {
		i.state = StateCreated
		return fmt.Errorf("setup tap: %w", err)
	}
	i.cleanups = append(i.cleanups, func() error { return CleanupTAP(ctx, i.netCfg.TapName) })

	if err := SetupFirewall(ctx, i.netCfg); err != nil {
		i.state = StateCreated
		return fmt.Errorf("setup firewall: %w", err)
	}
	i.cleanups = append(i.cleanups, func() error { return CleanupFirewall(ctx, i.netCfg) })

	macAddr := generateMAC(i.ID)
	guestIP, guestNetwork, err := net.ParseCIDR(i.netCfg.GuestIP)
	if err != nil {
		i.state = StateCreated
		return fmt.Errorf("parse guest IP %q: %w", i.netCfg.GuestIP, err)
	}
	guestNetwork.IP = guestIP
	gatewayIP := net.ParseIP(i.netCfg.HostIP)
	if gatewayIP == nil {
		i.state = StateCreated
		return fmt.Errorf("parse host/gateway IP %q: invalid IP", i.netCfg.HostIP)
	}

	fcConfig := fc.Config{
		SocketPath: i.SocketPath,
		Drives: []models.Drive{{
			DriveID:      fc.String("rootfs"),
			PathOnHost:   fc.String(i.cfg.RootfsPath),
			IsRootDevice: fc.Bool(true),
			IsReadOnly:   fc.Bool(false),
		}},
		MachineCfg: models.MachineConfiguration{
			VcpuCount:  fc.Int64(int64(i.cfg.VCPUCount)),
			MemSizeMib: fc.Int64(int64(i.cfg.MemSizeMB)),
		},
		NetworkInterfaces: []fc.NetworkInterface{{
			StaticConfiguration: &fc.StaticNetworkConfiguration{
				HostDevName: i.netCfg.TapName,
				MacAddress:  macAddr,
				IPConfiguration: &fc.IPConfiguration{
					IPAddr:  *guestNetwork,
					Gateway: gatewayIP,
				},
			},
		}},
		VsockDevices: []fc.VsockDevice{{ID: "vsock0", CID: i.vsockCfg.GuestCID}},
	}

	var opts []fc.Opt
	if i.snapCfg.EnableResume {
		if i.snapCfg.MemFilePath == "" || i.snapCfg.StateFilePath == "" {
			i.state = StateCreated
			return fmt.Errorf("snapshot restore enabled but snapshot paths are missing")
		}

		if err := i.validateSnapshotIntegrity(); err != nil {
			i.state = StateCreated
			return fmt.Errorf("validate snapshot integrity: %w", err)
		}

		opts = append(opts, fc.WithSnapshot(i.snapCfg.MemFilePath, i.snapCfg.StateFilePath))
	} else {
		fcConfig.KernelImagePath = i.cfg.KernelPath
		fcConfig.KernelArgs = guestKernelArgs(i.cfg.BootArgs, i.cfg.GuestAgentPath)
	}

	machine, err := fc.NewMachine(ctx, fcConfig, opts...)
	if err != nil {
		i.state = StateCreated
		return fmt.Errorf("create firecracker machine: %w", err)
	}
	i.machine = machine
	configured = true
	i.state = StateConfigured
	return nil
}

func (i *Instance) Start(ctx context.Context) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.state != StateConfigured {
		return fmt.Errorf("instance %s cannot be started in state %s", i.ID, i.state)
	}

	if i.machine == nil {
		return fmt.Errorf("machine not configured")
	}

	if i.snapCfg.EnableResume {
		if err := i.machine.ResumeVM(ctx); err != nil {
			return fmt.Errorf("resume microvm snapshot: %w", err)
		}
		i.state = StateRunning
		return nil
	}

	if err := i.machine.Start(ctx); err != nil {
		return fmt.Errorf("start guest os: %w", err)
	}
	i.state = StateRunning
	return nil
}

func (i *Instance) Stop(ctx context.Context) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.state != StateRunning && i.state != StateConfigured && i.state != StateCleanupFailed {
		return nil
	}
	i.state = StateStopping

	var errs []error

	if i.machine != nil {
		if err := i.machine.StopVMM(); err != nil {
			errs = append(errs, fmt.Errorf("stop vmm: %w", err))
		} else {
			i.machine = nil
		}
	}

	// Keep failed cleanup functions so a subsequent Stop call can retry them.
	remainingCleanups := make([]func() error, 0, len(i.cleanups))
	for j := len(i.cleanups) - 1; j >= 0; j-- {
		if err := i.cleanups[j](); err != nil {
			errs = append(errs, fmt.Errorf("cleanup step %d: %w", j, err))
			remainingCleanups = append(remainingCleanups, i.cleanups[j])
		}
	}
	i.cleanups = remainingCleanups

	if len(i.cleanups) == 0 {
		if err := os.Remove(i.SocketPath); err != nil && !os.IsNotExist(err) {
			errs = append(errs, fmt.Errorf("remove socket: %w", err))
		}
	}

	if len(errs) > 0 || len(i.cleanups) > 0 || i.machine != nil {
		i.state = StateCleanupFailed
		return fmt.Errorf("stop instance %s: %w", i.ID, errors.Join(errs...))
	}

	i.state = StateStopped
	return nil
}
