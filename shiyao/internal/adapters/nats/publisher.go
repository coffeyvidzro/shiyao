package nats

import (
	"context"
	"encoding/json"
	"fmt"
)

type Publisher struct {
	client *Client
}

func NewPublisher(client *Client) *Publisher {
	return &Publisher{client: client}
}

func (p *Publisher) PublishJSON(ctx context.Context, subject string, value any) error {
	if p == nil || p.client == nil || p.client.Conn() == nil {
		return fmt.Errorf("nats publisher is not configured")
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal nats message: %w", err)
	}
	if err := p.client.Conn().Publish(subject, payload); err != nil {
		return fmt.Errorf("publish %s: %w", subject, err)
	}
	if err := p.client.Conn().FlushWithContext(ctx); err != nil {
		return fmt.Errorf("flush %s: %w", subject, err)
	}
	return nil
}
