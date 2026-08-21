package vmm

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/coffeyvidzro/shiyao/internal/network"
	"github.com/coffeyvidzro/shiyao/internal/vsock"
)

type stopTestLease struct {
	mu       sync.Mutex
	releases int
	err      error
}

func (l *stopTestLease) Config() network.Config { return network.DefaultConfig("tap0") }
func (l *stopTestLease) CID() uint32            { return 42 }
func (l *stopTestLease) Setup(context.Context) error { return nil }
func (l *stopTestLease) Release(context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.releases++
	return l.err
}

func (l *stopTestLease) ReleaseCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.releases
}

func TestInstanceStopConcurrentCallsOnlyReleaseNetworkOnce(t *testing.T) {
	lease := &stopTestLease{}
	instance := NewInstance("vm1", t.TempDir()+"/firecracker.sock", Config{}, lease, vsock.Config{}, SnapshotConfig{})
	instance.state = StateConfigured

	start := make(chan struct{})
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if err := instance.Stop(context.Background()); err != nil {
				t.Errorf("Stop() error = %v", err)
			}
		}()
	}
	close(start)
	wg.Wait()

	if got := lease.ReleaseCount(); got != 1 {
		t.Fatalf("network Release called %d times, want 1", got)
	}
	if instance.state != StateStopped {
		t.Fatalf("state = %s, want stopped", instance.state)
	}
}

func TestInstanceStopCleanupFailureRemainsRetryable(t *testing.T) {
	lease := &stopTestLease{err: errors.New("release failed")}
	instance := NewInstance("vm1", t.TempDir()+"/firecracker.sock", Config{}, lease, vsock.Config{}, SnapshotConfig{})
	instance.state = StateConfigured

	if err := instance.Stop(context.Background()); err == nil {
		t.Fatal("Stop() succeeded, want cleanup failure")
	}
	if instance.state != StateCleanupFailed {
		t.Fatalf("state = %s, want cleanup-failed", instance.state)
	}

	lease.mu.Lock()
	lease.err = nil
	lease.mu.Unlock()

	if err := instance.Stop(context.Background()); err != nil {
		t.Fatalf("retry Stop() error = %v", err)
	}
	if instance.state != StateStopped {
		t.Fatalf("state after retry = %s, want stopped", instance.state)
	}
	if got := lease.ReleaseCount(); got != 2 {
		t.Fatalf("network Release called %d times, want 2", got)
	}
}
