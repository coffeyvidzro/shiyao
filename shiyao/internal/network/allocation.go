package network

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Allocation owns the host-side network resources assigned to one VM.
// The IPAM lease is returned only after all configured host resources have
// been cleaned up successfully.
type Allocation struct {
	mu       sync.Mutex
	pool     *IPAMPool
	cfg      Config
	cid      uint32
	cleanup  []func(context.Context) error
	released bool
}

func Acquire(vmID string, base Config, pool *IPAMPool) (*Allocation, error) {
	if pool == nil {
		return nil, errors.New("network: nil IPAM pool")
	}
	cfg, cid, err := pool.Allocate(vmID, base)
	if err != nil {
		return nil, err
	}
	return &Allocation{pool: pool, cfg: cfg, cid: cid}, nil
}

func (a *Allocation) Config() Config {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cfg
}

func (a *Allocation) CID() uint32 {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cid
}

// Setup creates TAP and firewall resources and records their cleanup in the
// allocation. A failed setup remains owned by the allocation so Release can
// retry cleanup without prematurely returning the IPAM resources.
func (a *Allocation) Setup(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.released {
		return errors.New("network: allocation already released")
	}
	if len(a.cleanup) != 0 {
		return errors.New("network: allocation already set up")
	}

	if err := SetupTAP(ctx, a.cfg); err != nil {
		return fmt.Errorf("setup tap: %w", err)
	}
	a.cleanup = append(a.cleanup, func(ctx context.Context) error {
		return CleanupTAP(ctx, a.cfg.TapName)
	})

	if err := SetupFirewall(ctx, a.cfg); err != nil {
		return fmt.Errorf("setup firewall: %w", err)
	}
	a.cleanup = append(a.cleanup, func(ctx context.Context) error {
		return CleanupFirewall(ctx, a.cfg)
	})
	return nil
}

// Release tears down all owned host resources and returns the IPAM lease only
// when cleanup succeeds. Failed cleanup steps remain owned for retry.
func (a *Allocation) Release(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.released {
		return nil
	}

	var errs []error
	remaining := make([]func(context.Context) error, 0, len(a.cleanup))
	for j := len(a.cleanup) - 1; j >= 0; j-- {
		if err := a.cleanup[j](ctx); err != nil {
			errs = append(errs, fmt.Errorf("cleanup step %d: %w", j, err))
			remaining = append(remaining, a.cleanup[j])
		}
	}
	a.cleanup = remaining
	if len(errs) != 0 {
		return errors.Join(errs...)
	}

	a.pool.Release(a.cfg.GuestIP, a.cid)
	a.released = true
	return nil
}
