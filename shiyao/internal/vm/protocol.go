package vm

import (
	"encoding/json"
	"fmt"
)

const (
	GuestVsockPort uint32 = 1024
	ProtocolVersion       = 1
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
	return nil
}

func EncodeMessage(v any) ([]byte, error) {
	payload, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("encode message: %w", err)
	}
	return append(payload, '\n'), nil
}
