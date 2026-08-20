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
	return &Instance{ID: id, SocketPath: socketPath, cfg: cfg, netCfg: netCfg, vsockCfg: vsockCfg}
}

func generateMAC(vmID string) string {
	hash := sha256.Sum256([]byte(vmID))
	mac := net.HardwareAddr{0x02, hash[0] & 0xFE, hash[1], hash[2], hash[3], hash[4]}
	return mac.String()
}

func (i *Instance) Configure(ctx context.Context) error {
	if err := i.cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
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

	kernelArgs := i.cfg.BootArgs
	if !strings.Contains(kernelArgs, "init=") {
		kernelArgs = strings.TrimSpace(kernelArgs + " init=" + i.cfg.GuestAgentPath)
	}

	fcConfig := fc.Config{
		SocketPath:      i.SocketPath,
		KernelImagePath: i.cfg.KernelPath,
		KernelArgs:      kernelArgs,
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

	machine, err := fc.NewMachine(ctx, fcConfig)
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
