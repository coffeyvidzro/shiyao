package vsock

import (
	"net"
	"testing"
	"time"

	"github.com/mdlayher/vsock"
)

type authTestConn struct{ remote net.Addr }

func (c authTestConn) Read([]byte) (int, error) { return 0, nil }
func (c authTestConn) Write([]byte) (int, error) { return 0, nil }
func (c authTestConn) Close() error { return nil }
func (c authTestConn) LocalAddr() net.Addr { return &vsock.Addr{ContextID: HostCID, Port: 1024} }
func (c authTestConn) RemoteAddr() net.Addr { return c.remote }
func (c authTestConn) SetDeadline(_ time.Time) error { return nil }
func (c authTestConn) SetReadDeadline(_ time.Time) error { return nil }
func (c authTestConn) SetWriteDeadline(_ time.Time) error { return nil }

func TestAuthorizeGuestConnection(t *testing.T) {
	conn := authTestConn{remote: &vsock.Addr{ContextID: 3, Port: GuestPort}}
	if err := AuthorizeGuestConnection(conn, 3); err != nil {
		t.Fatalf("expected matching guest CID to pass: %v", err)
	}
	if err := AuthorizeGuestConnection(conn, 4); err == nil {
		t.Fatal("expected mismatched guest CID to fail")
	}
}

func TestAuthorizeHostConnection(t *testing.T) {
	if err := AuthorizeHostConnection(authTestConn{remote: &vsock.Addr{ContextID: HostCID, Port: 1234}}); err != nil {
		t.Fatalf("expected host CID to pass: %v", err)
	}
	if err := AuthorizeHostConnection(authTestConn{remote: &vsock.Addr{ContextID: 3, Port: 1234}}); err == nil {
		t.Fatal("expected guest CID to fail host authorization")
	}
}
