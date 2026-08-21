package network

import (
	"context"
	"errors"
	"testing"
)

func TestAllocationReleaseRetainsIPAMUntilCleanupSucceeds(t *testing.T) {
	pool := NewIPAMPool()
	base := DefaultConfig("test-tap")
	allocation, err := Acquire("vm-1", base, pool)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	attempts := 0
	allocation.cleanup = []func(context.Context) error{
		func(context.Context) error {
			attempts++
			if attempts == 1 {
				return errors.New("simulated cleanup failure")
			}
			return nil
		},
	}

	if err := allocation.Release(context.Background()); err == nil {
		t.Fatal("Release succeeded despite simulated cleanup failure")
	}

	if _, _, err := pool.Allocate("vm-2", base); err != nil {
		t.Fatalf("expected another allocation while failed lease remains owned: %v", err)
	}

	if err := allocation.Release(context.Background()); err != nil {
		t.Fatalf("retry Release: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("cleanup attempts = %d, want 2", attempts)
	}

	allocation2, err := Acquire("vm-3", base, pool)
	if err != nil {
		t.Fatalf("Acquire after successful release: %v", err)
	}
	if err := allocation2.Release(context.Background()); err != nil {
		t.Fatalf("release second allocation: %v", err)
	}
}
