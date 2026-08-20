package network

import (
	"strings"
	"testing"
)

func TestNftablesPolicyUsesSharedSets(t *testing.T) {
	cfg := Config{TapName: "shy0", HostIP: "172.16.0.1", GuestIP: "172.16.0.2/24", AllowedPorts: []int{80, 443}, UplinkInterface: "eth0"}
	rules := nftAddVM(cfg)
	for _, want := range []string{"guest_taps", "guest_tcp_allow", "guest_udp_allow", "shy0 . 172.16.0.1 . 443"} {
		if !strings.Contains(rules, want) {
			t.Fatalf("nft policy missing %q: %s", want, rules)
		}
	}
}

func TestConfigValidate(t *testing.T) {
	valid := Config{TapName: "shy0", HostIP: "172.16.0.1", GuestIP: "172.16.0.2/24", AllowedPorts: []int{80, 443}, UplinkInterface: "eth0"}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	invalid := valid
	invalid.AllowedPorts = []int{70000}
	if err := invalid.Validate(); err == nil {
		t.Fatal("expected invalid port to be rejected")
	}
	invalid = valid
	invalid.HostIP = "10.0.0.1"
	if err := invalid.Validate(); err == nil {
		t.Fatal("expected gateway outside guest network to be rejected")
	}
}
