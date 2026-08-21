package vmm

import (
	"context"
	"errors"
	"testing"

	"github.com/coffeyvidzro/shiyao/internal/network"
	"github.com/coffeyvidzro/shiyao/internal/vsock"
)

type fakeNetworkLease struct {
	cfg          network.Config
	cid          uint32
	setupErr     error
	releaseErrs  []error
	releaseCalls int
}

func (f *fakeNetworkLease) Config() network.Config      { return f.cfg }
func (f *fakeNetworkLease) CID() uint32                 { return f.cid }
func (f *fakeNetworkLease) Setup(context.Context) error { return f.setupErr }
func (f *fakeNetworkLease) Release(context.Context) error {
	f.releaseCalls++
	if len(f.releaseErrs) == 0 {
		return nil
	}
	err := f.releaseErrs[0]
	f.releaseErrs = f.releaseErrs[1:]
	return err
}

func testInstanceConfig() Config {
	return Config{
		KernelPath:     "/kernel",
		RootfsPath:     "/rootfs",
		VCPUCount:      1,
		MemSizeMB:      128,
		BootArgs:       "console=ttyS0",
		GuestAgentPath: "/agent",
	}
}

func testNetworkConfig() network.Config {
	return network.DefaultConfig("test-tap")
}

func TestConfigureFailureReleasesNetworkAndReturnsToCreated(t *testing.T) {
	lease := &fakeNetworkLease{cfg: testNetworkConfig(), cid: 42}
	cfg := testInstanceConfig()
	inst := NewInstance("vm-1", "/tmp/test-vm.sock", cfg, lease, vsock.Config{GuestCID: 42}, SnapshotConfig{EnableResume: true})

	err := inst.Configure(context.Background())
	if err == nil {
		t.Fatal("Configure succeeded with incomplete snapshot configuration")
	}
	if inst.state != StateCreated {
		t.Fatalf("state = %s, want created", inst.state)
	}
	if lease.releaseCalls != 1 {
		t.Fatalf("release calls = %d, want 1", lease.releaseCalls)
	}
}

func TestConfigureFailureRetainsInstanceWhenCleanupFails(t *testing.T) {
	cleanupErr := errors.New("simulated cleanup failure")
	lease := &fakeNetworkLease{
		cfg:         testNetworkConfig(),
		cid:         42,
		releaseErrs: []error{cleanupErr, nil},
	}
	cfg := testInstanceConfig()
	inst := NewInstance("vm-2", "/tmp/test-vm.sock", cfg, lease, vsock.Config{GuestCID: 42}, SnapshotConfig{EnableResume: true})

	err := inst.Configure(context.Background())
	if err == nil || !errors.Is(err, cleanupErr) {
		t.Fatalf("Configure error = %v, want cleanup error", err)
	}
	if inst.state != StateCleanupFailed {
		t.Fatalf("state = %s, want cleanup-failed", inst.state)
	}
	if lease.releaseCalls != 1 {
		t.Fatalf("release calls = %d, want 1", lease.releaseCalls)
	}

	if err := inst.Stop(context.Background()); err != nil {
		t.Fatalf("Stop retry: %v", err)
	}
	if lease.releaseCalls != 2 {
		t.Fatalf("release calls after retry = %d, want 2", lease.releaseCalls)
	}
	if inst.state != StateStopped {
		t.Fatalf("state after retry = %s, want stopped", inst.state)
	}
}
