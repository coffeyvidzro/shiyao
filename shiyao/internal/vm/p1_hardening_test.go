package vm

import "testing"

func TestGuestKernelArgsRejectsInvalidAgentPath(t *testing.T) {
	if _, err := guestKernelArgs("console=ttyS0 init=/bin/sh", "relative-agent"); err == nil {
		t.Fatal("expected relative guest agent path to be rejected")
	}
}

func TestGuestKernelArgsOverridesInit(t *testing.T) {
	got, err := guestKernelArgs("console=ttyS0 init=/bin/sh panic=1", "/usr/local/bin/shiyao-agent")
	if err != nil {
		t.Fatal(err)
	}
	if got != "console=ttyS0 panic=1 init=/usr/local/bin/shiyao-agent" {
		t.Fatalf("unexpected kernel args: %q", got)
	}
}

func TestExecRequestRejectsDangerousEnvironment(t *testing.T) {
	req := ExecRequest{
		Version:   ProtocolVersion,
		ID:        "1",
		Command:   "/bin/true",
		Env:       map[string]string{"LD_PRELOAD": "/tmp/evil.so"},
	}
	if err := req.Validate(); err == nil {
		t.Fatal("expected dangerous environment variable to be rejected")
	}
}

func TestExecRequestRejectsExcessiveTimeout(t *testing.T) {
	req := ExecRequest{
		Version:   ProtocolVersion,
		ID:        "1",
		Command:   "/bin/true",
		TimeoutMS: MaxTimeoutMS + 1,
	}
	if err := req.Validate(); err == nil {
		t.Fatal("expected excessive timeout to be rejected")
	}
}
