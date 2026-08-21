package nats

import (
	"context"
	"fmt"
	"strings"

	natsgo "github.com/nats-io/nats.go"
	natsjs "github.com/nats-io/nats.go/jetstream"
)

type Publisher interface {
	Publish(context.Context, string, []byte, map[string]string, string) error
}

type PublishError struct {
	err       error
	code      string
	retryable bool
	permanent bool
}

func (err PublishError) Error() string       { return err.err.Error() }
func (err PublishError) Unwrap() error       { return err.err }
func (err PublishError) Retryable() bool     { return err.retryable }
func (err PublishError) Permanent() bool     { return err.permanent }
func (err PublishError) FailureCode() string { return err.code }

func (c *Client) Publish(ctx context.Context, subject string, payload []byte, headers map[string]string, messageID string) error {
	if ctx == nil {
		return PublishError{err: fmt.Errorf("JetStream publish context is required"), code: "invalid_context", permanent: true}
	}
	if c == nil || c.jetStream == nil || c.connection == nil {
		return PublishError{err: ErrClientUnavailable, code: "client_unavailable", retryable: true}
	}
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return PublishError{err: fmt.Errorf("JetStream subject is required"), code: "invalid_subject", permanent: true}
	}
	if len(payload) > maxMessageSize {
		return PublishError{
			err:       fmt.Errorf("JetStream message payload is %d bytes; maximum is %d", len(payload), maxMessageSize),
			code:      "message_too_large",
			permanent: true,
		}
	}

	message := &natsgo.Msg{Subject: subject, Data: payload, Header: natsgo.Header{}}
	for key, value := range headers {
		key = strings.TrimSpace(key)
		if key != "" {
			message.Header.Set(key, value)
		}
	}

	options := make([]natsjs.PublishOpt, 0, 1)
	if messageID = strings.TrimSpace(messageID); messageID != "" {
		options = append(options, natsjs.WithMsgID(messageID))
	}
	if _, err := c.jetStream.PublishMsg(ctx, message, options...); err != nil {
		return PublishError{err: fmt.Errorf("publish JetStream message to %s: %w", subject, err), code: "jetstream_publish_failed", retryable: true}
	}
	return nil
}
