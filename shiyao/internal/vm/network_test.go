package vm

import "testing"

func TestDefaultNetworkConfig(t *testing.T) {
	cfg := DefaultNetworkConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default network config should validate: %v", err)
	}
	if cfg.HostInterface != "eth0" {
		t.Fatalf("expected eth0 default host interface, got %q", cfg.HostInterface)
	}
}

func TestNetworkConfigValidate(t *testing.T) {
	tests := []struct {
		name string
		cfg  NetworkConfig
	}{
		{name: "missing tap", cfg: NetworkConfig{HostIP: "172.16.0.1/30", GuestIP: "172.16.0.2/30", HostInterface: "eth0"}},
		{name: "missing host ip", cfg: NetworkConfig{TapName: "tap0", GuestIP: "172.16.0.2/30", HostInterface: "eth0"}},
		{name: "missing guest ip", cfg: NetworkConfig{TapName: "tap0", HostIP: "172.16.0.1/30", HostInterface: "eth0"}},
		{name: "missing host interface", cfg: NetworkConfig{TapName: "tap0", HostIP: "172.16.0.1/30", GuestIP: "172.16.0.2/30"}},
		{name: "bad host ip", cfg: NetworkConfig{TapName: "tap0", HostIP: "172.16.0.1", GuestIP: "172.16.0.2/30", HostInterface: "eth0"}},
		{name: "bad guest ip", cfg: NetworkConfig{TapName: "tap0", HostIP: "172.16.0.1/30", GuestIP: "172.16.0.2", HostInterface: "eth0"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.cfg.Validate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
