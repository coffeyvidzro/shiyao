package websocket

const (
	TypeExec   = "exec"
	TypeStdout = "stdout"
	TypeStderr = "stderr"
	TypeResult = "result"
	TypeError  = "error"
)

type ClientMessage struct {
	Type      string            `json:"type"`
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	TimeoutMS int64             `json:"timeout_ms,omitempty"`
}

type ServerMessage struct {
	Type   string      `json:"type"`
	Data   string      `json:"data,omitempty"`
	Result *ExecResult `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

type ExecResult struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
	Error    string `json:"error,omitempty"`
}
