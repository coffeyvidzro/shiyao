package vmm

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/coffeyvidzro/shiyao/internal/network"
	"github.com/coffeyvidzro/shiyao/internal/vsock"
)

type blockingNetworkLease struct {
	cfg       network.Config
	cid       uint32
	setupOnce sync.Once
	setup     chan struct{}
	release   chan struct{}
}

func (l *blockingNetworkLease) Config() network.Config { return l.cfg }
func (l *blockingNetworkLease) CID() uint32            { return l.cid }

func (l *blockingNetworkLease) Setup(context.Context) error {
	l.setupOnce.Do(func() { close(l.setup) })
	<-l.release
	return errors.New("setup interrupted for test")
}

func (l *blockingNetworkLease) Release(context.Context) error { return nil }

func TestInstanceConfigureReservesLifecycleStateBeforeSlowSetup(t *testing.T) {
	lease := &blockingNetworkLease{
		cfg:     network.DefaultConfig("tap0"),
		cid:     3,
		setup:   make(chan struct{}),
		release: make(chan struct{}),
	}
	cfg := DefaultConfig()
	cfg.KernelPath = "/kernel"
	cfg.RootfsPath = "/rootfs"
	instance := NewInstance("vm1", "/tmp/vm1.sock", cfg, lease, vsock.Config{}, SnapshotConfig{})

	firstDone := make(chan error, 1)
	go func() { firstDone <- instance.Configure(context.Background()) }()
	<-lease.setup

	secondErr := instance.Configure(context.Background())
	if secondErr == nil || !strings.Contains(secondErr.Error(), "configuring") {
		t.Fatalf("expected concurrent Configure to be rejected while configuring, got %v", secondErr)
	}

	close(lease.release)
	if err := <-firstDone; err == nil {
		t.Fatal("expected first Configure to fail from test setup error")
	}
}
