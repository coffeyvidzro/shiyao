package vm

import (
	"testing"
)

func TestFirewallChainName(t *testing.T) {
	tapName := "shy0"
	expected := "SHIYAO_shy0"

	chain := firewallChainName(tapName)
	if chain != expected {
		t.Errorf("expected chain name %q, got %q", expected, chain)
	}
}

func TestNetworkConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     NetworkConfig
		wantErr bool
	}{
		{
			name: "valid network config",
			cfg: NetworkConfig{
				TapName:         "shy0",
				HostIP:          "172.16.0.1",
				GuestIP:         "172.16.0.2/24",
				AllowedPorts:    []int{80, 443},
				UplinkInterface: "eth0",
			},
			wantErr: false,
		},
		{
			name: "missing tap name",
			cfg: NetworkConfig{
				HostIP:          "172.16.0.1",
				GuestIP:         "172.16.0.2/24",
				UplinkInterface: "eth0",
			},
			wantErr: true,
		},
		{
			name: "invalid host ip",
			cfg: NetworkConfig{
				TapName:         "shy0",
				HostIP:          "invalid-ip",
				GuestIP:         "172.16.0.2/24",
				UplinkInterface: "eth0",
			},
			wantErr: true,
		},
		{
			name: "invalid guest CIDR",
			cfg: NetworkConfig{
				TapName:         "shy0",
				HostIP:          "172.16.0.1",
				GuestIP:         "172.16.0.2",
				UplinkInterface: "eth0",
			},
			wantErr: true,
		},
		{
			name: "host IP not in guest network",
			cfg: NetworkConfig{
				TapName:         "shy0",
				HostIP:          "10.0.0.1",
				GuestIP:         "172.16.0.2/24",
				UplinkInterface: "eth0",
			},
			wantErr: true,
		},
		{
			name: "invalid port in allowed ports",
			cfg: NetworkConfig{
				TapName:         "shy0",
				HostIP:          "172.16.0.1",
				GuestIP:         "172.16.0.2/24",
				AllowedPorts:    []int{70000},
				UplinkInterface: "eth0",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("NetworkConfig.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
