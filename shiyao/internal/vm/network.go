package vm

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/coreos/go-iptables/iptables"
	"github.com/vishvananda/netlink"
)

// NetworkConfig contains the host and guest networking settings for a microVM.
type NetworkConfig struct {
	TapName       string
	HostIP        string
	GuestIP       string
	HostInterface string
	AllowedPorts  []int
}

// DefaultNetworkConfig returns a NAT-based network configuration matching the
// Firecracker documentation's single-guest TAP layout.
func DefaultNetworkConfig() NetworkConfig {
	return NetworkConfig{
		TapName:       "tap0",
		HostIP:        "172.16.0.1/30",
		GuestIP:       "172.16.0.2/30",
		HostInterface: "eth0",
		AllowedPorts:  []int{80, 443},
	}
}

// Validate checks whether the network configuration can be applied.
func (c NetworkConfig) Validate() error {
	if c.TapName == "" {
		return fmt.Errorf("tap device name is required")
	}
	if c.HostIP == "" {
		return fmt.Errorf("host tap IP is required")
	}
	if _, _, err := net.ParseCIDR(c.HostIP); err != nil {
		return fmt.Errorf("host tap IP must be CIDR notation: %w", err)
	}
	if c.GuestIP == "" {
		return fmt.Errorf("guest IP is required")
	}
	if _, _, err := net.ParseCIDR(c.GuestIP); err != nil {
		return fmt.Errorf("guest IP must be CIDR notation: %w", err)
	}
	if c.HostInterface == "" {
		return fmt.Errorf("host interface is required")
	}
	for _, port := range c.AllowedPorts {
		if port <= 0 || port > 65535 {
			return fmt.Errorf("allowed port %d is outside valid TCP port range", port)
		}
	}
	return nil
}

// SetupNetwork creates the TAP device and applies egress rules.
// It returns a cleanup function that MUST be called when the VM is destroyed.
func SetupNetwork(ctx context.Context, cfg NetworkConfig) (cleanup func() error, err error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	if err := SetupTAP(ctx, cfg); err != nil {
		return nil, fmt.Errorf("setup tap: %w", err)
	}

	if err := ApplyEgressRules(ctx, cfg); err != nil {
		CleanupTAP(ctx, cfg.TapName)
		return nil, fmt.Errorf("apply egress: %w", err)
	}

	cleanup = func() error {
		RemoveEgressRules(ctx, cfg)
		return CleanupTAP(ctx, cfg.TapName)
	}
	return cleanup, nil
}

// SetupTAP creates and brings up a TAP network interface.
func SetupTAP(ctx context.Context, cfg NetworkConfig) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	addr, err := netlink.ParseAddr(cfg.HostIP)
	if err != nil {
		return fmt.Errorf("parse host tap IP %s: %w", cfg.HostIP, err)
	}

	tap := &netlink.Tuntap{
		LinkAttrs: netlink.LinkAttrs{Name: cfg.TapName},
		Mode:      netlink.TUNTAP_MODE_TAP,
		Flags:     netlink.TUNTAP_DEFAULTS,
	}
	if err := netlink.LinkAdd(tap); err != nil {
		return fmt.Errorf("add tap device %s: %w", cfg.TapName, err)
	}
	if err := netlink.AddrAdd(tap, addr); err != nil {
		_ = netlink.LinkDel(tap)
		return fmt.Errorf("assign host tap IP %s to %s: %w", cfg.HostIP, cfg.TapName, err)
	}
	if err := netlink.LinkSetUp(tap); err != nil {
		_ = netlink.LinkDel(tap)
		return fmt.Errorf("bring up tap device %s: %w", cfg.TapName, err)
	}
	return nil
}

// CleanupTAP removes the TAP network interface.
func CleanupTAP(ctx context.Context, tapName string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	link, err := netlink.LinkByName(tapName)
	if err != nil {
		return nil
	}
	return netlink.LinkDel(link)
}

// ApplyEgressRules installs firewall rules to limit outbound traffic.
func ApplyEgressRules(ctx context.Context, cfg NetworkConfig) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	ipt, err := iptables.New()
	if err != nil {
		return fmt.Errorf("init iptables: %w", err)
	}

	chain := fmt.Sprintf("SHIYAO_%s", cfg.TapName)

	if err := ipt.NewChain("filter", chain); err != nil {
		return fmt.Errorf("create chain %s: %w", chain, err)
	}
	if err := ipt.Append("filter", chain, "-m", "state", "--state", "ESTABLISHED,RELATED", "-j", "ACCEPT"); err != nil {
		return err
	}
	if err := ipt.Append("filter", chain, "-o", "lo", "-j", "ACCEPT"); err != nil {
		return err
	}
	if err := ipt.Append("filter", chain, "-p", "udp", "--dport", "53", "-j", "ACCEPT"); err != nil {
		return err
	}
	if err := ipt.Append("filter", chain, "-p", "tcp", "--dport", "53", "-j", "ACCEPT"); err != nil {
		return err
	}
	if err := ipt.Append("filter", chain, "-p", "icmp", "-j", "ACCEPT"); err != nil {
		return err
	}
	for _, port := range cfg.AllowedPorts {
		if err := ipt.Append("filter", chain, "-p", "tcp", "--dport", fmt.Sprintf("%d", port), "-j", "ACCEPT"); err != nil {
			return err
		}
	}
	if err := ipt.Append("filter", chain, "-j", "DROP"); err != nil {
		return err
	}
	if err := ipt.Append("filter", "FORWARD", "-i", cfg.TapName, "-j", chain); err != nil {
		return err
	}
	if err := ipt.Append("nat", "POSTROUTING", "-o", cfg.HostInterface, "-j", "MASQUERADE"); err != nil {
		log.Printf("Warning: Could not add MASQUERADE rule, VM may not reach internet: %v", err)
	}

	return nil
}

// RemoveEgressRules cleans up the firewall rules and NAT configuration.
func RemoveEgressRules(ctx context.Context, cfg NetworkConfig) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	ipt, err := iptables.New()
	if err != nil {
		return err
	}
	chain := fmt.Sprintf("SHIYAO_%s", cfg.TapName)
	_ = ipt.Delete("filter", "FORWARD", "-i", cfg.TapName, "-j", chain)
	_ = ipt.Delete("nat", "POSTROUTING", "-o", cfg.HostInterface, "-j", "MASQUERADE")
	_ = ipt.ClearChain("filter", chain)
	_ = ipt.DeleteChain("filter", chain)

	return nil
}
