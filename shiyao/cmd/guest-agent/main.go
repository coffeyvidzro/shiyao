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

	"github.com/coffeyvidzro/shiyao/internal/vm"
	"github.com/mdlayher/vsock"
)

var commandSlots = make(chan struct{}, vm.MaxConcurrentCommands)

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

	listener, err := vsock.Listen(vm.GuestVsockPort, nil)
	if err != nil {
		return fmt.Errorf("listen on guest vsock port %d: %w", vm.GuestVsockPort, err)
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

	limitedConn := io.LimitReader(conn, vm.MaxRequestBytes)
	decoder := json.NewDecoder(bufio.NewReader(limitedConn))
	decoder.DisallowUnknownFields()

	var req vm.ExecRequest
	if err := decoder.Decode(&req); err != nil {
		sendError(conn, fmt.Errorf("decode request: %w", err))
		return
	}

	result := execute(req)
	payload, err := vm.EncodeMessage(result)
	if err != nil {
		return
	}
	_, _ = conn.Write(payload)
}

func authorizedHost(conn io.ReadWriteCloser) bool {
	c, ok := conn.(*vsock.Conn)
	if !ok {
		return false
	}
	addr, ok := c.RemoteAddr().(*vsock.Addr)
	return ok && addr.CID == 2
}

func sendError(conn io.Writer, err error) {
	result := vm.ExecResult{
		Version:  vm.ProtocolVersion,
		ExitCode: -1,
		Error:    err.Error(),
	}
	payload, encodeErr := vm.EncodeMessage(result)
	if encodeErr == nil {
		_, _ = conn.Write(payload)
	}
}

func execute(req vm.ExecRequest) vm.ExecResult {
	result := vm.ExecResult{
		Version:  vm.ProtocolVersion,
		ID:       req.ID,
		ExitCode: -1,
	}
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
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid:   true,
		Pdeathsig: syscall.SIGKILL,
	}
	cmd.Env = loadPresetEnv()
	for key, value := range req.Env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}

	stdoutBuf := &limitedBuffer{limit: vm.MaxOutputBytes}
	stderrBuf := &limitedBuffer{limit: vm.MaxOutputBytes}
	cmd.Stdout = stdoutBuf
	cmd.Stderr = stderrBuf

	if err := startWithResourceLimits(cmd); err != nil {
		result.Error = err.Error()
		return result
	}

	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()

	var err error
	select {
	case err = <-waitCh:
	case <-ctx.Done():
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		err = <-waitCh
		result.Error = ctx.Err().Error()
	}

	result.Stdout = stdoutBuf.String()
	result.Stderr = stderrBuf.String()
	if stdoutBuf.truncated {
		result.Stdout += "\n[OUTPUT TRUNCATED DUE TO SIZE LIMIT]"
	}
	if stderrBuf.truncated {
		result.Stderr += "\n[OUTPUT TRUNCATED DUE TO SIZE LIMIT]"
	}
	if err == nil {
		result.ExitCode = 0
		return result
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
	} else if result.Error == "" {
		result.Error = err.Error()
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

func (b *limitedBuffer) String() string {
	return string(b.buf)
}
