package vm

import (
	"context"
	"fmt"
	"net"

	fcsv "github.com/firecracker-microvm/firecracker-go-sdk/vsock"
)

// SetupVsockListener starts a host-side listener for guest VSOCK connections.
//
// The listener returned by this function implements net.Listener and accepts
// connections from the guest through Firecracker's VSOCK device.
func SetupVsockListener(
	ctx context.Context,
	cfg VsockConfig,
	port uint32,
) (net.Listener, error) {
	if cfg.GuestCID <= 2 {
		return nil, fmt.Errorf("invalid guest CID %d: must be greater than 2", cfg.GuestCID)
	}

	if port == 0 {
		return nil, fmt.Errorf("vsock port must be greater than 0")
	}

	// The Firecracker SDK's Listener creates the host-side listener for
	// guest-initiated VSOCK connections.
	listener, err := fcsv.Listener(ctx, nil, port)
	if err != nil {
		return nil, fmt.Errorf(
			"start vsock listener for guest CID %d on port %d: %w",
			cfg.GuestCID,
			port,
			err,
		)
	}

	return listener, nil
}

// CleanupVsock closes the host-side VSOCK listener.
func CleanupVsock(listener net.Listener) error {
	if listener == nil {
		return nil
	}

	if err := listener.Close(); err != nil {
		return fmt.Errorf("close vsock listener: %w", err)
	}

	return nil
}
