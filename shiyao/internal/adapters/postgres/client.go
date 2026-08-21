package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	postgresMaxConns        = 20
	postgresMinConns        = 5
	postgresMaxConnIdle     = 30 * time.Minute
	postgresMaxConnLifetime = 1 * time.Hour
)

func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse postgres URL: %w", err)
	}

	cfg.MaxConns = postgresMaxConns
	cfg.MinConns = postgresMinConns
	cfg.MaxConnIdleTime = postgresMaxConnIdle
	cfg.MaxConnLifetime = postgresMaxConnLifetime

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return pool, nil
}
