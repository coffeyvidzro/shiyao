package vm

import (
	"context"
	"fmt"
	"net"

	fcsv "github.com/firecracker-microvm/firecracker-go-sdk/vsock"
	"github.com/mdlayher/vsock"
)

const HostVSOCKCID uint32 = 2

// SetupVsockListener starts a host-side listener for guest VSOCK connections.
// The listener accepts only the configured Firecracker guest connection path;
// callers should wrap accepted connections with AuthorizeGuestConnection before
// decoding commands.
func SetupVsockListener(ctx context.Context, cfg VsockConfig, port uint32) (net.Listener, error) {
	if cfg.GuestCID <= 2 { return nil, fmt.Errorf("invalid guest CID %d: must be greater than 2", cfg.GuestCID) }
	if port == 0 { return nil, fmt.Errorf("vsock port must be greater than 0") }
	listener, err := fcsv.Listener(ctx, nil, port)
	if err != nil { return nil, fmt.Errorf("start vsock listener for guest CID %d on port %d: %w", cfg.GuestCID, port, err) }
	return listener, nil
}

// AuthorizeGuestConnection ensures a host-side VSOCK connection belongs to the
// expected guest CID. VSOCK CIDs are assigned by the hypervisor, so this is a
// stronger authorization primitive than trusting a peer-supplied VM ID.
func AuthorizeGuestConnection(conn net.Conn, expectedCID uint32) error {
	if expectedCID <= 2 { return fmt.Errorf("invalid expected guest CID %d", expectedCID) }
	addr, ok := conn.RemoteAddr().(*vsock.Addr)
	if !ok { return fmt.Errorf("connection is not a VSOCK connection") }
	if addr.CID != expectedCID { return fmt.Errorf("unauthorized VSOCK peer CID %d", addr.CID) }
	return nil
}

func CleanupVsock(listener net.Listener) error {
	if listener == nil { return nil }
	if err := listener.Close(); err != nil { return fmt.Errorf("close vsock listener: %w", err) }
	return nil
}
