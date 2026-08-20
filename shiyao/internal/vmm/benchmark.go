package vmm

import (
	"context"
	"fmt"
	"time"
)

// BootMeasurements captures comparable cold-boot and snapshot-resume latency.
type BootMeasurements struct {
	ColdBoot       time.Duration
	SnapshotResume time.Duration
}

// MeasureBoots runs one cold boot and one snapshot resume using the supplied
// lifecycle operations. Callers can use it from integration benchmarks without
// embedding host paths or KVM assumptions in the vmm package.
func MeasureBoots(ctx context.Context, coldBoot, snapshotResume func(context.Context) error) (BootMeasurements, error) {
	if coldBoot == nil || snapshotResume == nil {
		return BootMeasurements{}, fmt.Errorf("cold boot and snapshot resume functions are required")
	}
	start := time.Now()
	if err := coldBoot(ctx); err != nil {
		return BootMeasurements{}, fmt.Errorf("cold boot: %w", err)
	}
	cold := time.Since(start)
	start = time.Now()
	if err := snapshotResume(ctx); err != nil {
		return BootMeasurements{}, fmt.Errorf("snapshot resume: %w", err)
	}
	return BootMeasurements{ColdBoot: cold, SnapshotResume: time.Since(start)}, nil
}
