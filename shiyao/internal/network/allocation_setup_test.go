package network

import (
	"context"
	"errors"
	"testing"
)

func TestAllocationReleaseRetainsFailedCleanupForRetry(t *testing.T) {
	allocation := &Allocation{
		pool: NewIPAMPool(),
		cfg:  DefaultConfig("test-tap"),
		cleanup: []func(context.Context) error{
			func(context.Context) error { return errors.New("simulated cleanup failure") },
		},
	}

	if err := allocation.Release(context.Background()); err == nil {
		t.Fatal("Release succeeded despite simulated cleanup failure")
	}
	if allocation.released {
		t.Fatal("allocation marked released after cleanup failure")
	}
	if len(allocation.cleanup) != 1 {
		t.Fatalf("cleanup entries = %d, want 1 after failed cleanup", len(allocation.cleanup))
	}
}

func TestAllocationReleaseRunsCleanupInReverseOrder(t *testing.T) {
	var order []int
	allocation := &Allocation{
		pool: NewIPAMPool(),
		cfg:  DefaultConfig("test-tap"),
		cleanup: []func(context.Context) error{
			func(context.Context) error {
				order = append(order, 1)
				return nil
			},
			func(context.Context) error {
				order = append(order, 2)
				return nil
			},
		},
	}

	if err := allocation.Release(context.Background()); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if len(order) != 2 || order[0] != 2 || order[1] != 1 {
		t.Fatalf("cleanup order = %v, want [2 1]", order)
	}
	if !allocation.released {
		t.Fatal("allocation was not marked released after successful cleanup")
	}
}
