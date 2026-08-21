package websocket

import (
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// Upgrader is the global configuration for upgrading HTTP connections to WebSocket.
var Upgrader = websocket.Upgrader{
	ReadBufferSize:    1024,
	WriteBufferSize:   1024,
	HandshakeTimeout:  10 * time.Second,
	EnableCompression: true,
	CheckOrigin: func(r *http.Request) bool {
		// TODO: In production, restrict this to the configured frontend origins.
		return true
	},
}
