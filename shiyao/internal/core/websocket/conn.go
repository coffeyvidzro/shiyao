package websocket

import (
	"time"

	"github.com/gorilla/websocket"
)

// DefaultConnectionTimeout is the fallback timeout for WebSocket connections.
const DefaultConnectionTimeout = 5 * time.Minute

// Conn wraps a websocket.Conn with helper methods for JSON messaging and deadline management.
type Conn struct {
	*websocket.Conn
}

// NewConn creates a new WebSocket connection wrapper.
func NewConn(conn *websocket.Conn) *Conn {
	return &Conn{Conn: conn}
}

// SetDeadlines applies a default read and write timeout to prevent hanging connections.
func (c *Conn) SetDeadlines() {
	deadline := time.Now().Add(DefaultConnectionTimeout)
	_ = c.Conn.SetReadDeadline(deadline)
	_ = c.Conn.SetWriteDeadline(deadline)
}

// SetExecutionDeadline dynamically extends the read/write deadlines based on the
// expected execution time, plus a buffer for network latency.
func (c *Conn) SetExecutionDeadline(timeoutMS int64) {
	buffer := 10 * time.Second
	timeout := time.Duration(timeoutMS) * time.Millisecond

	// Fallback to default if timeout is invalid or zero
	if timeout <= 0 {
		timeout = DefaultConnectionTimeout
	}

	deadline := time.Now().Add(timeout + buffer)
	_ = c.Conn.SetReadDeadline(deadline)
	_ = c.Conn.SetWriteDeadline(deadline)
}
