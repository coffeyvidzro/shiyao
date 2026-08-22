package outbox

import (
	"context"
	"time"
)

type Relay struct {
	Repository Repository
	Publisher  Publisher
	BatchSize  int
	Owner      string
}

func (r Relay) RunOnce(ctx context.Context) error {
	batchSize := r.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}
	events, err := r.Repository.Claim(ctx, batchSize, r.Owner)
	if err != nil {
		return err
	}
	for _, event := range events {
		if err := r.publish(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

func (r Relay) publish(ctx context.Context, event Event) error {
	err := r.Publisher.Publish(ctx, event.Subject, event.Payload, event.Headers, event.ID.String())
	if err == nil {
		return r.Repository.MarkPublished(ctx, event.ID, r.Owner)
	}

	class, code := classifyPublishError(err)
	switch class {
	case publishErrorPermanent:
		return r.Repository.Quarantine(ctx, event.ID, r.Owner, code, err.Error())
	case publishErrorRetryable, publishErrorUnknown:
		return r.Repository.MarkRetry(ctx, event.ID, r.Owner, time.Now().Add(backoff(event.PublishFailures)), code, err.Error())
	default:
		return err
	}
}

func backoff(failures int) time.Duration {
	if failures < 0 {
		failures = 0
	}
	if failures > 10 {
		failures = 10
	}
	return time.Duration(1<<failures) * time.Second
}
