package websocket

import (
	"time"

	"github.com/gorilla/websocket"
)

const connectionTimeout = 60 * time.Second

type Conn struct {
	*websocket.Conn
}

func NewConn(conn *websocket.Conn) *Conn {
	return &Conn{Conn: conn}
}

func (c *Conn) SetDeadlines() {
	deadline := time.Now().Add(connectionTimeout)
	_ = c.Conn.SetReadDeadline(deadline)
	_ = c.Conn.SetWriteDeadline(deadline)
}
