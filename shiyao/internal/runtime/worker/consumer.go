package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	adapter "github.com/coffeyvidzro/shiyao/internal/adapters/nats"
	"github.com/coffeyvidzro/shiyao/internal/platform/sandbox"
	natsjs "github.com/nats-io/nats.go/jetstream"
)

const (
	sandboxCreateConsumer  = "shiyao-sandbox-create-worker"
	sandboxDestroyConsumer = "shiyao-sandbox-destroy-worker"
	consumerAckWait        = 2 * time.Minute
	consumerMaxDeliver     = 5
)

func (r *Registry) subscribe(ctx context.Context) error {
	if err := r.NATS.Provision(ctx, adapter.DefaultStreamLimits()); err != nil {
		return err
	}

	createHandle, err := r.consumeSandboxLifecycle(ctx, sandboxCreateConsumer, adapter.SubjectSandboxCreate, r.handleSandboxCreate)
	if err != nil {
		return err
	}
	destroyHandle, err := r.consumeSandboxLifecycle(ctx, sandboxDestroyConsumer, adapter.SubjectSandboxDestroy, r.handleSandboxDestroy)
	if err != nil {
		createHandle.Stop()
		return err
	}
	r.Consumers = []adapter.ConsumerHandle{createHandle, destroyHandle}
	return nil
}

func (r *Registry) consumeSandboxLifecycle(
	ctx context.Context,
	durable string,
	subject string,
	handler func(context.Context, sandbox.LifecycleEvent) error,
) (adapter.ConsumerHandle, error) {
	consumer, err := r.NATS.CreateOrUpdateConsumer(ctx, adapter.JobsStreamName, natsjs.ConsumerConfig{
		Durable:       durable,
		Name:          durable,
		FilterSubject: subject,
		AckPolicy:     natsjs.AckExplicitPolicy,
		AckWait:       consumerAckWait,
		MaxDeliver:    consumerMaxDeliver,
	})
	if err != nil {
		return nil, err
	}

	consumeContext, err := consumer.Consume(func(msg natsjs.Msg) {
		var event sandbox.LifecycleEvent
		if err := json.Unmarshal(msg.Data(), &event); err != nil {
			_ = msg.Term()
			return
		}
		if err := handler(context.Background(), event); err != nil {
			_ = msg.Nak()
			return
		}
		_ = msg.Ack()
	})
	if err != nil {
		return nil, fmt.Errorf("consume %s: %w", subject, err)
	}
	return consumeContext, nil
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
