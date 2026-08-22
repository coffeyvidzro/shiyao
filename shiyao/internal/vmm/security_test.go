package vmm

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/coffeyvidzro/shiyao/internal/network"
	"github.com/coffeyvidzro/shiyao/internal/vsock"
)

type testNetworkLease struct{ released bool }

func (l *testNetworkLease) Config() network.Config { return network.Config{} }
func (l *testNetworkLease) CID() uint32 { return 3 }
func (l *testNetworkLease) Setup(context.Context) error { return nil }
func (l *testNetworkLease) Release(context.Context) error { l.released = true; return nil }

func TestValidateRuntimeAssetsRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	kernel := filepath.Join(dir, "kernel")
	rootfs := filepath.Join(dir, "rootfs")
	if err := os.WriteFile(kernel, []byte("kernel"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rootfs, []byte("rootfs"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "rootfs-link")
	if err := os.Symlink(rootfs, link); err != nil {
		t.Fatal(err)
	}
	cfg := Config{KernelPath: kernel, RootfsPath: link, VCPUCount: 1, MemSizeMB: 128, GuestAgentPath: "/usr/local/bin/shiyao-agent"}
	if err := validateRuntimeAssets(cfg, SnapshotConfig{}); err == nil {
		t.Fatal("expected symlink rootfs to be rejected")
	}
}

func TestWarmPoolRequiresRunningInstances(t *testing.T) {
	pool, err := NewWarmPool(1)
	if err != nil {
		t.Fatal(err)
	}
	inst := NewInstance("vm", "", Config{}, nil, vsock.Config{}, SnapshotConfig{})
	if err := pool.Add(inst); err == nil {
		t.Fatal("expected non-running instance to be rejected")
	}
}

func TestWarmPoolSuccessfulCheckinRequiresRunningState(t *testing.T) {
	pool, err := NewWarmPool(1)
	if err != nil {
		t.Fatal(err)
	}
	inst := &Instance{ID: "vm", state: StateRunning}
	if err := pool.Add(inst); err != nil {
		t.Fatal(err)
	}
	checkedOut, err := pool.Checkout("lease")
	if err != nil || checkedOut != inst {
		t.Fatalf("Checkout = %v, %v", checkedOut, err)
	}
	if err := pool.Checkin(context.Background(), "lease", func(context.Context, *Instance) error { return nil }); err != nil {
		t.Fatalf("Checkin = %v", err)
	}
	idle, inUse := pool.Stats()
	if idle != 1 || inUse != 0 {
		t.Fatalf("Stats = (%d,%d), want (1,0)", idle, inUse)
	}
}

func TestManagerReconcileRemovesCleanupFailedInstance(t *testing.T) {
	lease := &testNetworkLease{}
	manager := NewManagerWithLimits(Config{}, network.Config{}, vsock.Config{}, SnapshotConfig{}, ManagerLimits{MaxVMs: 1, MaxConcurrentProvision: 1})
	inst := &Instance{ID: "vm", SocketPath: filepath.Join(t.TempDir(), "firecracker.sock"), network: lease, state: StateCleanupFailed}
	manager.instances[inst.ID] = inst
	if err := manager.Reconcile(context.Background()); err != nil {
		t.Fatalf("Reconcile = %v", err)
	}
	if !lease.released {
		t.Fatal("expected network lease to be released")
	}
	if _, err := manager.GetVM(inst.ID); err == nil {
		t.Fatal("cleanup-failed instance should be removed after successful reconciliation")
	}
}
