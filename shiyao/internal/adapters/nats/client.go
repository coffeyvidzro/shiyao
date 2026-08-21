package nats

import (
	"context"
	"fmt"
	"time"

	natsgo "github.com/nats-io/nats.go"
)

const connectTimeout = 5 * time.Second

type Client struct {
	conn *natsgo.Conn
}

func New(ctx context.Context, url string) (*Client, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	conn, err := natsgo.Connect(url, natsgo.Timeout(connectTimeout), natsgo.Name("shiyao"))
	if err != nil {
		return nil, fmt.Errorf("connect nats: %w", err)
	}
	return &Client{conn: conn}, nil
}

func (c *Client) Close() {
	if c == nil || c.conn == nil {
		return
	}
	c.conn.Drain()
	c.conn.Close()
}

func (c *Client) Conn() *natsgo.Conn {
	if c == nil {
		return nil
	}
	return c.conn
}
