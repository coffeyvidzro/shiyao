package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository provides durable outbox operations. Claim uses a lease so a
// crashed relay does not permanently own an event.
type Repository interface {
	Insert(ctx context.Context, event Event) error
	Claim(ctx context.Context, limit int, owner string, lease time.Duration) ([]Event, error)
	MarkPublished(ctx context.Context, id uuid.UUID, owner string) error
	MarkRetry(ctx context.Context, id uuid.UUID, owner string, availableAt time.Time, code, reason string) error
	Quarantine(ctx context.Context, id uuid.UUID, owner, code, reason string) error
	Redrive(ctx context.Context, id uuid.UUID, availableAt time.Time) error
	Release(ctx context.Context, id uuid.UUID, owner string) error
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) (*PostgresRepository, error) {
	if pool == nil {
		return nil, fmt.Errorf("outbox postgres pool is required")
	}
	return &PostgresRepository{pool: pool}, nil
}

func (r *PostgresRepository) Insert(ctx context.Context, event Event) error {
	if event.ID == uuid.Nil {
		event.ID = uuid.New()
	}
	if strings.TrimSpace(event.Subject) == "" {
		return fmt.Errorf("outbox subject is required")
	}
	if strings.TrimSpace(event.AggregateType) == "" {
		return fmt.Errorf("outbox aggregate type is required")
	}
	if event.AggregateID == uuid.Nil {
		return fmt.Errorf("outbox aggregate ID is required")
	}
	if event.Payload == nil {
		return fmt.Errorf("outbox payload is required")
	}
	if !json.Valid(event.Payload) {
		return fmt.Errorf("outbox payload must be valid JSON")
	}

	headers := event.Headers
	if headers == nil {
		headers = map[string]string{}
	}
	availableAt := event.AvailableAt
	if availableAt.IsZero() {
		availableAt = time.Now().UTC()
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO outbox_events (
			id, subject, aggregate_type, aggregate_id, payload, headers,
			available_at, attempts, publish_failures, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5::jsonb, $6::jsonb, $7, 0, 0, now(), now())
	`, event.ID, event.Subject, event.AggregateType, event.AggregateID, event.Payload, headersJSON(headers), availableAt)
	if err != nil {
		return fmt.Errorf("insert outbox event: %w", err)
	}
	return nil
}

func (r *PostgresRepository) Claim(ctx context.Context, limit int, owner string, lease time.Duration) ([]Event, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("outbox claim limit must be greater than zero")
	}
	if strings.TrimSpace(owner) == "" {
		return nil, fmt.Errorf("outbox claim owner is required")
	}
	if lease <= 0 {
		return nil, fmt.Errorf("outbox claim lease must be greater than zero")
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("begin outbox claim transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, `
		WITH candidates AS (
			SELECT id
			FROM outbox_events
			WHERE published_at IS NULL
			  AND quarantined_at IS NULL
			  AND available_at <= now()
			  AND (locked_at IS NULL OR locked_at <= now())
			ORDER BY available_at ASC, created_at ASC
			FOR UPDATE SKIP LOCKED
			LIMIT $1
		), claimed AS (
			UPDATE outbox_events o
			SET locked_at = now() + $2::interval,
			    locked_by = $3,
			    attempts = o.attempts + 1,
			    updated_at = now()
			FROM candidates c
			WHERE o.id = c.id
			RETURNING o.id, o.subject, o.aggregate_type, o.aggregate_id,
			          o.payload, o.headers, o.available_at, o.attempts,
			          o.publish_failures, o.created_at
		)
		SELECT id, subject, aggregate_type, aggregate_id, payload, headers,
		       available_at, attempts, publish_failures, created_at
		FROM claimed
		ORDER BY available_at ASC, created_at ASC
	`, limit, fmt.Sprintf("%d seconds", int(lease/time.Second)), owner)
	if err != nil {
		return nil, fmt.Errorf("claim outbox events: %w", err)
	}
	defer rows.Close()

	events, err := scanEvents(rows)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit outbox claim: %w", err)
	}
	return events, nil
}

func (r *PostgresRepository) MarkPublished(ctx context.Context, id uuid.UUID, owner string) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE outbox_events
		SET published_at = now(), locked_at = NULL, locked_by = NULL, updated_at = now()
		WHERE id = $1
		  AND published_at IS NULL
		  AND locked_by = $2
	`, id, owner)
	if err != nil {
		return fmt.Errorf("mark outbox event %s published: %w", id, err)
	}
	if result.RowsAffected() != 1 {
		return ErrClaimLost
	}
	return nil
}

