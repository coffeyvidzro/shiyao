package daemon

import (
	"context"
	"encoding/json"
	"fmt"

	adapter "github.com/coffeyvidzro/shiyao/internal/adapters/nats"
	"github.com/coffeyvidzro/shiyao/internal/platform/sandbox"
)

type sandboxDispatcher struct {
	publisher adapter.Publisher
}

func newSandboxDispatcher(publisher adapter.Publisher) *sandboxDispatcher {
	return &sandboxDispatcher{publisher: publisher}
}

func (d *sandboxDispatcher) DispatchCreate(ctx context.Context, event sandbox.LifecycleEvent) error {
	return d.publish(ctx, adapter.SubjectSandboxCreate, event)
}

func (d *sandboxDispatcher) DispatchDestroy(ctx context.Context, event sandbox.LifecycleEvent) error {
	return d.publish(ctx, adapter.SubjectSandboxDestroy, event)
}

func (d *sandboxDispatcher) publish(ctx context.Context, subject string, event sandbox.LifecycleEvent) error {
	if d == nil || d.publisher == nil {
		return fmt.Errorf("sandbox lifecycle publisher is not configured")
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal sandbox lifecycle event: %w", err)
	}
	return d.publisher.Publish(ctx, subject, payload, nil, event.SandboxID.String()+":"+subject)
}
