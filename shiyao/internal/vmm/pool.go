package vmm

import (
	"context"
	"fmt"
	"sync"
)

// WarmPool keeps already-started instances available for checkout. A caller
// supplies the reset operation so the pool never returns an instance with a
// previous tenant's state.
type WarmPool struct {
	mu       sync.Mutex
	capacity int
	idle     []*Instance
	inUse    map[string]*Instance
}

func NewWarmPool(capacity int) (*WarmPool, error) {
	if capacity <= 0 {
		return nil, fmt.Errorf("warm pool capacity must be greater than zero")
	}
	return &WarmPool{capacity: capacity, inUse: make(map[string]*Instance)}, nil
}

// Add registers a running, clean instance as available. It never starts a VM;
// callers can fill the pool using Manager.ProvisionVM under its admission gate.
func (p *WarmPool) Add(inst *Instance) error {
	if inst == nil {
		return fmt.Errorf("warm instance is required")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.idle)+len(p.inUse) >= p.capacity {
		return fmt.Errorf("%w: warm pool capacity %d", ErrBackpressure, p.capacity)
	}
	p.idle = append(p.idle, inst)
	return nil
}

func (p *WarmPool) Checkout(leaseID string) (*Instance, error) {
	if leaseID == "" {
		return nil, fmt.Errorf("lease ID is required")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, exists := p.inUse[leaseID]; exists {
		return nil, fmt.Errorf("lease %s already exists", leaseID)
	}
	if len(p.idle) == 0 {
		return nil, fmt.Errorf("%w: no warm instances available", ErrBackpressure)
	}
	last := len(p.idle) - 1
	inst := p.idle[last]
	p.idle = p.idle[:last]
	p.inUse[leaseID] = inst
	return inst, nil
}

// Checkin runs reset before making an instance available to another tenant.
// Failed resets evict and stop the instance rather than risking state reuse.
func (p *WarmPool) Checkin(ctx context.Context, leaseID string, reset func(context.Context, *Instance) error) error {
	if reset == nil {
		return fmt.Errorf("instance reset function is required")
	}
	p.mu.Lock()
	inst, ok := p.inUse[leaseID]
	if ok {
		delete(p.inUse, leaseID)
	}
	p.mu.Unlock()
	if !ok {
		return fmt.Errorf("lease %s not found", leaseID)
	}
	if err := reset(ctx, inst); err != nil {
		_ = inst.Stop(ctx)
		return fmt.Errorf("reset warm instance: %w", err)
	}
	p.mu.Lock()
	p.idle = append(p.idle, inst)
	p.mu.Unlock()
	return nil
}

func (p *WarmPool) Stats() (idle, inUse int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.idle), len(p.inUse)
}
