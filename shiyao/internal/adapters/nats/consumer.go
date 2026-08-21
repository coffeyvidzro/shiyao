package nats

import (
	"context"
	"encoding/json"
	"fmt"

	natsgo "github.com/nats-io/nats.go"
)

type Handler[T any] func(context.Context, T) error

type Subscription struct {
	sub *natsgo.Subscription
}

func (s *Subscription) Unsubscribe() error {
	if s == nil || s.sub == nil {
		return nil
	}
	return s.sub.Unsubscribe()
}

func SubscribeJSON[T any](client *Client, subject string, handler Handler[T]) (*Subscription, error) {
	return QueueSubscribeJSON(client, subject, "", handler)
}

func QueueSubscribeJSON[T any](client *Client, subject, queue string, handler Handler[T]) (*Subscription, error) {
	if client == nil || client.Conn() == nil {
		return nil, fmt.Errorf("nats client is not configured")
	}
	if handler == nil {
		return nil, fmt.Errorf("nats handler is required")
	}

	callback := func(msg *natsgo.Msg) {
		var event T
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			return
		}
		_ = handler(context.Background(), event)
	}

	var (
		sub *natsgo.Subscription
		err error
	)
	if queue == "" {
		sub, err = client.Conn().Subscribe(subject, callback)
	} else {
		sub, err = client.Conn().QueueSubscribe(subject, queue, callback)
	}
	if err != nil {
		return nil, fmt.Errorf("subscribe %s: %w", subject, err)
	}
	return &Subscription{sub: sub}, nil
}
