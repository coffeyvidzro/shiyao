// Package postgres provides the PostgreSQL connection pool used by the control plane.
package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Client owns a PostgreSQL connection.
type Client struct {
	config *pgx.ConnConfig
	conn   *pgx.Conn
}

// New validates connection settings without opening a connection. Call Open
// when startup should verify that PostgreSQL is reachable.
func New(databaseURL string) (*Client, error) {
	if strings.TrimSpace(databaseURL) == "" {
		return nil, fmt.Errorf("postgres database URL is required")
	}

	cfg, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse postgres database URL: %w", err)
	}

	return &Client{config: cfg}, nil
}

// Open establishes a connection and verifies that PostgreSQL is reachable.
func Open(ctx context.Context, databaseURL string) (*Client, error) {
	client, err := New(databaseURL)
	if err != nil {
		return nil, err
	}
	conn, err := pgx.ConnectConfig(ctx, client.config)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	client.conn = conn
	if err := client.Ping(ctx); err != nil {
		client.Close()
		return nil, err
	}
	return client, nil
}

// Conn returns the underlying connection for sqlc queries and transactions.
func (c *Client) Conn() *pgx.Conn {
	return c.conn
}

// Ping verifies that PostgreSQL can serve connections.
func (c *Client) Ping(ctx context.Context) error {
	if c == nil || c.conn == nil {
		return fmt.Errorf("postgres client is not connected")
	}
	if err := c.conn.Ping(ctx); err != nil {
		return fmt.Errorf("ping postgres: %w", err)
	}
	return nil
}

// Close releases all connections owned by the client.
func (c *Client) Close() {
	if c != nil && c.conn != nil {
		_ = c.conn.Close(context.Background())
	}
}
