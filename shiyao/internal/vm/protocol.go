package vm

import (
	"encoding/json"
	"fmt"
	"regexp"
)

const (
	GuestVsockPort       uint32 = 1024
	ProtocolVersion             = 1
	MaxRequestBytes             = 1 << 20
	MaxOutputBytes              = 10 << 20
	MaxEnvEntries               = 100
	MaxEnvKeyLen                = 256
	MaxEnvValLen                = 64 << 10
	MaxArgs                     = 256
	MaxArgLen                   = 64 << 10
	MaxCommandLen               = 4096
	MaxTimeoutMS         int64  = 5 * 60 * 1000
	MaxConcurrentCommands       = 4
)

var allowedEnvKey = regexp.MustCompile(`^(LANG|LC_[A-Z0-9_]+|TZ|TERM|COLORTERM|NO_COLOR|FORCE_COLOR|PYTHONIOENCODING|GOPROXY|GOSUMDB|GOFLAGS|RUST_BACKTRACE|NODE_OPTIONS)$`)

type ExecRequest struct {
	Version   int               `json:"version"`
	ID        string            `json:"id"`
	Command   string            `json:"command"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	TimeoutMS int64             `json:"timeout_ms,omitempty"`
}

type ExecResult struct {
	Version  int    `json:"version"`
	ID       string `json:"id"`
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
	Error    string `json:"error,omitempty"`
}

func (r ExecRequest) Validate() error {
	if r.Version != ProtocolVersion { return fmt.Errorf("unsupported protocol version %d", r.Version) }
	if r.ID == "" { return fmt.Errorf("request id is required") }
	if len(r.ID) > 128 { return fmt.Errorf("request id too long") }
	if r.Command == "" { return fmt.Errorf("command is required") }
	if r.TimeoutMS < 0 { return fmt.Errorf("timeout_ms cannot be negative") }
	if r.TimeoutMS > MaxTimeoutMS { return fmt.Errorf("timeout_ms exceeds maximum of %d", MaxTimeoutMS) }
	if len(r.Command) > MaxCommandLen { return fmt.Errorf("command too long: %d > %d", len(r.Command), MaxCommandLen) }
	if len(r.Args) > MaxArgs { return fmt.Errorf("too many arguments: %d > %d", len(r.Args), MaxArgs) }
	for i, arg := range r.Args { if len(arg) > MaxArgLen { return fmt.Errorf("argument %d too long: %d > %d", i, len(arg), MaxArgLen) } }
	if len(r.Env) > MaxEnvEntries { return fmt.Errorf("too many environment variables: %d > %d", len(r.Env), MaxEnvEntries) }
	for key, value := range r.Env {
		if len(key) > MaxEnvKeyLen { return fmt.Errorf("environment key %q too long: %d > %d", key, MaxEnvKeyLen) }
		if !allowedEnvKey.MatchString(key) { return fmt.Errorf("environment variable %q is not permitted", key) }
		if len(value) > MaxEnvValLen { return fmt.Errorf("environment value for %q too long: %d > %d", key, MaxEnvValLen) }
	}
	return nil
}

func EncodeMessage(v any) ([]byte, error) {
	payload, err := json.Marshal(v)
	if err != nil { return nil, fmt.Errorf("encode message: %w", err) }
	return append(payload, '\n'), nil
}
