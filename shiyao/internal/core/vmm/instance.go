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

type networkLease interface {
	Config() network.Config
	CID() uint32
	Setup(context.Context) error
	Release(context.Context) error
}

type Instance struct {
	mu         sync.Mutex
	ID         string
	InstanceID string
	SocketPath string
	cfg        Config
	network    networkLease
	vsockCfg   vsock.Config
	snapCfg    SnapshotConfig
	snapInteg  SnapshotIntegrity
	machine    *fc.Machine
	state      State
}

func NewInstance(id, socketPath string, cfg Config, allocation networkLease, vsockCfg vsock.Config, snapCfg SnapshotConfig) *Instance {
	return &Instance{ID: id, SocketPath: socketPath, cfg: cfg, network: allocation, vsockCfg: vsockCfg, snapCfg: snapCfg, state: StateCreated}
}

func (i *Instance) State() State {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.state
}

func (i *Instance) Configure(ctx context.Context) (retErr error) {
	instanceID, err := generateInstanceID()
	if err != nil {
		return fmt.Errorf("generate instance ID: %w", err)
	}
	if err := i.cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	if err := validateRuntimeAssets(i.cfg, i.snapCfg); err != nil {
		return err
	}
	netCfg := i.network.Config()
	if err := netCfg.Validate(); err != nil {
		return fmt.Errorf("invalid network config: %w", err)
	}

	i.mu.Lock()
	if i.state != StateCreated {
		state := i.state
		i.mu.Unlock()
		return fmt.Errorf("instance %s cannot be configured in state %s", i.ID, state)
	}
	i.state = StateConfiguring
	i.InstanceID = instanceID
	cfg := i.cfg
	networkLease := i.network
	vsockCfg := i.vsockCfg
	snapCfg := i.snapCfg
	socketPath := i.SocketPath
	i.mu.Unlock()

	configured := false
	defer func() {
		if configured {
			return
		}
		cleanupErr := networkLease.Release(context.Background())
		if cleanupErr != nil {
			retErr = errors.Join(retErr, fmt.Errorf("cleanup network resources after configure failure: %w", cleanupErr))
			i.mu.Lock()
			i.state = StateCleanupFailed
			i.mu.Unlock()
			return
		}
		i.mu.Lock()
		i.state = StateCreated
		i.mu.Unlock()
	}()

	if err := networkLease.Setup(ctx); err != nil {
		return fmt.Errorf("setup network resources: %w", err)
	}

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
		SocketPath:        socketPath,
		Drives:            []models.Drive{{DriveID: fc.String("rootfs"), PathOnHost: fc.String(cfg.RootfsPath), IsRootDevice: fc.Bool(true), IsReadOnly: fc.Bool(true)}},
		MachineCfg:        models.MachineConfiguration{VcpuCount: fc.Int64(int64(cfg.VCPUCount)), MemSizeMib: fc.Int64(int64(cfg.MemSizeMB))},
		NetworkInterfaces: []fc.NetworkInterface{{StaticConfiguration: &fc.StaticNetworkConfiguration{HostDevName: netCfg.TapName, MacAddress: generateMAC(i.ID), IPConfiguration: &fc.IPConfiguration{IPAddr: *guestNetwork, Gateway: gatewayIP}}}},
		VsockDevices:      []fc.VsockDevice{{ID: "vsock0", CID: vsockCfg.GuestCID}},
	}
	var opts []fc.Opt
	if snapCfg.EnableResume {
		if snapCfg.MemFilePath == "" || snapCfg.StateFilePath == "" {
			return fmt.Errorf("snapshot restore enabled but snapshot paths are missing")
		}
		if err := i.validateSnapshotIntegrity(); err != nil {
			return fmt.Errorf("validate snapshot integrity: %w", err)
		}
		opts = append(opts, fc.WithSnapshot(snapCfg.MemFilePath, snapCfg.StateFilePath))
	} else {
		fcConfig.KernelImagePath = cfg.KernelPath
		kernelArgs, err := guestKernelArgs(cfg.BootArgs, cfg.GuestAgentPath)
		if err != nil {
			return err
		}
		fcConfig.KernelArgs = kernelArgs
	}
	machine, err := fc.NewMachine(ctx, fcConfig, opts...)
	if err != nil {
		return fmt.Errorf("create firecracker machine: %w", err)
	}

	i.mu.Lock()
	i.machine = machine
	i.state = StateConfigured
	i.mu.Unlock()
	configured = true
	return nil
}

func (i *Instance) Start(ctx context.Context) error {
	i.mu.Lock()
	if i.state != StateConfigured {
		state := i.state
		i.mu.Unlock()
		return fmt.Errorf("instance %s cannot be started in state %s", i.ID, state)
	}
	if i.machine == nil {
		i.mu.Unlock()
		return fmt.Errorf("machine not configured")
	}
	i.state = StateStarting
	machine := i.machine
	snapCfg := i.snapCfg
	i.mu.Unlock()

	var err error
	if snapCfg.EnableResume {
		err = machine.ResumeVM(ctx)
	} else {
		err = machine.Start(ctx)
	}
	if err != nil {
		i.mu.Lock()
		i.state = StateConfigured
		i.mu.Unlock()
		if snapCfg.EnableResume {
			return fmt.Errorf("resume microvm snapshot: %w", err)
		}
		return fmt.Errorf("start guest os: %w", err)
	}

	i.mu.Lock()
	if i.state != StateStarting {
		state := i.state
		i.mu.Unlock()
		return fmt.Errorf("instance %s start completed in unexpected state %s", i.ID, state)
	}
	i.state = StateRunning
	i.mu.Unlock()
	return nil
}

func (i *Instance) Stop(ctx context.Context) error {
	i.mu.Lock()
	if i.state != StateRunning && i.state != StateConfigured && i.state != StateCleanupFailed {
		i.mu.Unlock()
		return nil
	}
	i.state = StateStopping
	machine := i.machine
	networkLease := i.network
	socketPath := i.SocketPath
	i.mu.Unlock()

	var errs []error
	machineStopped := machine == nil
	if machine != nil {
		if err := machine.StopVMM(); err != nil {
			errs = append(errs, fmt.Errorf("stop vmm: %w", err))
		} else {
			machineStopped = true
		}
	}

	networkReleased := false
	if networkLease != nil {
		if err := networkLease.Release(ctx); err != nil {
			errs = append(errs, fmt.Errorf("release network resources: %w", err))
		} else {
			networkReleased = true
		}
	}

	socketRemoved := false
	if machineStopped {
		if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
			errs = append(errs, fmt.Errorf("remove socket: %w", err))
		} else {
			socketRemoved = true
		}
	}

	i.mu.Lock()
	defer i.mu.Unlock()
	if machineStopped {
		i.machine = nil
	}
	if len(errs) > 0 || !machineStopped || !networkReleased || !socketRemoved {
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
