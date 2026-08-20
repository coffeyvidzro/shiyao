package vm

import "testing"

func TestGuestKernelArgsAddsAgentInit(t *testing.T) {
	got := guestKernelArgs("console=ttyS0 reboot=k", "/usr/local/bin/shiyao-agent")
	want := "console=ttyS0 reboot=k init=/usr/local/bin/shiyao-agent"
	if got != want {
		t.Fatalf("guestKernelArgs() = %q, want %q", got, want)
	}
}

func TestGuestKernelArgsPreservesExplicitInit(t *testing.T) {
	got := guestKernelArgs("console=ttyS0 init=/sbin/init", "/usr/local/bin/shiyao-agent")
	want := "console=ttyS0 init=/sbin/init"
	if got != want {
		t.Fatalf("guestKernelArgs() = %q, want %q", got, want)
	}
}
