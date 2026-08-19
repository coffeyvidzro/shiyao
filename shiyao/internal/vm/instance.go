package vm

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net"
	"os"

	fc "github.com/firecracker-microvm/firecracker-go-sdk"
	models "github.com/firecracker-microvm/firecracker-go-sdk/client/models"
)

// Instance represents a single Firecracker microVM managed by the SDK.
type Instance struct {
	ID         string
	SocketPath string
	cfg        Config
	netCfg     NetworkConfig
	machine    *fc.Machine
	cleanupNet func() error
}

// NewInstance creates a new VM instance representation.
func NewInstance(id, socketPath string, cfg Config, netCfg NetworkConfig) *Instance {
	return &Instance{
		ID:         id,
		SocketPath: socketPath,
		cfg:        cfg,
		netCfg:     netCfg,
	}
}

// generateMAC creates a deterministic, locally-administered MAC address from the VM ID.
// This prevents MAC collisions when running multiple VMs concurrently on the same host.
func generateMAC(vmID string) string {
	hash := sha256.Sum256([]byte(vmID))
	// Set the locally administered bit (second least significant bit of the first byte)
	// and clear the multicast bit to ensure it's a valid unicast MAC.
	mac := net.HardwareAddr{
		0x02,           // Locally administered
		hash[0] & 0xFE, // Clear multicast bit
		hash[1],
		hash[2],
		hash[3],
		hash[4],
	}
	return mac.String()
}

// Configure prepares the host network (TAP + iptables) and initializes the
// Firecracker Machine struct without starting the VMM process yet.
func (i *Instance) Configure(ctx context.Context) error {
	if i.cfg.KernelPath == "" || i.cfg.RootfsPath == "" {
		return fmt.Errorf("kernel and rootfs paths are required")
	}

	// 1. Setup Host Network (TAP device + Egress iptables rules)
	cleanup, err := SetupNetwork(ctx, i.netCfg)
	if err != nil {
		return fmt.Errorf("setup network: %w", err)
	}
	i.cleanupNet = cleanup

	// 2. Generate a unique MAC address for this specific VM
	macAddr := generateMAC(i.ID)

	// 3. Build the Firecracker SDK Configuration
	fcConfig := fc.Config{
		SocketPath:      i.SocketPath,
		KernelImagePath: i.cfg.KernelPath,
		KernelArgs:      i.cfg.BootArgs,
		RootDrive: &fc.BlockDevice{
			HostPath:     i.cfg.RootfsPath,
			DriveID:      "rootfs",
			IsRootDevice: true,
			IsReadOnly:   false,
		},
		MachineCfg: models.MachineConfiguration{
			VcpuCount:  models.Int64(int64(i.cfg.VCPUCount)),
			MemSizeMiB: models.Int64(int64(i.cfg.MemSizeMB)),
		},
		NetworkInterfaces: []fc.NetworkInterface{
			{
				StaticConfig: &fc.StaticNetworkConfiguration{
					HostDevName: i.netCfg.TapName,
					MacAddress:  macAddr,
					// Explicitly define the Guest IP configuration.
					// These must match the subnet configured on the host TAP device.
					IPConfig: &fc.IPConfiguration{
						Address: i.netCfg.GuestIP, // e.g., "172.16.0.2/24"
						Gateway: i.netCfg.HostIP,  // e.g., "172.16.0.1"
					},
				},
			},
		},
	}

	// 4. Create the Machine object (does not start the VMM process yet)
	machine, err := fc.NewMachine(ctx, fcConfig)
	if err != nil {
		return fmt.Errorf("create firecracker machine: %w", err)
	}

	i.machine = machine
	return nil
}

// Start boots the VMM process and sends the action to start the guest OS.
func (i *Instance) Start(ctx context.Context) error {
	if i.machine == nil {
		return fmt.Errorf("machine not configured")
	}

	// StartVMM launches the Firecracker process in the background.
	// We use this instead of machine.Start() so our Go daemon remains responsive.
	if err := i.machine.StartVMM(); err != nil {
		return fmt.Errorf("start vmm process: %w", err)
	}

	// Send the InstanceStart action to boot the guest kernel.
	if err := i.machine.Start(ctx); err != nil {
		// If boot fails, ensure we clean up the VMM process we just started.
		i.machine.StopVMM()
		return fmt.Errorf("start guest os: %w", err)
	}

	return nil
}

// Stop gracefully shuts down the VMM process and cleans up all host resources
// (network rules, TAP device, and the Unix socket file).
func (i *Instance) Stop(ctx context.Context) error {
	// 1. Kill the Firecracker VMM process
	if i.machine != nil {
		i.machine.StopVMM()
	}

	// 2. Clean up network rules and TAP device
	if i.cleanupNet != nil {
		if err := i.cleanupNet(); err != nil {
			fmt.Printf("warning: failed to cleanup network for vm %s: %v\n", i.ID, err)
		}
	}

	// 3. Remove the Unix socket file from disk to prevent orphaned sockets
	os.Remove(i.SocketPath)

	return nil
}
