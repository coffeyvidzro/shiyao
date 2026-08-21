package nats

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	natsgo "github.com/nats-io/nats.go"
	natsjs "github.com/nats-io/nats.go/jetstream"
)

const (
	defaultClientName     = "shiyao"
	defaultConnectTimeout = 5 * time.Second
	defaultReconnectWait  = 2 * time.Second
)

var ErrClientUnavailable = errors.New("JetStream client is unavailable")

type Client struct {
	connection *natsgo.Conn
	jetStream  natsjs.JetStream
}

func New(ctx context.Context, serverURL string, clientName ...string) (*Client, error) {
	if ctx == nil {
		return nil, errors.New("NATS context is required")
	}
	serverURL = strings.TrimSpace(serverURL)
	if serverURL == "" {
		return nil, errors.New("NATS URL is required")
	}

	name := defaultClientName
	if len(clientName) > 0 && strings.TrimSpace(clientName[0]) != "" {
		name = strings.TrimSpace(clientName[0])
	}

	connection, err := natsgo.Connect(serverURL, connectionOptions(name)...)
	if err != nil {
		return nil, fmt.Errorf("connect to NATS: %w", err)
	}

	jetStream, err := natsjs.New(connection)
	if err != nil {
		connection.Close()
		return nil, fmt.Errorf("initialize JetStream: %w", err)
	}
	if _, err := jetStream.AccountInfo(ctx); err != nil {
		connection.Close()
		return nil, fmt.Errorf("verify JetStream account: %w", err)
	}

	return &Client{connection: connection, jetStream: jetStream}, nil
}

func (c *Client) Close() error {
	if c == nil || c.connection == nil || c.connection.IsClosed() {
		return nil
	}
	if err := c.connection.Drain(); err != nil {
		c.connection.Close()
		return fmt.Errorf("drain NATS connection: %w", err)
	}
	return nil
}

func (c *Client) Conn() *natsgo.Conn {
	if c == nil {
		return nil
	}
	return c.connection
}

func (c *Client) JetStream() natsjs.JetStream {
	if c == nil {
		return nil
	}
	return c.jetStream
}

func connectionOptions(clientName string) []natsgo.Option {
	return []natsgo.Option{
		natsgo.Name(clientName),
		natsgo.Timeout(defaultConnectTimeout),
		natsgo.MaxReconnects(-1),
		natsgo.ReconnectWait(defaultReconnectWait),
	}
}
