package outbox

import (
	"context"
	"errors"
	"time"
)

var (
	ErrRepositoryUnavailable = errors.New("outbox repository is not configured")
	ErrPublisherUnavailable  = errors.New("outbox publisher is not configured")
)

type Relay struct {
	repository *Repository
	publisher  Publisher
	owner      string
	batchSize  int
	lockAge    time.Duration
}

func NewRelay(repository *Repository, publisher Publisher, owner string) *Relay {
	return &Relay{repository: repository, publisher: publisher, owner: owner, batchSize: 100, lockAge: time.Minute}
}

func (r *Relay) RunOnce(ctx context.Context) error {
	if r.repository == nil {
		return ErrRepositoryUnavailable
	}
	if r.publisher == nil {
		return ErrPublisherUnavailable
	}

	events, err := r.repository.ClaimBatch(ctx, r.owner, r.batchSize, time.Now().Add(-r.lockAge))
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

func (r *Relay) processEvent(ctx context.Context, event Event) error {
	headers := make(map[string]string, len(event.Headers)+3)
	for key, value := range event.Headers {
		headers[key] = value
	}
	headers["Shiyao-Event-Id"] = event.ID.String()
	headers["Shiyao-Aggregate-Type"] = event.AggregateType
	headers["Shiyao-Aggregate-Id"] = event.AggregateID.String()

	publishErr := r.publisher.Publish(ctx, event.Subject, event.Payload, headers, event.ID.String())
	if publishErr == nil {
		return r.repository.MarkPublished(ctx, event.ID, r.owner)
	}

	class, code := classifyPublishError(publishErr)
	if class == publishErrorPermanent {
		return r.repository.Quarantine(ctx, event.ID, r.owner, code, publishErr.Error())
	}

	nextAttempt := time.Now().Add(retryDelay(event.PublishFailures))
	return r.repository.Release(ctx, event.ID, r.owner, nextAttempt, publishErr.Error())
}

func retryDelay(failures int) time.Duration {
	if failures < 0 {
		failures = 0
	}
	if failures > 10 {
		failures = 10
	}
	return time.Duration(1<<failures) * time.Second
}
