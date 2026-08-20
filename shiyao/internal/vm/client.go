package vm

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	fcvsock "github.com/firecracker-microvm/firecracker-go-sdk/vsock"
)

// Exec connects to the guest agent through Firecracker's host-side Unix socket
// and requests one command execution.
func Exec(ctx context.Context, socketPath string, req ExecRequest) (ExecResult, error) {
	if err := req.Validate(); err != nil {
		return ExecResult{}, err
	}

	conn, err := fcvsock.DialContext(ctx, socketPath, GuestVsockPort)
	if err != nil {
		return ExecResult{}, fmt.Errorf("dial guest vsock: %w", err)
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	payload, err := EncodeMessage(req)
	if err != nil {
		return ExecResult{}, err
	}
	if _, err := conn.Write(payload); err != nil {
		return ExecResult{}, fmt.Errorf("send execution request: %w", err)
	}

	var result ExecResult
	if err := json.NewDecoder(bufio.NewReader(conn)).Decode(&result); err != nil {
		return ExecResult{}, fmt.Errorf("read execution result: %w", err)
	}
	if result.Version != ProtocolVersion {
		return ExecResult{}, fmt.Errorf("unsupported guest protocol version %d", result.Version)
	}
	if result.ID != req.ID {
		return ExecResult{}, fmt.Errorf("execution result id %q does not match request %q", result.ID, req.ID)
	}

	return result, nil
}

// SetConnectionDeadline is a small helper for callers that need a bounded
// VSOCK operation independent of the request timeout.
func SetConnectionDeadline(conn net.Conn, timeout time.Duration) error {
	if timeout <= 0 {
		return conn.SetDeadline(time.Time{})
	}
	return conn.SetDeadline(time.Now().Add(timeout))
}
