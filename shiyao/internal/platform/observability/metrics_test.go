package observability

import (
	"testing"
	"time"
)

func TestMetricsRecordProvision(t *testing.T) {
	var m Metrics
	m.RecordProvision(true, 10*time.Millisecond)
	m.RecordProvision(false, 5*time.Millisecond)

	s := m.Snapshot()
	if s.ProvisionStarted != 2 || s.ProvisionSucceeded != 1 || s.ProvisionFailed != 1 {
		t.Fatalf("unexpected counters: %+v", s)
	}
	if s.ProvisionDurationMicros != 15_000 {
		t.Fatalf("unexpected duration: %d", s.ProvisionDurationMicros)
	}
}
