package proxy

import (
	"net/netip"
	"testing"
)

func TestIsAllowedPublicIP(t *testing.T) {
	tests := []struct {
		addr    string
		allowed bool
	}{
		{"1.1.1.1", true},
		{"10.0.0.1", false},
		{"172.16.0.1", false},
		{"192.168.1.1", false},
		{"127.0.0.1", false},
		{"169.254.169.254", false},
		{"100.64.0.1", false},
		{"::1", false},
		{"fc00::1", false},
		{"fe80::1", false},
		{"2001:4860:4860::8888", true},
	}

	for _, tt := range tests {
		addr, err := netip.ParseAddr(tt.addr)
		if err != nil {
			t.Fatalf("parse %s: %v", tt.addr, err)
		}
		if got := isAllowedPublicIP(addr); got != tt.allowed {
			t.Errorf("%s: got %v, want %v", tt.addr, got, tt.allowed)
		}
	}
}
