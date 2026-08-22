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
	events, err := r.Repository.Claim(ctx, batchSize, r.Owner, time.Minute)
	if err != nil {
		return err
	}
	for _, event := range events {
		if err := r.processEvent(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

func (r Relay) processEvent(ctx context.Context, event Event) error {
	headers := make(map[string]string, len(event.Headers)+3)
	for key, value := range event.Headers {
		headers[key] = value
	}
	headers["Shiyao-Event-Id"] = event.ID.String()
	headers["Shiyao-Aggregate-Type"] = event.AggregateType
	headers["Shiyao-Aggregate-Id"] = event.AggregateID.String()

	publishErr := r.Publisher.Publish(ctx, event.Subject, event.Payload, headers, event.ID.String())
	if publishErr == nil {
		return r.Repository.MarkPublished(ctx, event.ID, r.Owner)
	}

	class, code := classifyPublishError(publishErr)
	switch class {
	case publishErrorPermanent:
		return r.Repository.Quarantine(ctx, event.ID, r.Owner, code, publishErr.Error())
	case publishErrorRetryable, publishErrorUnknown:
		return r.Repository.MarkRetry(ctx, event.ID, r.Owner, time.Now().Add(backoff(event.PublishFailures)), code, publishErr.Error())
	default:
		return publishErr
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
