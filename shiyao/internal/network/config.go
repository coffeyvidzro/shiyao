package network

import (
	"fmt"
	"net"
)

// Config describes the host TAP interface and guest egress policy.
type Config struct {
	TapName         string
	HostIP          string
	GuestIP         string
	AllowedPorts    []int
	UplinkInterface string
}

func DefaultConfig(tapName string) Config {
	return Config{
		TapName:         tapName,
		HostIP:          "172.16.0.1",
		GuestIP:         "172.16.0.2/24",
		AllowedPorts:    []int{443, 80, 53},
		UplinkInterface: "eth0",
	}
}

func (n *Config) Validate() error {
	if n.TapName == "" {
		return fmt.Errorf("tap name is required")
	}
	if n.UplinkInterface == "" {
		return fmt.Errorf("uplink interface is required")
	}
	hostIP := net.ParseIP(n.HostIP)
	if hostIP == nil {
		return fmt.Errorf("invalid host IP %q", n.HostIP)
	}
	guestIP, guestNet, err := net.ParseCIDR(n.GuestIP)
	if err != nil {
		return fmt.Errorf("invalid guest IP/CIDR %q: %w", n.GuestIP, err)
	}
	if !guestNet.Contains(guestIP) {
		return fmt.Errorf("guest IP %q is not valid for network %q", guestIP, guestNet)
	}
	if !guestNet.Contains(hostIP) {
		return fmt.Errorf("host IP %q is not in guest network %q", n.HostIP, guestNet)
	}
	for _, port := range n.AllowedPorts {
		if port < 1 || port > 65535 {
			return fmt.Errorf("invalid allowed port %d", port)
		}
	}
	return nil
}
