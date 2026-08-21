package controlplane

import (
	"context"

	adapter "github.com/coffeyvidzro/shiyao/internal/adapters/nats"
	"github.com/coffeyvidzro/shiyao/internal/platform/sandbox"
)

type SandboxDispatcher struct {
	publisher *adapter.Publisher
}

func NewSandboxDispatcher(publisher *adapter.Publisher) *SandboxDispatcher {
	return &SandboxDispatcher{publisher: publisher}
}

func (d *SandboxDispatcher) DispatchCreate(ctx context.Context, event sandbox.LifecycleEvent) error {
	return d.publisher.PublishJSON(ctx, adapter.SubjectSandboxCreate, event)
}

func (d *SandboxDispatcher) DispatchDestroy(ctx context.Context, event sandbox.LifecycleEvent) error {
	return d.publisher.PublishJSON(ctx, adapter.SubjectSandboxDestroy, event)
}
