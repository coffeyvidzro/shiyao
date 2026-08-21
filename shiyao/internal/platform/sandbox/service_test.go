package sandbox

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/coffeyvidzro/shiyao/internal/database/sqlc"
)

type fakeVMManager struct {
	provisionErr error
	destroyErr   error
	provisioned  []string
	destroyed    []string
}

func (f *fakeVMManager) ProvisionVM(_ context.Context, vmID string) error {
	f.provisioned = append(f.provisioned, vmID)
	return f.provisionErr
}

func (f *fakeVMManager) DestroyVM(_ context.Context, vmID string) error {
	f.destroyed = append(f.destroyed, vmID)
	return f.destroyErr
}

var _ VMManager = (*fakeVMManager)(nil)

func TestServiceCreateRequiresVMManager(t *testing.T) {
	service := NewService(&Repository{}, nil)
	_, err := service.Create(context.Background(), uuid.New(), CreateRequest{
		Template:       "default",
		VCPU:           1,
		MemoryMB:       128,
		TimeoutSeconds: 60,
	})
	if err == nil {
		t.Fatal("expected VM manager configuration error")
	}
}

func TestFakeVMManagerRecordsLifecycle(t *testing.T) {
	manager := &fakeVMManager{}
	vmID := "sbx-test"

	if err := manager.ProvisionVM(context.Background(), vmID); err != nil {
		t.Fatalf("provision: %v", err)
	}
	if err := manager.DestroyVM(context.Background(), vmID); err != nil {
		t.Fatalf("destroy: %v", err)
	}

	if len(manager.provisioned) != 1 || manager.provisioned[0] != vmID {
		t.Fatalf("unexpected provisioned VMs: %#v", manager.provisioned)
	}
	if len(manager.destroyed) != 1 || manager.destroyed[0] != vmID {
		t.Fatalf("unexpected destroyed VMs: %#v", manager.destroyed)
	}
}

func TestFakeVMManagerPropagatesErrors(t *testing.T) {
	want := errors.New("provision failed")
	manager := &fakeVMManager{provisionErr: want}
	if err := manager.ProvisionVM(context.Background(), "sbx-test"); !errors.Is(err, want) {
		t.Fatalf("expected %v, got %v", want, err)
	}
}

func TestSandboxResponseStatusValues(t *testing.T) {
	for _, status := range []string{"pending", "running", "failed", "stopped", "cleanup_failed"} {
		row := sqlc.Sandbox{Status: status}
		if row.Status != status {
			t.Fatalf("unexpected status %q", status)
		}
	}
}
