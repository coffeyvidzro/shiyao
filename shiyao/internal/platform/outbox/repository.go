package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository is the durable boundary for the outbox.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a PostgreSQL-backed outbox repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Enqueue(ctx context.Context, event Event) (uuid.UUID, error) {
	return enqueue(ctx, r.pool, event)
}

// EnqueueTx stores an outbox event in the caller's transaction. This is the
// operation application writes should use when the event must commit with the
// state change it describes.
func (r *Repository) EnqueueTx(ctx context.Context, tx pgx.Tx, event Event) (uuid.UUID, error) {
	if tx == nil {
		return uuid.Nil, errors.New("outbox transaction is required")
	}
	return enqueue(ctx, tx, event)
}

func (r *Repository) ClaimBatch(ctx context.Context, owner string, limit int, staleBefore time.Time) ([]Event, error) {
	if r == nil || r.pool == nil {
		return nil, errors.New("outbox repository is not configured")
	}
	owner = strings.TrimSpace(owner)
	if owner == "" {
		return nil, errors.New("outbox owner is required")
	}
	if limit <= 0 {
		return nil, errors.New("outbox claim limit must be positive")
	}
	if staleBefore.IsZero() {
		return nil, errors.New("outbox stale lock time is required")
	}

	rows, err := r.pool.Query(ctx, `
		WITH candidates AS (
			SELECT id
			FROM outbox_events
			WHERE published_at IS NULL
			  AND quarantined_at IS NULL
			  AND available_at <= now()
			  AND (locked_at IS NULL OR locked_at < $3)
			ORDER BY available_at ASC, created_at ASC
			FOR UPDATE SKIP LOCKED
			LIMIT $2
		)
		UPDATE outbox_events AS event
		SET locked_at = now(),
			locked_by = $1,
			attempts = event.attempts + 1,
			updated_at = now()
		FROM candidates
		WHERE event.id = candidates.id
		RETURNING
			event.id,
			event.subject,
			event.aggregate_type,
			event.aggregate_id,
			event.payload,
			event.headers,
			event.available_at,
			event.attempts,
			event.publish_failures,
			event.created_at
	`, owner, limit, staleBefore)
	if err != nil {
		return nil, fmt.Errorf("claim outbox events: %w", err)
	}
	defer rows.Close()

	events := make([]Event, 0, limit)
	for rows.Next() {
		var event Event
		var headersJSON []byte
		if err := rows.Scan(
			&event.ID,
			&event.Subject,
			&event.AggregateType,
			&event.AggregateID,
			&event.Payload,
			&headersJSON,
			&event.AvailableAt,
			&event.Attempts,
			&event.PublishFailures,
			&event.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan outbox event: %w", err)
		}
		if len(headersJSON) != 0 {
			if err := json.Unmarshal(headersJSON, &event.Headers); err != nil {
				return nil, fmt.Errorf("decode outbox headers for %s: %w", event.ID, err)
			}
		}
		if event.Headers == nil {
			event.Headers = map[string]string{}
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate outbox events: %w", err)
	}
	return events, nil
}

func (r *Repository) MarkPublished(ctx context.Context, eventID uuid.UUID, owner string) error {
	if r == nil || r.pool == nil {
		return errors.New("outbox repository is not configured")
	}
	result, err := r.pool.Exec(ctx, `
		UPDATE outbox_events
		SET published_at = now(),
			locked_at = NULL,
			locked_by = NULL,
			last_error = NULL,
			updated_at = now()
		WHERE id = $1
		  AND published_at IS NULL
		  AND quarantined_at IS NULL
		  AND locked_by = $2
	`, eventID, owner)
	if err != nil {
		return fmt.Errorf("mark outbox event %s published: %w", eventID, err)
	}
	if result.RowsAffected() != 1 {
		return ErrClaimLost
	}
	return nil
}

// Release makes a claimed event eligible again after a publish failure.
func (r *Repository) Release(ctx context.Context, eventID uuid.UUID, owner string, availableAt time.Time, lastError string) error {
	if r == nil || r.pool == nil {
		return errors.New("outbox repository is not configured")
	}
	result, err := r.pool.Exec(ctx, `
		UPDATE outbox_events
		SET available_at = $3,
			publish_failures = publish_failures + 1,
			locked_at = NULL,
			locked_by = NULL,
			last_error = $4,
			updated_at = now()
		WHERE id = $1
		  AND published_at IS NULL
		  AND quarantined_at IS NULL
		  AND locked_by = $2
	`, eventID, owner, availableAt, lastError)
	if err != nil {
		return fmt.Errorf("release outbox event %s: %w", eventID, err)
	}
	if result.RowsAffected() != 1 {
		return ErrClaimLost
	}
	return nil
}

func (r *Repository) Quarantine(ctx context.Context, eventID uuid.UUID, owner, code, reason string) error {
	if r == nil || r.pool == nil {
		return errors.New("outbox repository is not configured")
	}
	code = strings.TrimSpace(code)
	reason = strings.TrimSpace(reason)
	if code == "" || reason == "" {
		return errors.New("outbox quarantine code and reason are required")
	}

	result, err := r.pool.Exec(ctx, `
		UPDATE outbox_events
		SET quarantined_at = now(),
			quarantine_code = $3,
			quarantine_reason = $4,
			publish_failures = publish_failures + 1,
			locked_at = NULL,
			locked_by = NULL,
			last_error = $4,
			updated_at = now()
		WHERE id = $1
		  AND published_at IS NULL
		  AND quarantined_at IS NULL
		  AND locked_by = $2
	`, eventID, owner, code, reason)
	if err != nil {
		return fmt.Errorf("quarantine outbox event %s: %w", eventID, err)
	}
	if result.RowsAffected() != 1 {
		return ErrClaimLost
	}
	return nil
}

func (r *Repository) Redrive(ctx context.Context, eventID uuid.UUID, availableAt time.Time) error {
	if r == nil || r.pool == nil {
		return errors.New("outbox repository is not configured")
	}
	result, err := r.pool.Exec(ctx, `
		UPDATE outbox_events
		SET quarantined_at = NULL,
			quarantine_code = NULL,
			quarantine_reason = NULL,
			available_at = $2,
			redrive_count = redrive_count + 1,
			updated_at = now()
		WHERE id = $1
		  AND published_at IS NULL
		  AND quarantined_at IS NOT NULL
	`, eventID, availableAt)
	if err != nil {
		return fmt.Errorf("redrive outbox event %s: %w", eventID, err)
	}
	if result.RowsAffected() != 1 {
		return ErrNotQuarantined
	}
	return nil
}

func enqueue(ctx context.Context, executor interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}, event Event) (uuid.UUID, error) {
	if executor == nil {
		return uuid.Nil, errors.New("outbox database executor is not configured")
	}
	if event.ID == uuid.Nil {
		event.ID = uuid.New()
	}
	event.Subject = strings.TrimSpace(event.Subject)
	event.AggregateType = strings.TrimSpace(event.AggregateType)
	if event.Subject == "" {
		return uuid.Nil, errors.New("outbox subject is required")
	}
	if event.AggregateType == "" {
		return uuid.Nil, errors.New("outbox aggregate type is required")
	}
	if event.AggregateID == uuid.Nil {
		return uuid.Nil, errors.New("outbox aggregate ID is required")
	}
	if event.Payload == nil || !json.Valid(event.Payload) {
		return uuid.Nil, errors.New("outbox payload must be valid JSON")
	}
	if event.Headers == nil {
		event.Headers = map[string]string{}
	}
	availableAt := event.AvailableAt
	if availableAt.IsZero() {
		availableAt = time.Now().UTC()
	}
	headers, err := json.Marshal(event.Headers)
	if err != nil {
		return uuid.Nil, fmt.Errorf("encode outbox headers: %w", err)
	}

	_, err = executor.Exec(ctx, `
		INSERT INTO outbox_events (
			id, subject, aggregate_type, aggregate_id, payload, headers, available_at
		) VALUES ($1, $2, $3, $4, $5::jsonb, $6::jsonb, $7)
	`, event.ID, event.Subject, event.AggregateType, event.AggregateID, event.Payload, headers, availableAt)
	if err != nil {
		return uuid.Nil, fmt.Errorf("enqueue outbox event: %w", err)
	}
	return event.ID, nil
}
