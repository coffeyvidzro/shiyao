package vmm

import (
	"context"
	"testing"
)

func TestMeasureBoots(t *testing.T) {
	got, err := MeasureBoots(context.Background(), func(context.Context) error { return nil }, func(context.Context) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if got.ColdBoot < 0 || got.SnapshotResume < 0 {
		t.Fatalf("invalid measurements: %+v", got)
	}
}
