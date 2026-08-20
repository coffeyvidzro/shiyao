package vmm

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"

	fc "github.com/firecracker-microvm/firecracker-go-sdk"
	models "github.com/firecracker-microvm/firecracker-go-sdk/client/models"

	"github.com/coffeyvidzro/shiyao/internal/network"
	"github.com/coffeyvidzro/shiyao/internal/vsock"
)

type Instance struct {
	mu         sync.Mutex
	ID         string
	InstanceID string
	SocketPath string
	cfg        Config
	network    *network.Allocation
	vsockCfg   vsock.Config
	snapCfg    SnapshotConfig
	snapInteg  SnapshotIntegrity
	machine    *fc.Machine
	state      State
}

func NewInstance(id, socketPath string, cfg Config, allocation *network.Allocation, vsockCfg vsock.Config, snapCfg SnapshotConfig) *Instance {
	return &Instance{ID: id, SocketPath: socketPath, cfg: cfg, network: allocation, vsockCfg: vsockCfg, snapCfg: snapCfg, state: StateCreated}
}

func (i *Instance) Configure(ctx context.Context) (retErr error) {
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
	if err := i.cfg.Validate(); err != nil {
		i.state = StateCreated
		return fmt.Errorf("invalid config: %w", err)
	}
	netCfg := i.network.Config()
	if err := netCfg.Validate(); err != nil {
		i.state = StateCreated
		return fmt.Errorf("invalid network config: %w", err)
	}
	if err := i.network.Setup(ctx); err != nil {
		i.state = StateCreated
		return fmt.Errorf("setup network resources: %w", err)
	}
	configured := false
	defer func() {
		if configured {
			return
		}
		cleanupErr := i.network.Release(context.Background())
		if cleanupErr != nil {
			i.state = StateCleanupFailed
			retErr = errors.Join(retErr, fmt.Errorf("cleanup network resources after configure failure: %w", cleanupErr))
			return
		}
		i.state = StateCreated
	}()
	guestIP, guestNetwork, err := net.ParseCIDR(netCfg.GuestIP)
	if err != nil {
		return fmt.Errorf("parse guest IP %q: %w", netCfg.GuestIP, err)
	}
	guestNetwork.IP = guestIP
	gatewayIP := net.ParseIP(netCfg.HostIP)
	if gatewayIP == nil {
		return fmt.Errorf("parse host/gateway IP %q: invalid IP", netCfg.HostIP)
	}
	fcConfig := fc.Config{
		SocketPath: i.SocketPath,
		Drives: []models.Drive{{DriveID: fc.String("rootfs"), PathOnHost: fc.String(i.cfg.RootfsPath), IsRootDevice: fc.Bool(true), IsReadOnly: fc.Bool(true)}},
		MachineCfg: models.MachineConfiguration{VcpuCount: fc.Int64(int64(i.cfg.VCPUCount)), MemSizeMib: fc.Int64(int64(i.cfg.MemSizeMB))},
		NetworkInterfaces: []fc.NetworkInterface{{StaticConfiguration: &fc.StaticNetworkConfiguration{HostDevName: netCfg.TapName, MacAddress: generateMAC(i.ID), IPConfiguration: &fc.IPConfiguration{IPAddr: *guestNetwork, Gateway: gatewayIP}}}},
		VsockDevices: []fc.VsockDevice{{ID: "vsock0", CID: i.vsockCfg.GuestCID}},
	}
	var opts []fc.Opt
	if i.snapCfg.EnableResume {
		if i.snapCfg.MemFilePath == "" || i.snapCfg.StateFilePath == "" {
			return fmt.Errorf("snapshot restore enabled but snapshot paths are missing")
		}
		if err := i.validateSnapshotIntegrity(); err != nil {
			return fmt.Errorf("validate snapshot integrity: %w", err)
		}
		opts = append(opts, fc.WithSnapshot(i.snapCfg.MemFilePath, i.snapCfg.StateFilePath))
	} else {
		fcConfig.KernelImagePath = i.cfg.KernelPath
		kernelArgs, err := guestKernelArgs(i.cfg.BootArgs, i.cfg.GuestAgentPath)
		if err != nil {
			return err
		}
		fcConfig.KernelArgs = kernelArgs
	}
	machine, err := fc.NewMachine(ctx, fcConfig, opts...)
	if err != nil {
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
	} else if err := i.machine.Start(ctx); err != nil {
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
	if i.network != nil {
		if err := i.network.Release(ctx); err != nil {
			errs = append(errs, fmt.Errorf("release network resources: %w", err))
		}
	}
	if i.machine == nil {
		if err := os.Remove(i.SocketPath); err != nil && !os.IsNotExist(err) {
			errs = append(errs, fmt.Errorf("remove socket: %w", err))
		}
	}
	if len(errs) > 0 || i.machine != nil {
		i.state = StateCleanupFailed
		return fmt.Errorf("stop instance %s: %w", i.ID, errors.Join(errs...))
	}
	i.state = StateStopped
	return nil
}

func generateInstanceID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
