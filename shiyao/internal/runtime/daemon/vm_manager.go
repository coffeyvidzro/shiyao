package daemon

import (
	"context"

	"github.com/coffeyvidzro/shiyao/internal/platform/sandbox"
	"github.com/coffeyvidzro/shiyao/internal/vmm"
)

type vmManager struct {
	manager *vmm.Manager
}

var _ sandbox.VMManager = (*vmManager)(nil)

func newVMManager(manager *vmm.Manager) *vmManager {
	return &vmManager{manager: manager}
}

func (m *vmManager) ProvisionVM(ctx context.Context, vmID string) error {
	_, err := m.manager.ProvisionVM(ctx, vmID)
	return err
}

func (m *vmManager) DestroyVM(ctx context.Context, vmID string) error {
	return m.manager.DestroyVM(ctx, vmID)
}
