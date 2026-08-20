package vmm

import "testing"

func TestGuestKernelArgsRejectsInvalidAgentPath(t *testing.T) {
	if _, err := guestKernelArgs("console=ttyS0 init=/bin/sh", "relative-agent"); err == nil { t.Fatal("expected relative guest agent path to be rejected") }
}

func TestGuestKernelArgsOverridesInit(t *testing.T) {
	got, err := guestKernelArgs("console=ttyS0 init=/bin/sh panic=1", "/usr/local/bin/shiyao-agent")
	if err != nil { t.Fatal(err) }
	if got != "console=ttyS0 panic=1 init=/usr/local/bin/shiyao-agent" { t.Fatalf("unexpected kernel args: %q", got) }
}
