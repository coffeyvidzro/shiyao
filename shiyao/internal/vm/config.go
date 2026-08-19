package vm

import (
	"fmt"
	"net"
)

// Config contains the core VM runtime configuration.
type Config struct {
	KernelPath string
	RootfsPath string
	VCPUCount  int
	MemSizeMB  int
	BootArgs   string
}

// NetworkConfig describes the TAP interface and egress policy.
type NetworkConfig struct {
	TapName         string
	HostIP          string // e.g. "172.16.0.1"
	GuestIP         string // e.g. "172.16.0.2/24"
	AllowedPorts    []int  // Whitelisted outbound TCP ports
	UplinkInterface string // Host interface used for Internet egress, e.g. "eth0" or "ens3"
}

// VsockConfig describes the VirtIO VSOCK setup for host-guest communication.
type VsockConfig struct {
	GuestCID uint32 // Context ID for the guest (must be > 2)
}

// DefaultConfig builds a sane default runtime configuration.
func DefaultConfig() Config {
	return Config{
		VCPUCount: 2,
		MemSizeMB: 512,
		BootArgs:  "console=ttyS0 reboot=k panic=1 pci=off",
	}
}

// DefaultNetworkConfig builds a standard isolated network configuration.
func DefaultNetworkConfig(tapName string, guestCID uint32) NetworkConfig {
	return NetworkConfig{
		TapName:         tapName,
		HostIP:          "172.16.0.1",
		GuestIP:         "172.16.0.2/24",
		AllowedPorts:    []int{443, 80, 53},
		UplinkInterface: "eth0",
	}
}

// Validate checks if the core configuration is valid.
func (c *Config) Validate() error {
	if c.KernelPath == "" {
		return fmt.Errorf("kernel path is required")
	}

	if c.RootfsPath == "" {
		return fmt.Errorf("rootfs path is required")
	}

	if c.VCPUCount <= 0 {
		return fmt.Errorf("vcpu count must be greater than 0")
	}

	if c.MemSizeMB <= 0 {
		return fmt.Errorf("memory size must be greater than 0")
	}

	return nil
}

// Validate checks the network configuration.
func (n *NetworkConfig) Validate() error {
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

	// Make sure the guest IP actually belongs to the supplied network.
	if !guestNet.Contains(guestIP) {
		return fmt.Errorf("guest IP %q is not valid for network %q", guestIP, guestNet)
	}

	if !guestNet.Contains(hostIP) {
		return fmt.Errorf(
			"host IP %q is not in guest network %q",
			n.HostIP,
			guestNet.String(),
		)
	}

	for _, port := range n.AllowedPorts {
		if port < 1 || port > 65535 {
			return fmt.Errorf("invalid allowed port %d", port)
		}
	}

	return nil
}

// Validate checks the VSOCK configuration.
func (v *VsockConfig) Validate() error {
	if v.GuestCID <= 2 {
		return fmt.Errorf("guest CID must be greater than 2")
	}

	return nil
}
