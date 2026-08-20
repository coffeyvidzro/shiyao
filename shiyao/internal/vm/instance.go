package vm

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
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

type Instance struct {
	mu         sync.Mutex
	ID         string
	InstanceID string // Random instance ID for host resource naming
	SocketPath string
	cfg        Config
	netCfg     NetworkConfig
	vsockCfg   VsockConfig
	snapCfg    SnapshotConfig
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

func guestKernelArgs(bootArgs, guestAgentPath string) string {
	bootArgs = strings.TrimSpace(bootArgs)
	if strings.Contains(bootArgs, "init=") || guestAgentPath == "" {
		return bootArgs
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

	// Generate random instance ID for host resource naming
	var err error
	i.InstanceID, err = generateInstanceID()
	if err != nil {
		return fmt.Errorf("generate instance ID: %w", err)
	}

	// Update TAP name to use instance ID for unpredictable naming
	i.netCfg.TapName = uniqueTapNameWithInstanceID(i.InstanceID)

	if err := i.cfg.Validate(); err != nil {
		i.state = StateCreated
		return fmt.Errorf("invalid config: %w", err)
	}

	// Use rollback pattern to clean up on configuration failure
	configured := false
	defer func() {
		if !configured {
			// Rollback cleanups in reverse order
			for j := len(i.cleanups) - 1; j >= 0; j-- {
				if cleanupErr := i.cleanups[j](); cleanupErr != nil {
					fmt.Printf("warning: rollback cleanup failed for vm %s: %v\n", i.ID, cleanupErr)
				}
			}
			i.cleanups = nil
		}
	}()

	// 1. Provision host TAP network and firewall isolation
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

	// 2. Build base Firecracker VM Configuration
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

	// 3. Branching: Snapshot Resume vs. Full Boot Path
	var opts []fc.Opt
	if i.snapCfg.EnableResume {
		if i.snapCfg.MemFilePath == "" || i.snapCfg.StateFilePath == "" {
			i.state = StateCreated
			return fmt.Errorf("snapshot restore enabled but snapshot paths are missing")
		}

		// Inject Snapshot Opt into Firecracker SDK
		opts = append(opts, fc.WithSnapshot(i.snapCfg.MemFilePath, i.snapCfg.StateFilePath))
	} else {
		// Kernel path and boot parameters are only needed for standard cold boot
		fcConfig.KernelImagePath = i.cfg.KernelPath
		fcConfig.KernelArgs = guestKernelArgs(i.cfg.BootArgs, i.cfg.GuestAgentPath)
	}

	// 4. Instantiate Machine
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

	// Resume from snapshot or execute standard start
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

	// Allow Stop to be called from Running or Configured state
	if i.state != StateRunning && i.state != StateConfigured {
		// Already stopped or stopping - return nil for idempotency
		return nil
	}
	i.state = StateStopping

	var errs []error

	if i.machine != nil {
		if err := i.machine.StopVMM(); err != nil {
			errs = append(errs, fmt.Errorf("stop vmm: %w", err))
		}
	}

	// Execute cleanups in reverse order and collect all errors
	for j := len(i.cleanups) - 1; j >= 0; j-- {
		if err := i.cleanups[j](); err != nil {
			errs = append(errs, fmt.Errorf("cleanup step %d: %w", j, err))
		}
	}

	// Clean up socket file
	if err := os.Remove(i.SocketPath); err != nil && !os.IsNotExist(err) {
		errs = append(errs, fmt.Errorf("remove socket: %w", err))
	}

	// Clear cleanups after execution
	i.cleanups = nil
	i.state = StateStopped

	if len(errs) > 0 {
		return fmt.Errorf("stop instance %s: %w", i.ID, errors.Join(errs...))
	}
	return nil
}
