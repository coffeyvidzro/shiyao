package vmm

import (
	"errors"
	"testing"

	"github.com/coffeyvidzro/shiyao/internal/core/network"
	"github.com/coffeyvidzro/shiyao/internal/core/vsock"
)

func TestManagerRejectsVMsOverResidentLimit(t *testing.T) {
	manager := NewManagerWithLimits(Config{}, network.Config{UplinkInterface: "eth0"}, vsock.Config{}, SnapshotConfig{}, ManagerLimits{MaxVMs: 1, MaxConcurrentProvision: 1})
	if _, err := manager.CreateVM("first"); err != nil {
		t.Fatalf("create first VM: %v", err)
	}
	if _, err := manager.CreateVM("second"); !errors.Is(err, ErrBackpressure) {
		t.Fatalf("CreateVM error = %v, want ErrBackpressure", err)
	}
}

func TestProvisionVMRejectsWhenAdmissionGateIsFull(t *testing.T) {
	manager := NewManagerWithLimits(Config{}, network.Config{}, vsock.Config{}, SnapshotConfig{}, ManagerLimits{MaxVMs: 1, MaxConcurrentProvision: 1})
	manager.provision <- struct{}{}
	defer func() { <-manager.provision }()
	if _, err := manager.ProvisionVM(t.Context(), "blocked"); !errors.Is(err, ErrBackpressure) {
		t.Fatalf("ProvisionVM error = %v, want ErrBackpressure", err)
	}
}
