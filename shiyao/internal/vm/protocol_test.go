package vm

import "testing"

func TestExecRequestValidate(t *testing.T) {
	tests := []struct {
		name string
		req  ExecRequest
		want bool
	}{
		{
			name: "valid",
			req: ExecRequest{Version: ProtocolVersion, ID: "1", Command: "/bin/echo"},
			want: true,
		},
		{
			name: "missing id",
			req: ExecRequest{Version: ProtocolVersion, Command: "/bin/echo"},
		},
		{
			name: "missing command",
			req: ExecRequest{Version: ProtocolVersion, ID: "1"},
		},
		{
			name: "wrong version",
			req: ExecRequest{Version: 99, ID: "1", Command: "/bin/echo"},
		},
		{
			name: "negative timeout",
			req: ExecRequest{Version: ProtocolVersion, ID: "1", Command: "/bin/echo", TimeoutMS: -1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.Validate() == nil
			if got != tt.want {
				t.Fatalf("Validate() = %v, want %v", got, tt.want)
			}
		})
	}
}
