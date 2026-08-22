package vsock

import (
	"fmt"
	"net"

	"github.com/mdlayher/vsock"
)

// AuthorizeGuestConnection verifies the hypervisor-assigned peer CID for a
// host-side connection to a specific guest.
func AuthorizeGuestConnection(conn net.Conn, expectedCID uint32) error {
	if expectedCID <= HostCID {
		return fmt.Errorf("invalid expected guest CID %d", expectedCID)
	}
	addr, ok := conn.RemoteAddr().(*vsock.Addr)
	if !ok {
		return fmt.Errorf("connection is not a VSOCK connection")
	}
	if addr.ContextID != expectedCID {
		return fmt.Errorf("unauthorized VSOCK peer CID %d", addr.ContextID)
	}
	return nil
}

// AuthorizeHostConnection verifies that a guest-side connection originates
// from the Firecracker host CID. Keeping this separate from guest authorization
// prevents accidentally accepting the host CID where a guest CID is required.
func AuthorizeHostConnection(conn net.Conn) error {
	addr, ok := conn.RemoteAddr().(*vsock.Addr)
	if !ok {
		return fmt.Errorf("connection is not a VSOCK connection")
	}
	if addr.ContextID != HostCID {
		return fmt.Errorf("unauthorized VSOCK host CID %d", addr.ContextID)
	}
	return nil
}
