package vsock

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	fcvsock "github.com/firecracker-microvm/firecracker-go-sdk/vsock"
)

func Exec(ctx context.Context, socketPath string, req ExecRequest) (ExecResult, error) {
	if err := req.Validate(); err != nil {
		return ExecResult{}, err
	}
	conn, err := fcvsock.DialContext(ctx, socketPath, GuestPort)
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

// ExecStream executes a request and delivers bounded stdout/stderr frames in
// protocol order. The final frame always contains the terminal ExecResult.
func ExecStream(ctx context.Context, socketPath string, req ExecRequest, receive func(ExecFrame) error) (ExecResult, error) {
	if receive == nil {
		return ExecResult{}, fmt.Errorf("stream receiver is required")
	}
	req.Stream = true
	if err := req.Validate(); err != nil {
		return ExecResult{}, err
	}
	conn, err := fcvsock.DialContext(ctx, socketPath, GuestPort)
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
	decoder := json.NewDecoder(bufio.NewReader(conn))
	for {
		var frame ExecFrame
		if err := decoder.Decode(&frame); err != nil {
			return ExecResult{}, fmt.Errorf("read execution frame: %w", err)
		}
		if frame.Version != ProtocolVersion || frame.ID != req.ID {
			return ExecResult{}, fmt.Errorf("invalid execution frame")
		}
		if frame.Result != nil {
			return *frame.Result, nil
		}
		if frame.Stream != "stdout" && frame.Stream != "stderr" || len(frame.Data) > MaxStreamFrameBytes {
			return ExecResult{}, fmt.Errorf("invalid execution output frame")
		}
		if err := receive(frame); err != nil {
			return ExecResult{}, err
		}
	}
}

func SetConnectionDeadline(conn net.Conn, timeout time.Duration) error {
	if timeout <= 0 {
		return conn.SetDeadline(time.Time{})
	}
	return conn.SetDeadline(time.Now().Add(timeout))
}
