package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	guestvsock "github.com/coffeyvidzro/shiyao/internal/vsock"
	"github.com/mdlayher/vsock"
)

var commandSlots = make(chan struct{}, guestvsock.MaxConcurrentCommands)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if err := setupEphemeralRootfs(); err != nil {
		return err
	}

	listener, err := vsock.Listen(guestvsock.GuestPort, nil)
	if err != nil {
		return fmt.Errorf("listen on guest vsock port %d: %w", guestvsock.GuestPort, err)
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			return fmt.Errorf("accept guest vsock connection: %w", err)
		}
		go serve(conn)
	}
}

func serve(conn io.ReadWriteCloser) {
	defer conn.Close()

	if !authorizedHost(conn) {
		sendError(conn, errors.New("unauthorized vsock peer"))
		return
	}

	limitedConn := io.LimitReader(conn, guestvsock.MaxRequestBytes)
	decoder := json.NewDecoder(bufio.NewReader(limitedConn))
	decoder.DisallowUnknownFields()

	var req guestvsock.ExecRequest
	if err := decoder.Decode(&req); err != nil {
		sendError(conn, fmt.Errorf("decode request: %w", err))
		return
	}

	result := execute(req)
	if req.Stream {
		streamResult(conn, result)
		return
	}
	payload, err := guestvsock.EncodeMessage(result)
	if err != nil {
		return
	}
	_, _ = conn.Write(payload)
}

func streamResult(conn io.Writer, result guestvsock.ExecResult) {
	for _, output := range []struct{ name, data string }{{"stdout", result.Stdout}, {"stderr", result.Stderr}} {
		for len(output.data) > 0 {
			n := len(output.data)
			if n > guestvsock.MaxStreamFrameBytes {
				n = guestvsock.MaxStreamFrameBytes
			}
			frame := guestvsock.ExecFrame{Version: guestvsock.ProtocolVersion, ID: result.ID, Stream: output.name, Data: output.data[:n]}
			payload, err := guestvsock.EncodeMessage(frame)
			if err != nil {
				return
			}
			if _, err := conn.Write(payload); err != nil {
				return
			}
			output.data = output.data[n:]
		}
	}
	result.Stdout, result.Stderr = "", ""
	payload, err := guestvsock.EncodeMessage(guestvsock.ExecFrame{Version: guestvsock.ProtocolVersion, ID: result.ID, Result: &result})
	if err == nil {
		_, _ = conn.Write(payload)
	}
}

func authorizedHost(conn io.ReadWriteCloser) bool {
	c, ok := conn.(*vsock.Conn)
	if !ok {
		return false
	}
	return guestvsock.AuthorizeHostConnection(c) == nil
}

func sendError(conn io.Writer, err error) {
	result := guestvsock.ExecResult{Version: guestvsock.ProtocolVersion, ExitCode: -1, Error: err.Error()}
	payload, encodeErr := guestvsock.EncodeMessage(result)
	if encodeErr == nil {
		_, _ = conn.Write(payload)
	}
}

func execute(req guestvsock.ExecRequest) guestvsock.ExecResult {
	result := guestvsock.ExecResult{Version: guestvsock.ProtocolVersion, ID: req.ID, ExitCode: -1}
	if err := req.Validate(); err != nil {
		result.Error = err.Error()
		return result
	}

	select {
	case commandSlots <- struct{}{}:
		defer func() { <-commandSlots }()
	default:
		result.Error = "guest command concurrency limit reached"
		return result
	}

	ctx := context.Background()
	cancel := func() {}
	if req.TimeoutMS > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(req.TimeoutMS)*time.Millisecond)
	}
	defer cancel()

	cmd := exec.Command(req.Command, req.Args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	cmd.Env = loadPresetEnv()
	for key, value := range req.Env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}

	stdoutBuf := &limitedBuffer{limit: guestvsock.MaxOutputBytes}
	stderrBuf := &limitedBuffer{limit: guestvsock.MaxOutputBytes}
	cmd.Stdout = stdoutBuf
	cmd.Stderr = stderrBuf

	cleanupCgroup, err := startWithResourceLimits(cmd)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()

	var waitErr error
	select {
	case waitErr = <-waitCh:
	case <-ctx.Done():
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		waitErr = <-waitCh
		result.Error = ctx.Err().Error()
	}

	if err := cleanupCgroup(); err != nil && result.Error == "" {
		result.Error = fmt.Sprintf("cleanup command cgroup: %v", err)
	}

	result.Stdout = stdoutBuf.String()
	result.Stderr = stderrBuf.String()
	if stdoutBuf.truncated {
		result.Stdout += "\n[OUTPUT TRUNCATED DUE TO SIZE LIMIT]"
	}
	if stderrBuf.truncated {
		result.Stderr += "\n[OUTPUT TRUNCATED DUE TO SIZE LIMIT]"
	}
	if waitErr == nil {
		result.ExitCode = 0
		return result
	}

	var exitErr *exec.ExitError
	if errors.As(waitErr, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
	} else if result.Error == "" {
		result.Error = waitErr.Error()
	}
	return result
}

func loadPresetEnv() []string {
	env := os.Environ()
	data, err := os.ReadFile("/etc/shiyao-env")
	if err != nil {
		return env
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			env = append(env, line)
		}
	}
	return env
}

type limitedBuffer struct {
	limit     int
	buf       []byte
	truncated bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.truncated {
		return len(p), nil
	}
	remaining := b.limit - len(b.buf)
	if remaining <= 0 {
		b.truncated = true
		return len(p), nil
	}
	if len(p) > remaining {
		b.buf = append(b.buf, p[:remaining]...)
		b.truncated = true
		return len(p), nil
	}
	b.buf = append(b.buf, p...)
	return len(p), nil
}

func (b *limitedBuffer) String() string { return string(b.buf) }
