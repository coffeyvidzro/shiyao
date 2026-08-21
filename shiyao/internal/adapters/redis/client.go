// Package redis provides a small Redis client for control-plane coordination.
package redis

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// Client contains the connection settings shared by Redis operations.
type Client struct {
	address  string
	username string
	password string
	database int
	useTLS   bool
}

// New parses a redis:// or rediss:// URL without opening a connection.
func New(redisURL string) (*Client, error) {
	if strings.TrimSpace(redisURL) == "" {
		return nil, fmt.Errorf("redis URL is required")
	}

	u, err := url.Parse(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis URL: %w", err)
	}
	if u.Scheme != "redis" && u.Scheme != "rediss" {
		return nil, fmt.Errorf("unsupported redis URL scheme %q", u.Scheme)
	}
	if u.Hostname() == "" {
		return nil, fmt.Errorf("redis host is required")
	}

	port := u.Port()
	if port == "" {
		port = "6379"
	}
	database, err := parseDatabase(u.Path)
	if err != nil {
		return nil, err
	}

	client := &Client{address: net.JoinHostPort(u.Hostname(), port), database: database, useTLS: u.Scheme == "rediss"}
	if u.User != nil {
		client.username = u.User.Username()
		client.password, _ = u.User.Password()
	}
	return client, nil
}

// Ping verifies the Redis endpoint and configured credentials.
func (c *Client) Ping(ctx context.Context) error {
	conn, err := c.dial(ctx)
	if err != nil {
		return fmt.Errorf("connect redis: %w", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	if c.password != "" {
		args := []string{"AUTH"}
		if c.username != "" {
			args = append(args, c.username)
		}
		args = append(args, c.password)
		if err := command(conn, reader, args...); err != nil {
			return fmt.Errorf("authenticate redis: %w", err)
		}
	}
	if c.database != 0 {
		if err := command(conn, reader, "SELECT", strconv.Itoa(c.database)); err != nil {
			return fmt.Errorf("select redis database: %w", err)
		}
	}
	if err := command(conn, reader, "PING"); err != nil {
		return fmt.Errorf("ping redis: %w", err)
	}
	return nil
}

func (c *Client) dial(ctx context.Context) (net.Conn, error) {
	dialer := net.Dialer{}
	if c.useTLS {
		return tls.DialWithDialer(&dialer, "tcp", c.address, &tls.Config{MinVersion: tls.VersionTLS12})
	}
	return dialer.DialContext(ctx, "tcp", c.address)
}

func parseDatabase(path string) (int, error) {
	if path == "" || path == "/" {
		return 0, nil
	}
	database, err := strconv.Atoi(strings.TrimPrefix(path, "/"))
	if err != nil || database < 0 {
		return 0, fmt.Errorf("invalid redis database %q", path)
	}
	return database, nil
}

func command(writer io.Writer, reader *bufio.Reader, args ...string) error {
	if _, err := fmt.Fprintf(writer, "*%d\r\n", len(args)); err != nil {
		return err
	}
	for _, arg := range args {
		if _, err := fmt.Fprintf(writer, "$%d\r\n%s\r\n", len(arg), arg); err != nil {
			return err
		}
	}

	response, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	if strings.HasPrefix(response, "-ERR") {
		return fmt.Errorf("%s", strings.TrimSpace(response))
	}
	if response != "+OK\r\n" && response != "+PONG\r\n" {
		return fmt.Errorf("unexpected redis response %q", strings.TrimSpace(response))
	}
	return nil
}
