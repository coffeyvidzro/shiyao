package network

import "testing"

func TestConfigValidateRejectsIPv6Host(t *testing.T) {
	cfg := DefaultConfig("tap0")
	cfg.HostIP = "fd00::1"

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected IPv6 host IP to be rejected")
	}
}

func TestConfigValidateRejectsIPv6Guest(t *testing.T) {
	cfg := DefaultConfig("tap0")
	cfg.HostIP = "172.16.0.1"
	cfg.GuestIP = "fd00::2/64"

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected IPv6 guest IP to be rejected")
	}
}

func TestConfigValidateAcceptsIPv4(t *testing.T) {
	cfg := DefaultConfig("tap0")

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected default IPv4 configuration to validate: %v", err)
	}
}
