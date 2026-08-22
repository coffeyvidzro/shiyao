package scheduler

import (
	"context"
	"testing"
	"time"
)

func TestPickLeastLoadedHealthyWorker(t *testing.T) {
	now := time.Now()
	s := New(15 * time.Second)
	worker, err := s.Pick(context.Background(), []Worker{
		{ID: "worker-a", Status: "active", MaxSlots: 4, UsedSlots: 3, LastHeartbeat: now},
		{ID: "worker-b", Status: "active", MaxSlots: 4, UsedSlots: 1, LastHeartbeat: now},
		{ID: "worker-c", Status: "draining", MaxSlots: 4, UsedSlots: 0, LastHeartbeat: now},
	})
	if err != nil {
		t.Fatal(err)
	}
	if worker.ID != "worker-b" {
		t.Fatalf("expected worker-b, got %s", worker.ID)
	}
}

func TestPickRejectsStaleWorker(t *testing.T) {
	_, err := New(time.Second).Pick(context.Background(), []Worker{{
		ID: "stale", Status: "active", MaxSlots: 1, LastHeartbeat: time.Now().Add(-2 * time.Second),
	}})
	if err != ErrNoCapacity {
		t.Fatalf("expected ErrNoCapacity, got %v", err)
	}
}

func TestPickHonorsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := New(time.Second).Pick(ctx, nil)
	if err != context.Canceled {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
