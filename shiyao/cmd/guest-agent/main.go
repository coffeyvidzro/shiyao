package main

import (
	"bufio"
	"bytes"
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

	decoder := json.NewDecoder(bufio.NewReader(conn))
	var req vm.ExecRequest
	if err := decoder.Decode(&req); err != nil {
		return
	}

	result := execute(req)
	payload, err := vm.EncodeMessage(result)
	if err != nil {
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

	// Capture stdout and stderr independently to preserve output on non-zero exit
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	result.Stdout = stdoutBuf.String()
	result.Stderr = stderrBuf.String()

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
