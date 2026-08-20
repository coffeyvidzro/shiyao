package vm

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net"
	"os"
	"strings"

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
	ID         string
	SocketPath string
	cfg        Config
	netCfg     NetworkConfig
	vsockCfg   VsockConfig
	snapCfg    SnapshotConfig
	machine    *fc.Machine
	cleanups   []func() error
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
		SocketPath: socketPath,
		cfg:        cfg,
		netCfg:     netCfg,
		vsockCfg:   vsockCfg,
		snapCfg:    snapCfg,
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
	if err := i.cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// 1. Provision host TAP network and firewall isolation
	if err := SetupTAP(ctx, i.netCfg); err != nil {
		return fmt.Errorf("setup tap: %w", err)
	}
	i.cleanups = append(i.cleanups, func() error { return CleanupTAP(ctx, i.netCfg.TapName) })

	if err := SetupFirewall(ctx, i.netCfg); err != nil {
		return fmt.Errorf("setup firewall: %w", err)
	}
	i.cleanups = append(i.cleanups, func() error { return CleanupFirewall(ctx, i.netCfg) })

	macAddr := generateMAC(i.ID)
	guestIP, guestNetwork, err := net.ParseCIDR(i.netCfg.GuestIP)
	if err != nil {
		return fmt.Errorf("parse guest IP %q: %w", i.netCfg.GuestIP, err)
	}
	guestNetwork.IP = guestIP
	gatewayIP := net.ParseIP(i.netCfg.HostIP)
	if gatewayIP == nil {
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
		return fmt.Errorf("create firecracker machine: %w", err)
	}
	i.machine = machine
	return nil
}

func (i *Instance) Start(ctx context.Context) error {
	if i.machine == nil {
		return fmt.Errorf("machine not configured")
	}

	// Resume from snapshot or execute standard start
	if i.snapCfg.EnableResume {
		if err := i.machine.ResumeVM(ctx); err != nil {
			return fmt.Errorf("resume microvm snapshot: %w", err)
		}
		return nil
	}

	if err := i.machine.Start(ctx); err != nil {
		return fmt.Errorf("start guest os: %w", err)
	}
	return nil
}

func (i *Instance) Stop(ctx context.Context) error {
	if i.machine != nil {
		if err := i.machine.StopVMM(); err != nil {
			fmt.Printf("warning: stop vmm failed for vm %s: %v\n", i.ID, err)
		}
	}
	for j := len(i.cleanups) - 1; j >= 0; j-- {
		if err := i.cleanups[j](); err != nil {
			fmt.Printf("warning: cleanup failed for vm %s: %v\n", i.ID, err)
		}
	}
	if err := os.Remove(i.SocketPath); err != nil && !os.IsNotExist(err) {
		fmt.Printf("warning: remove socket failed for vm %s: %v\n", i.ID, err)
	}
	return nil
}
