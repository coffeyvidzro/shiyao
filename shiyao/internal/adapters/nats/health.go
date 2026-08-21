package nats

import (
	"context"
	"fmt"
)

type HealthChecker interface {
	Ping(context.Context) error
}

func (c *Client) Ping(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("JetStream health context is required")
	}
	if c == nil || c.connection == nil || c.jetStream == nil {
		return ErrClientUnavailable
	}
	if !c.connection.IsConnected() {
		return fmt.Errorf("JetStream client is not connected")
	}
	if _, err := c.jetStream.AccountInfo(ctx); err != nil {
		return fmt.Errorf("read JetStream account info: %w", err)
	}
	return nil
}
