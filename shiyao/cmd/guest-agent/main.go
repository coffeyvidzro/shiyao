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
	"time"

	"github.com/mdlayher/vsock"

	"github.com/coffeyvidzro/shiyao/internal/vm"
)

const (
	// maxRequestBytes limits the maximum size of incoming JSON requests
	maxRequestBytes = 1 << 20 // 1 MiB
	// maxOutputBytes limits stdout/stderr capture per command
	maxOutputBytes = 10 << 20 // 10 MiB
	// maxEnvEntries limits number of environment variables
	maxEnvEntries = 100
	// maxEnvKeyLen limits environment variable key length
	maxEnvKeyLen = 256
	// maxEnvValLen limits environment variable value length
	maxEnvValLen = 64 << 10 // 64 KiB
	// maxArgs limits number of command arguments
	maxArgs = 256
	// maxArgLen limits individual argument length
	maxArgLen = 64 << 10 // 64 KiB
	// maxCommandLen limits command path length
	maxCommandLen = 4096
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
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

	// Wrap connection with size limit to prevent memory exhaustion
	limitedConn := io.LimitReader(conn, maxRequestBytes)
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

func sendError(conn io.Writer, err error) {
	result := vm.ExecResult{
		Version:  vm.ProtocolVersion,
		ID:       "",
		ExitCode: -1,
		Error:    err.Error(),
	}
	payload, encodeErr := vm.EncodeMessage(result)
	if encodeErr != nil {
		return
	}
	_, _ = conn.Write(payload)
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

	// Validate request size limits to prevent resource exhaustion
	if len(req.Command) > maxCommandLen {
		result.Error = fmt.Sprintf("command too long: %d > %d", len(req.Command), maxCommandLen)
		return result
	}
	if len(req.Args) > maxArgs {
		result.Error = fmt.Sprintf("too many arguments: %d > %d", len(req.Args), maxArgs)
		return result
	}
	for i, arg := range req.Args {
		if len(arg) > maxArgLen {
			result.Error = fmt.Sprintf("argument %d too long: %d > %d", i, len(arg), maxArgLen)
			return result
		}
	}
	if len(req.Env) > maxEnvEntries {
		result.Error = fmt.Sprintf("too many environment variables: %d > %d", len(req.Env), maxEnvEntries)
		return result
	}
	for key, value := range req.Env {
		if len(key) > maxEnvKeyLen {
			result.Error = fmt.Sprintf("environment key %q too long: %d > %d", key, len(key), maxEnvKeyLen)
			return result
		}
		if len(value) > maxEnvValLen {
			result.Error = fmt.Sprintf("environment value for %q too long: %d > %d", key, len(value), maxEnvValLen)
			return result
		}
	}

	ctx := context.Background()
	cancel := func() {}
	if req.TimeoutMS > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(req.TimeoutMS)*time.Millisecond)
	}
	defer cancel()

	cmd := exec.CommandContext(ctx, req.Command, req.Args...)

	// Load base environment + /etc/shiyao-env
	cmd.Env = loadPresetEnv()
	for key, value := range req.Env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}

	// Capture stdout and stderr with bounded buffers to prevent memory exhaustion
	stdoutBuf := &limitedBuffer{limit: maxOutputBytes}
	stderrBuf := &limitedBuffer{limit: maxOutputBytes}
	cmd.Stdout = stdoutBuf
	cmd.Stderr = stderrBuf

	err := cmd.Run()
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
	} else {
		result.Error = err.Error()
		if ctx.Err() != nil {
			result.Error = ctx.Err().Error()
		}
	}

	return result
}

func loadPresetEnv() []string {
	env := os.Environ()
	data, err := os.ReadFile("/etc/shiyao-env")
	if err != nil {
		return env
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			env = append(env, line)
		}
	}
	return env
}

// limitedBuffer is a bounded buffer that discards data after reaching its limit.
type limitedBuffer struct {
	limit     int
	buf       []byte
	truncated bool
}

func (b *limitedBuffer) Write(p []byte) (n int, err error) {
	if b.truncated {
		// Already at limit, discard all further writes
		return len(p), nil
	}

	remaining := b.limit - len(b.buf)
	if remaining <= 0 {
		b.truncated = true
		return len(p), nil
	}

	if len(p) > remaining {
		p = p[:remaining]
		b.buf = append(b.buf, p...)
		b.truncated = true
		return len(p) + (len(p) - remaining), nil
	}

	b.buf = append(b.buf, p...)
	return len(p), nil
}

func (b *limitedBuffer) String() string {
	return string(b.buf)
}
