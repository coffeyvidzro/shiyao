package vmm

import (
	"context"
	"errors"
	"testing"
)

func TestWarmPoolCheckoutBackpressure(t *testing.T) {
	p, err := NewWarmPool(1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Checkout("lease"); !errors.Is(err, ErrBackpressure) {
		t.Fatalf("Checkout error = %v", err)
	}
}

func TestWarmPoolRejectsInvalidCapacity(t *testing.T) {
	if _, err := NewWarmPool(0); err == nil {
		t.Fatal("expected capacity error")
	}
}

func TestWarmPoolCheckinRequiresKnownLease(t *testing.T) {
	p, _ := NewWarmPool(1)
	if err := p.Checkin(context.Background(), "missing", func(context.Context, *Instance) error { return nil }); err == nil {
		t.Fatal("expected lease error")
	}
}
