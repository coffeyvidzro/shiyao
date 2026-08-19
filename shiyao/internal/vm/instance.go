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

func parseGuestIPConfig(cfg NetworkConfig) (*fc.IPConfiguration, error) {
	guestAddr, err := parseIPNet(cfg.GuestIP)
	if err != nil {
		return nil, fmt.Errorf("parse guest IP %s: %w", cfg.GuestIP, err)
	}

	gateway, _, err := net.ParseCIDR(cfg.HostIP)
	if err != nil {
		return nil, fmt.Errorf("parse host gateway IP %s: %w", cfg.HostIP, err)
	}

	return &fc.IPConfiguration{
		IPAddr:  guestAddr,
		Gateway: gateway,
	}, nil
}

func parseIPNet(value string) (net.IPNet, error) {
	ip, ipNet, err := net.ParseCIDR(value)
	if err != nil {
		return net.IPNet{}, err
	}
	ipNet.IP = ip
	return *ipNet, nil
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
	guestIP, err := parseGuestIPConfig(i.netCfg)
	if err != nil {
		return err
	}

	driveID := "rootfs"
	isRoot := true
	isReadOnly := false
	rootfsPath := i.cfg.RootfsPath
	vcpuCount := int64(i.cfg.VCPUCount)
	memSize := int64(i.cfg.MemSizeMB)

	fcConfig := fc.Config{
		SocketPath:      i.SocketPath,
		KernelImagePath: i.cfg.KernelPath,
		KernelArgs:      i.cfg.BootArgs,
		Drives: []models.Drive{
			{
				DriveID:      &driveID,
				PathOnHost:   &rootfsPath,
				IsRootDevice: &isRoot,
				IsReadOnly:   &isReadOnly,
			},
		},
		MachineCfg: models.MachineConfiguration{
			VcpuCount:  &vcpuCount,
			MemSizeMib: &memSize,
		},
		NetworkInterfaces: []fc.NetworkInterface{
			{
				StaticConfiguration: &fc.StaticNetworkConfiguration{
					HostDevName: i.netCfg.TapName,
					MacAddress:  macAddr,
					// Explicitly define guest IP configuration. These values must
					// match the subnet configured on the host TAP device.
					IPConfiguration: guestIP,
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

	// Start launches the Firecracker process, configures the microVM over the
	// API socket, and sends the InstanceStart action to boot the guest OS.
	if err := i.machine.Start(ctx); err != nil {
		return fmt.Errorf("start firecracker machine: %w", err)
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
