package sandbox

import (
	"context"

	"github.com/google/uuid"
)

type LifecycleController interface {
	ProvisionVM(context.Context, string) error
	DestroyVM(context.Context, string) error
}

type LifecycleDispatcher interface {
	DispatchCreate(context.Context, LifecycleEvent) error
	DispatchDestroy(context.Context, LifecycleEvent) error
}

type LifecycleEvent struct {
	SandboxID uuid.UUID `json:"sandbox_id"`
	UserID    uuid.UUID `json:"user_id"`
	VMID      string    `json:"vm_id"`
}

type localLifecycleDispatcher struct {
	controller LifecycleController
}

func NewLocalLifecycleDispatcher(controller LifecycleController) LifecycleDispatcher {
	if controller == nil {
		return nil
	}
	return &localLifecycleDispatcher{controller: controller}
}

func (d *localLifecycleDispatcher) DispatchCreate(ctx context.Context, event LifecycleEvent) error {
	return d.controller.ProvisionVM(ctx, event.VMID)
}

func (d *localLifecycleDispatcher) DispatchDestroy(ctx context.Context, event LifecycleEvent) error {
	return d.controller.DestroyVM(ctx, event.VMID)
}
