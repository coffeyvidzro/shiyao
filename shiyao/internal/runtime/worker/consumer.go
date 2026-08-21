package worker

import (
	"context"
	"fmt"

	adapter "github.com/coffeyvidzro/shiyao/internal/adapters/nats"
	"github.com/coffeyvidzro/shiyao/internal/platform/sandbox"
)

func (r *Registry) subscribe(_ context.Context) error {
	createSub, err := adapter.QueueSubscribeJSON(r.NATS, adapter.SubjectSandboxCreate, "shiyao-workers", r.handleSandboxCreate)
	if err != nil {
		return err
	}
	destroySub, err := adapter.QueueSubscribeJSON(r.NATS, adapter.SubjectSandboxDestroy, "shiyao-workers", r.handleSandboxDestroy)
	if err != nil {
		_ = createSub.Unsubscribe()
		return err
	}
	r.Subscriptions = []*adapter.Subscription{createSub, destroySub}
	return nil
}

func (r *Registry) handleSandboxCreate(ctx context.Context, event sandbox.LifecycleEvent) error {
	if _, err := r.Repository.UpdateStatus(ctx, event.SandboxID, "provisioning"); err != nil {
		return fmt.Errorf("mark sandbox provisioning: %w", err)
	}
	if _, err := r.VMM.ProvisionVM(ctx, event.VMID); err != nil {
		_, _ = r.Repository.UpdateStatus(ctx, event.SandboxID, "failed")
		return fmt.Errorf("provision sandbox vm: %w", err)
	}
	if _, err := r.Repository.UpdateStatus(ctx, event.SandboxID, "running"); err != nil {
		return fmt.Errorf("mark sandbox running: %w", err)
	}
	return nil
}

func (r *Registry) handleSandboxDestroy(ctx context.Context, event sandbox.LifecycleEvent) error {
	if err := r.VMM.DestroyVM(ctx, event.VMID); err != nil {
		_, _ = r.Repository.UpdateStatus(ctx, event.SandboxID, "cleanup_failed")
		return fmt.Errorf("destroy sandbox vm: %w", err)
	}
	if _, err := r.Repository.UpdateStatus(ctx, event.SandboxID, "stopped"); err != nil {
		return fmt.Errorf("mark sandbox stopped: %w", err)
	}
	if err := r.Repository.Delete(ctx, event.SandboxID); err != nil {
		return fmt.Errorf("delete sandbox row: %w", err)
	}
	return nil
}
