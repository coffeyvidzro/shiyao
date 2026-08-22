package vmm

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

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

func (p *WarmPool) Add(inst *Instance) error {
	if inst == nil {
		return fmt.Errorf("warm instance is required")
	}
	if inst.State() != StateRunning {
		return fmt.Errorf("warm instance must be running, got %s", inst.State())
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
	if inst.State() != StateRunning {
		p.idle = append(p.idle, inst)
		return nil, fmt.Errorf("warm instance %s is not running", inst.ID)
	}
	p.inUse[leaseID] = inst
	return inst, nil
}

// Checkin runs reset before making an instance available to another tenant.
// A successful reset must leave the instance running; otherwise the instance
// is evicted and stopped rather than risking state reuse.
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
		stopErr := inst.Stop(ctx)
		return errors.Join(fmt.Errorf("reset warm instance: %w", err), stopErr)
	}
	if state := inst.State(); state != StateRunning {
		stopErr := inst.Stop(ctx)
		return errors.Join(fmt.Errorf("warm reset completed in non-running state %s", state), stopErr)
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
