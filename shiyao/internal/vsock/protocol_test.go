package vsock

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestExecRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     ExecRequest
		wantErr bool
	}{
		{name: "valid request", req: ExecRequest{Version: ProtocolVersion, ID: "req-1", Command: "/bin/ls", Args: []string{"-la"}, TimeoutMS: 1000}},
		{name: "invalid protocol version", req: ExecRequest{Version: 99, ID: "req-1", Command: "/bin/ls"}, wantErr: true},
		{name: "missing request id", req: ExecRequest{Version: ProtocolVersion, Command: "/bin/ls"}, wantErr: true},
		{name: "missing command", req: ExecRequest{Version: ProtocolVersion, ID: "req-1"}, wantErr: true},
		{name: "negative timeout", req: ExecRequest{Version: ProtocolVersion, ID: "req-1", Command: "/bin/ls", TimeoutMS: -500}, wantErr: true},
		{name: "dangerous environment", req: ExecRequest{Version: ProtocolVersion, ID: "req-1", Command: "/bin/true", Env: map[string]string{"LD_PRELOAD": "/tmp/evil.so"}}, wantErr: true},
		{name: "excessive timeout", req: ExecRequest{Version: ProtocolVersion, ID: "req-1", Command: "/bin/true", TimeoutMS: MaxTimeoutMS + 1}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("ExecRequest.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEncodeMessage(t *testing.T) {
	req := ExecRequest{Version: ProtocolVersion, ID: "enc-1", Command: "echo"}
	encoded, err := EncodeMessage(req)
	if err != nil {
		t.Fatalf("failed to encode message: %v", err)
	}
	if !bytes.HasSuffix(encoded, []byte("\n")) {
		t.Errorf("expected newline delimiter at end of encoded message")
	}
	var decoded ExecRequest
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("failed to decode encoded message: %v", err)
	}
	if decoded.ID != req.ID || decoded.Command != req.Command {
		t.Errorf("decoded message mismatch: got %+v, want %+v", decoded, req)
	}
}

func TestExecFrameEncoding(t *testing.T) {
	frame := ExecFrame{Version: ProtocolVersion, ID: "stream-1", Stream: "stdout", Data: "hello"}
	encoded, err := EncodeMessage(frame)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ExecFrame
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Stream != "stdout" || decoded.Data != "hello" || decoded.ID != frame.ID {
		t.Fatalf("decoded frame = %+v", decoded)
	}
}
