package scheduler

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"
)

var ErrNoCapacity = errors.New("no schedulable worker capacity")

type Worker struct {
	ID            string
	Status        string
	MaxSlots      int
	UsedSlots     int
	LastHeartbeat time.Time
}

func (w Worker) Available(now time.Time, heartbeatTimeout time.Duration) bool {
	return w.Status == "active" && w.UsedSlots < w.MaxSlots && now.Sub(w.LastHeartbeat) <= heartbeatTimeout
}

type Scheduler struct {
	mu              sync.Mutex
	heartbeatTimeout time.Duration
}

func New(heartbeatTimeout time.Duration) *Scheduler {
	if heartbeatTimeout <= 0 {
		heartbeatTimeout = 15 * time.Second
	}
	return &Scheduler{heartbeatTimeout: heartbeatTimeout}
}

// Pick chooses the least-loaded healthy worker. Ties are resolved by worker ID
// to make placement deterministic and tests reproducible.
func (s *Scheduler) Pick(ctx context.Context, workers []Worker) (Worker, error) {
	if err := ctx.Err(); err != nil {
		return Worker{}, err
	}
	now := time.Now()
	candidates := make([]Worker, 0, len(workers))
	for _, worker := range workers {
		if worker.Available(now, s.heartbeatTimeout) {
			candidates = append(candidates, worker)
		}
	}
	if len(candidates) == 0 {
		return Worker{}, ErrNoCapacity
	}
	sort.Slice(candidates, func(i, j int) bool {
		left, right := candidates[i], candidates[j]
		leftLoad := float64(left.UsedSlots) / float64(left.MaxSlots)
		rightLoad := float64(right.UsedSlots) / float64(right.MaxSlots)
		if leftLoad == rightLoad {
			return left.ID < right.ID
		}
		return leftLoad < rightLoad
	})
	return candidates[0], nil
}
