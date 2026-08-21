package nats

import (
	"context"
	"fmt"
	"time"

	natsjs "github.com/nats-io/nats.go/jetstream"
)

const maxMessageSize = 64 * 1024

type StreamLimits struct {
	JobsMaxBytes int64
	DLQMaxBytes  int64
	JobsMaxAge   time.Duration
	DLQMaxAge    time.Duration
	Replicas     int
}

func DefaultStreamLimits() StreamLimits {
	return StreamLimits{
		JobsMaxBytes: 5 * 1024 * 1024 * 1024,
		DLQMaxBytes:  5 * 1024 * 1024 * 1024,
		JobsMaxAge:   7 * 24 * time.Hour,
		DLQMaxAge:    90 * 24 * time.Hour,
		Replicas:     1,
	}
}

func (c *Client) Provision(ctx context.Context, limits StreamLimits) error {
	if ctx == nil {
		return fmt.Errorf("JetStream provisioning context is required")
	}
	if c == nil || c.jetStream == nil {
		return ErrClientUnavailable
	}
	for _, config := range StreamConfigs(limits) {
		if _, err := c.jetStream.CreateOrUpdateStream(ctx, config); err != nil {
			return fmt.Errorf("provision JetStream stream %s: %w", config.Name, err)
		}
	}
	return nil
}

func StreamConfigs(limits StreamLimits) []natsjs.StreamConfig {
	limits = normalizeStreamLimits(limits)
	return []natsjs.StreamConfig{
		{
			Name:        JobsStreamName,
			Description: "Durable Shiyao sandbox lifecycle jobs",
			Subjects:    []string{JobsSubject},
			Retention:   natsjs.WorkQueuePolicy,
			Discard:     natsjs.DiscardNew,
			Storage:     natsjs.FileStorage,
			Replicas:    limits.Replicas,
			MaxBytes:    limits.JobsMaxBytes,
			MaxAge:      limits.JobsMaxAge,
			MaxMsgSize:  maxMessageSize,
			Duplicates:  10 * time.Minute,
		},
		{
			Name:        DLQStreamName,
			Description: "Shiyao jobs requiring operator inspection or redrive",
			Subjects:    []string{DLQSubject},
			Retention:   natsjs.LimitsPolicy,
			Discard:     natsjs.DiscardOld,
			Storage:     natsjs.FileStorage,
			Replicas:    limits.Replicas,
			MaxBytes:    limits.DLQMaxBytes,
			MaxAge:      limits.DLQMaxAge,
			MaxMsgSize:  maxMessageSize,
			Duplicates:  10 * time.Minute,
		},
	}
}

func normalizeStreamLimits(limits StreamLimits) StreamLimits {
	defaults := DefaultStreamLimits()
	if limits.JobsMaxBytes <= 0 {
		limits.JobsMaxBytes = defaults.JobsMaxBytes
	}
	if limits.DLQMaxBytes <= 0 {
		limits.DLQMaxBytes = defaults.DLQMaxBytes
	}
	if limits.JobsMaxAge <= 0 {
		limits.JobsMaxAge = defaults.JobsMaxAge
	}
	if limits.DLQMaxAge <= 0 {
		limits.DLQMaxAge = defaults.DLQMaxAge
	}
	if limits.Replicas <= 0 {
		limits.Replicas = defaults.Replicas
	}
	return limits
}
