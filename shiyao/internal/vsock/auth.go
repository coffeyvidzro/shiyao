package vsock

import (
	"fmt"
	"net"

	"github.com/mdlayher/vsock"
)

// AuthorizeGuestConnection verifies the hypervisor-assigned peer CID.
func AuthorizeGuestConnection(conn net.Conn, expectedCID uint32) error {
	if expectedCID <= HostCID {
		return fmt.Errorf("invalid expected guest CID %d", expectedCID)
	}
	addr, ok := conn.RemoteAddr().(*vsock.Addr)
	if !ok {
		return fmt.Errorf("connection is not a VSOCK connection")
	}
	if addr.CID != expectedCID {
		return fmt.Errorf("unauthorized VSOCK peer CID %d", addr.CID)
	}
	return nil
}
