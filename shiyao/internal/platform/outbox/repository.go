package outbox

import "context"

type Repository interface {
	Claim(ctx context.Context, limit int, owner string) ([]Event, error)
	MarkPublished(ctx context.Context, id interface{}, owner string) error
	MarkRetry(ctx context.Context, id interface{}, owner string, availableAt interface{}, code, reason string) error
	Quarantine(ctx context.Context, id interface{}, owner, code, reason string) error
	Release(ctx context.Context, id interface{}, owner string) error
}
