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

// Instance represents a single Firecracker microVM.
type Instance struct {
	ID         string
	SocketPath string
	cfg        Config
	netCfg     NetworkConfig
	vsockCfg   VsockConfig
	machine    *fc.Machine
	cleanups   []func() error
}

func NewInstance(id, socketPath string, cfg Config, netCfg NetworkConfig, vsockCfg VsockConfig) *Instance {
	return &Instance{
		ID:         id,
		SocketPath: socketPath,
		cfg:        cfg,
		netCfg:     netCfg,
		vsockCfg:   vsockCfg,
	}
}

// generateMAC creates a deterministic, locally-administered MAC address from the VM ID.
func generateMAC(vmID string) string {
	hash := sha256.Sum256([]byte(vmID))

	mac := net.HardwareAddr{
		0x02,           // Locally administered, unicast.
		hash[0] & 0xFE, // Clear multicast bit.
		hash[1],
		hash[2],
		hash[3],
		hash[4],
	}

	return mac.String()
}

// Configure prepares the host network, firewall, and initializes the Firecracker Machine.
func (i *Instance) Configure(ctx context.Context) error {
	if err := i.cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// 1. Setup TAP.
	if err := SetupTAP(ctx, i.netCfg); err != nil {
		return fmt.Errorf("setup tap: %w", err)
	}
	i.cleanups = append(i.cleanups, func() error {
		return CleanupTAP(ctx, i.netCfg.TapName)
	})

	// 2. Setup firewall.
	if err := SetupFirewall(ctx, i.netCfg); err != nil {
		return fmt.Errorf("setup firewall: %w", err)
	}
	i.cleanups = append(i.cleanups, func() error {
		return CleanupFirewall(ctx, i.netCfg)
	})

	// 3. Generate unique MAC.
	macAddr := generateMAC(i.ID)

	// 4. Parse guest IP/CIDR.
	guestIP, guestNetwork, err := net.ParseCIDR(i.netCfg.GuestIP)
	if err != nil {
		return fmt.Errorf("parse guest IP %q: %w", i.netCfg.GuestIP, err)
	}

	// Keep the host portion of the address while using the supplied subnet.
	guestNetwork.IP = guestIP

	// 5. Parse gateway IP.
	gatewayIP := net.ParseIP(i.netCfg.HostIP)
	if gatewayIP == nil {
		return fmt.Errorf("parse host/gateway IP %q: invalid IP", i.netCfg.HostIP)
	}

	// 6. Build Firecracker SDK configuration.
	fcConfig := fc.Config{
		SocketPath:      i.SocketPath,
		KernelImagePath: i.cfg.KernelPath,
		KernelArgs:      i.cfg.BootArgs,

		Drives: []models.Drive{
			{
				DriveID:      fc.String("rootfs"),
				PathOnHost:   fc.String(i.cfg.RootfsPath),
				IsRootDevice: fc.Bool(true),
				IsReadOnly:   fc.Bool(false),
			},
		},

		MachineCfg: models.MachineConfiguration{
			VcpuCount:  fc.Int64(int64(i.cfg.VCPUCount)),
			MemSizeMib: fc.Int64(int64(i.cfg.MemSizeMB)),
		},

		NetworkInterfaces: []fc.NetworkInterface{
			{
				StaticConfiguration: &fc.StaticNetworkConfiguration{
					HostDevName: i.netCfg.TapName,
					MacAddress:  macAddr,
					IPConfiguration: &fc.IPConfiguration{
						IPAddr:  *guestNetwork,
						Gateway: gatewayIP,
					},
				},
			},
		},

		VsockDevices: []fc.VsockDevice{
			{
				ID:  "vsock0",
				CID: i.vsockCfg.GuestCID,
			},
		},
	}

	// 7. Create the Machine object.
	machine, err := fc.NewMachine(ctx, fcConfig)
	if err != nil {
		return fmt.Errorf("create firecracker machine: %w", err)
	}

	i.machine = machine
	return nil
}

// Start boots the VMM process and the guest OS.
func (i *Instance) Start(ctx context.Context) error {
	if i.machine == nil {
		return fmt.Errorf("machine not configured")
	}

	// The current SDK starts the VMM and initializes the guest through Start.
	if err := i.machine.Start(ctx); err != nil {
		return fmt.Errorf("start guest os: %w", err)
	}

	return nil
}

// Stop gracefully shuts down the VMM and cleans up all host resources.
func (i *Instance) Stop(ctx context.Context) error {
	if i.machine != nil {
		if err := i.machine.StopVMM(); err != nil {
			fmt.Printf("warning: stop vmm failed for vm %s: %v\n", i.ID, err)
		}
	}

	// Run all cleanups in reverse order.
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
