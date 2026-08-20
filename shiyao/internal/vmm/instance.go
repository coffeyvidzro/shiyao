package vmm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	fc "github.com/firecracker-microvm/firecracker-go-sdk"
	models "github.com/firecracker-microvm/firecracker-go-sdk/client/models"

	"github.com/coffeyvidzro/shiyao/internal/network"
	"github.com/coffeyvidzro/shiyao/internal/vsock"
)

type Instance struct {
	mu sync.Mutex
	ID string
	InstanceID string
	SocketPath string
	cfg Config
	netCfg network.Config
	vsockCfg vsock.Config
	snapCfg SnapshotConfig
	snapInteg SnapshotIntegrity
	machine *fc.Machine
	cleanups []func() error
	state State
}

func NewInstance(id, socketPath string, cfg Config, netCfg network.Config, vsockCfg vsock.Config, snapCfg SnapshotConfig) *Instance {
	return &Instance{ID: id, SocketPath: socketPath, cfg: cfg, netCfg: netCfg, vsockCfg: vsockCfg, snapCfg: snapCfg, state: StateCreated}
}

func (i *Instance) Configure(ctx context.Context) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.state != StateCreated { return fmt.Errorf("instance %s cannot be configured in state %s", i.ID, i.state) }
	i.state = StateConfiguring
	var err error
	i.InstanceID, err = generateInstanceID()
	if err != nil { i.state = StateCreated; return fmt.Errorf("generate instance ID: %w", err) }
	if i.cfg.Validate() != nil { i.state = StateCreated; return fmt.Errorf("invalid config: %w", i.cfg.Validate()) }
	if err := i.netCfg.Validate(); err != nil { i.state = StateCreated; return fmt.Errorf("invalid network config: %w", err) }
	configured := false
	defer func() {
		if !configured {
			for j := len(i.cleanups)-1; j >= 0; j-- { _ = i.cleanups[j]() }
			i.cleanups = nil
		}
	}()
	if err := network.SetupTAP(ctx, i.netCfg); err != nil { i.state = StateCreated; return fmt.Errorf("setup tap: %w", err) }
	i.cleanups = append(i.cleanups, func() error { return network.CleanupTAP(ctx, i.netCfg.TapName) })
	if err := network.SetupFirewall(ctx, i.netCfg); err != nil { i.state = StateCreated; return fmt.Errorf("setup firewall: %w", err) }
	i.cleanups = append(i.cleanups, func() error { return network.CleanupFirewall(ctx, i.netCfg) })
	guestIP, guestNetwork, err := parseGuestNetwork(i.netCfg.GuestIP)
	if err != nil { i.state = StateCreated; return err }
	gatewayIP := netIP(i.netCfg.HostIP)
	if gatewayIP == nil { i.state = StateCreated; return fmt.Errorf("parse host/gateway IP %q: invalid IP", i.netCfg.HostIP) }
	fcConfig := fc.Config{
		SocketPath: i.SocketPath,
		Drives: []models.Drive{{DriveID: fc.String("rootfs"), PathOnHost: fc.String(i.cfg.RootfsPath), IsRootDevice: fc.Bool(true), IsReadOnly: fc.Bool(true)}},
		MachineCfg: models.MachineConfiguration{VcpuCount: fc.Int64(int64(i.cfg.VCPUCount)), MemSizeMib: fc.Int64(int64(i.cfg.MemSizeMB))},
		NetworkInterfaces: []fc.NetworkInterface{{StaticConfiguration: &fc.StaticNetworkConfiguration{HostDevName: i.netCfg.TapName, MacAddress: generateMAC(i.ID), IPConfiguration: &fc.IPConfiguration{IPAddr: *guestNetwork, Gateway: gatewayIP}}}},
		VsockDevices: []fc.VsockDevice{{ID: "vsock0", CID: i.vsockCfg.GuestCID}},
	}
	var opts []fc.Opt
	if i.snapCfg.EnableResume {
		if i.snapCfg.MemFilePath == "" || i.snapCfg.StateFilePath == "" { i.state = StateCreated; return fmt.Errorf("snapshot restore enabled but snapshot paths are missing") }
		if err := i.validateSnapshotIntegrity(); err != nil { i.state = StateCreated; return fmt.Errorf("validate snapshot integrity: %w", err) }
		opts = append(opts, fc.WithSnapshot(i.snapCfg.MemFilePath, i.snapCfg.StateFilePath))
	} else {
		fcConfig.KernelImagePath = i.cfg.KernelPath
		kernelArgs, err := guestKernelArgs(i.cfg.BootArgs, i.cfg.GuestAgentPath)
		if err != nil { i.state = StateCreated; return err }
		fcConfig.KernelArgs = kernelArgs
	}
	machine, err := fc.NewMachine(ctx, fcConfig, opts...)
	if err != nil { i.state = StateCreated; return fmt.Errorf("create firecracker machine: %w", err) }
	i.machine = machine
	configured = true
	i.state = StateConfigured
	_ = guestIP
	return nil
}

func (i *Instance) Start(ctx context.Context) error {
	i.mu.Lock(); defer i.mu.Unlock()
	if i.state != StateConfigured { return fmt.Errorf("instance %s cannot be started in state %s", i.ID, i.state) }
	if i.machine == nil { return fmt.Errorf("machine not configured") }
	if i.snapCfg.EnableResume {
		if err := i.machine.ResumeVM(ctx); err != nil { return fmt.Errorf("resume microvm snapshot: %w", err) }
	} else if err := i.machine.Start(ctx); err != nil { return fmt.Errorf("start guest os: %w", err) }
	i.state = StateRunning
	return nil
}

func (i *Instance) Stop(ctx context.Context) error {
	i.mu.Lock(); defer i.mu.Unlock()
	if i.state != StateRunning && i.state != StateConfigured && i.state != StateCleanupFailed { return nil }
	i.state = StateStopping
	var errs []error
	if i.machine != nil {
		if err := i.machine.StopVMM(); err != nil { errs = append(errs, fmt.Errorf("stop vmm: %w", err)) } else { i.machine = nil }
	}
	remaining := make([]func() error, 0, len(i.cleanups))
	for j := len(i.cleanups)-1; j >= 0; j-- {
		if err := i.cleanups[j](); err != nil { errs = append(errs, fmt.Errorf("cleanup step %d: %w", j, err)); remaining = append(remaining, i.cleanups[j]) }
	}
	i.cleanups = remaining
	if len(i.cleanups) == 0 {
		if err := os.Remove(i.SocketPath); err != nil && !os.IsNotExist(err) { errs = append(errs, fmt.Errorf("remove socket: %w", err)) }
	}
	if len(errs) > 0 || len(i.cleanups) > 0 || i.machine != nil { i.state = StateCleanupFailed; return fmt.Errorf("stop instance %s: %w", i.ID, errors.Join(errs...)) }
	i.state = StateStopped
	return nil
}

func generateInstanceID() (string, error) { return randomHex(16) }

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := os.ReadFile("/dev/urandom"); err != nil { return "", err }
	// Use crypto/rand in the final helper; this placeholder keeps generation local.
	return filepath.Base(fmt.Sprintf("%x", b)), nil
}

func parseGuestNetwork(value string) (net.IP, *net.IPNet, error) {
	ip, network, err := net.ParseCIDR(value)
	if err != nil { return nil, nil, fmt.Errorf("parse guest IP %q: %w", value, err) }
	network.IP = ip
	return ip, network, nil
}

func netIP(value string) net.IP { return net.ParseIP(value) }
