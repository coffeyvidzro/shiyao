package nats

import (
	"context"
	"fmt"
	"strings"

	natsjs "github.com/nats-io/nats.go/jetstream"
)

type ConsumerManager interface {
	CreateOrUpdateConsumer(context.Context, string, natsjs.ConsumerConfig) (natsjs.Consumer, error)
}

type ConsumerHandle interface {
	Stop()
}

func (c *Client) CreateOrUpdateConsumer(ctx context.Context, stream string, config natsjs.ConsumerConfig) (natsjs.Consumer, error) {
	if ctx == nil {
		return nil, fmt.Errorf("JetStream consumer context is required")
	}
	if c == nil || c.jetStream == nil {
		return nil, ErrClientUnavailable
	}
	stream = strings.TrimSpace(stream)
	if stream == "" {
		return nil, fmt.Errorf("JetStream stream name is required")
	}
	if strings.TrimSpace(config.Durable) == "" && strings.TrimSpace(config.Name) == "" {
		return nil, fmt.Errorf("JetStream consumer durable or name is required")
	}
	consumer, err := c.jetStream.CreateOrUpdateConsumer(ctx, stream, config)
	if err != nil {
		return nil, fmt.Errorf("create or update consumer %q on %s: %w", consumerName(config), stream, err)
	}
	return consumer, nil
}

func consumerName(config natsjs.ConsumerConfig) string {
	if durable := strings.TrimSpace(config.Durable); durable != "" {
		return durable
	}
	return strings.TrimSpace(config.Name)
}
