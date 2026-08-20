package vsock

import (
	"context"
	"fmt"
	"net"

	fcsv "github.com/firecracker-microvm/firecracker-go-sdk/vsock"
)

func SetupListener(ctx context.Context, cfg Config, port uint32) (net.Listener, error) {
	if cfg.GuestCID <= HostCID {
		return nil, fmt.Errorf("invalid guest CID %d: must be greater than %d", cfg.GuestCID, HostCID)
	}
	if port == 0 {
		return nil, fmt.Errorf("vsock port must be greater than 0")
	}
	listener, err := fcsv.Listener(ctx, nil, port)
	if err != nil {
		return nil, fmt.Errorf("start vsock listener for guest CID %d on port %d: %w", cfg.GuestCID, port, err)
	}
	return listener, nil
}

func CleanupListener(listener net.Listener) error {
	if listener == nil {
		return nil
	}
	if err := listener.Close(); err != nil {
		return fmt.Errorf("close vsock listener: %w", err)
	}
	return nil
}
