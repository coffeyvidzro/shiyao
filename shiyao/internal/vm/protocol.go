package vm

import (
	"encoding/json"
	"fmt"
)

const (
	GuestVsockPort  uint32 = 1024
	ProtocolVersion        = 1
	// MaxRequestBytes limits the maximum size of incoming JSON requests
	MaxRequestBytes = 1 << 20 // 1 MiB
	// MaxOutputBytes limits stdout/stderr capture per command
	MaxOutputBytes = 10 << 20 // 10 MiB
	// MaxEnvEntries limits number of environment variables
	MaxEnvEntries = 100
	// MaxEnvKeyLen limits environment variable key length
	MaxEnvKeyLen = 256
	// MaxEnvValLen limits environment variable value length
	MaxEnvValLen = 64 << 10 // 64 KiB
	// MaxArgs limits number of command arguments
	MaxArgs = 256
	// MaxArgLen limits individual argument length
	MaxArgLen = 64 << 10 // 64 KiB
	// MaxCommandLen limits command path length
	MaxCommandLen = 4096
)

// ExecRequest is the command sent from the host to the guest agent.
// Arguments are passed directly to the process; no shell is involved.
type ExecRequest struct {
	Version   int               `json:"version"`
	ID        string            `json:"id"`
	Command   string            `json:"command"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	TimeoutMS int64             `json:"timeout_ms,omitempty"`
}

// ExecResult is the guest agent's complete command result.
type ExecResult struct {
	Version  int    `json:"version"`
	ID       string `json:"id"`
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
	Error    string `json:"error,omitempty"`
}

func (r ExecRequest) Validate() error {
	if r.Version != ProtocolVersion {
		return fmt.Errorf("unsupported protocol version %d", r.Version)
	}
	if r.ID == "" {
		return fmt.Errorf("request id is required")
	}
	if r.Command == "" {
		return fmt.Errorf("command is required")
	}
	if r.TimeoutMS < 0 {
		return fmt.Errorf("timeout_ms cannot be negative")
	}

	// Validate request size limits to prevent resource exhaustion
	if len(r.Command) > MaxCommandLen {
		return fmt.Errorf("command too long: %d > %d", len(r.Command), MaxCommandLen)
	}
	if len(r.Args) > MaxArgs {
		return fmt.Errorf("too many arguments: %d > %d", len(r.Args), MaxArgs)
	}
	for i, arg := range r.Args {
		if len(arg) > MaxArgLen {
			return fmt.Errorf("argument %d too long: %d > %d", i, len(arg), MaxArgLen)
		}
	}
	if len(r.Env) > MaxEnvEntries {
		return fmt.Errorf("too many environment variables: %d > %d", len(r.Env), MaxEnvEntries)
	}
	for key, value := range r.Env {
		if len(key) > MaxEnvKeyLen {
			return fmt.Errorf("environment key %q too long: %d > %d", key, len(key), MaxEnvKeyLen)
		}
		if len(value) > MaxEnvValLen {
			return fmt.Errorf("environment value for %q too long: %d > %d", key, len(value), MaxEnvValLen)
		}
	}

	return nil
}

func EncodeMessage(v any) ([]byte, error) {
	payload, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("encode message: %w", err)
	}
	return append(payload, '\n'), nil
}
