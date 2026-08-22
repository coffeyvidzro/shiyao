package observability

import (
	"sync/atomic"
	"time"
)

type Metrics struct {
	ProvisionStarted   atomic.Int64
	ProvisionSucceeded atomic.Int64
	ProvisionFailed    atomic.Int64
	ProvisionDuration  atomic.Int64
	JobsQueued         atomic.Int64
	JobsCompleted      atomic.Int64
	JobsFailed         atomic.Int64
	WorkerHeartbeats   atomic.Int64
}

func (m *Metrics) RecordProvision(success bool, duration time.Duration) {
	m.ProvisionStarted.Add(1)
	m.ProvisionDuration.Add(duration.Microseconds())
	if success {
		m.ProvisionSucceeded.Add(1)
	} else {
		m.ProvisionFailed.Add(1)
	}
}

func (m *Metrics) Snapshot() Snapshot {
	return Snapshot{
		ProvisionStarted: m.ProvisionStarted.Load(), ProvisionSucceeded: m.ProvisionSucceeded.Load(), ProvisionFailed: m.ProvisionFailed.Load(), ProvisionDurationMicros: m.ProvisionDuration.Load(), JobsQueued: m.JobsQueued.Load(), JobsCompleted: m.JobsCompleted.Load(), JobsFailed: m.JobsFailed.Load(), WorkerHeartbeats: m.WorkerHeartbeats.Load(),
	}
}

type Snapshot struct {
	ProvisionStarted       int64
	ProvisionSucceeded     int64
	ProvisionFailed        int64
	ProvisionDurationMicros int64
	JobsQueued             int64
	JobsCompleted          int64
	JobsFailed             int64
	WorkerHeartbeats       int64
}