func (r *PostgresRepository) MarkRetry(ctx context.Context, id uuid.UUID, owner string, availableAt time.Time, code, reason string) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE outbox_events
		SET available_at = $3,
		    publish_failures = publish_failures + 1,
		    last_error = $4,
		    locked_at = NULL,
		    locked_by = NULL,
		    updated_at = now()
		WHERE id = $1
		  AND published_at IS NULL
		  AND quarantined_at IS NULL
		  AND locked_by = $2
	`, id, owner, availableAt, formatFailure(code, reason))
	if err != nil {
		return fmt.Errorf("mark outbox event %s retryable: %w", id, err)
	}
	if result.RowsAffected() != 1 {
		return ErrClaimLost
	}
	return nil
}

func (r *PostgresRepository) Quarantine(ctx context.Context, id uuid.UUID, owner, code, reason string) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE outbox_events
		SET quarantined_at = now(),
		    quarantine_code = $3,
		    quarantine_reason = $4,
		    publish_failures = publish_failures + 1,
		    last_error = $4,
		    locked_at = NULL,
		    locked_by = NULL,
		    updated_at = now()
		WHERE id = $1
		  AND published_at IS NULL
		  AND quarantined_at IS NULL
		  AND locked_by = $2
	`, id, owner, code, reason)
	if err != nil {
		return fmt.Errorf("quarantine outbox event %s: %w", id, err)
	}
	if result.RowsAffected() != 1 {
		return ErrClaimLost
	}
	return nil
}

func (r *PostgresRepository) Redrive(ctx context.Context, id uuid.UUID, availableAt time.Time) error {
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
	`, id, availableAt)
	if err != nil {
		return fmt.Errorf("redrive outbox event %s: %w", id, err)
	}
	if result.RowsAffected() != 1 {
		return ErrNotQuarantined
	}
	return nil
}

func (r *PostgresRepository) Release(ctx context.Context, id uuid.UUID, owner string) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE outbox_events
		SET locked_at = NULL, locked_by = NULL, updated_at = now()
		WHERE id = $1
		  AND published_at IS NULL
		  AND quarantined_at IS NULL
		  AND locked_by = $2
	`, id, owner)
	if err != nil {
		return fmt.Errorf("release outbox event %s: %w", id, err)
	}
	if result.RowsAffected() != 1 {
		return ErrClaimLost
	}
	return nil
}

func headersJSON(headers map[string]string) []byte {
	payload, _ := json.Marshal(headers)
	return payload
}

func formatFailure(code, reason string) string {
	if code == "" {
		return reason
	}
	if reason == "" {
		return code
	}
	return code + ": " + reason
}

type rowScanner interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

func scanEvents(rows rowScanner) ([]Event, error) {
	events := make([]Event, 0)
	for rows.Next() {
		var event Event
		var payload []byte
		var headersJSONBytes []byte
		if err := rows.Scan(
			&event.ID,
			&event.Subject,
			&event.AggregateType,
			&event.AggregateID,
			&payload,
			&headersJSONBytes,
			&event.AvailableAt,
			&event.Attempts,
			&event.PublishFailures,
			&event.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan outbox event: %w", err)
		}
		event.Payload = json.RawMessage(payload)
		if len(headersJSONBytes) != 0 {
			if err := json.Unmarshal(headersJSONBytes, &event.Headers); err != nil {
				return nil, fmt.Errorf("decode outbox headers: %w", err)
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
