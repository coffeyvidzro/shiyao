package vm

import (
	"context"
	"fmt"
	"log"

	"github.com/coreos/go-iptables/iptables"
	"github.com/vishvananda/netlink"
)

// SetupNetwork creates the TAP device and applies egress rules.
// It returns a cleanup function that MUST be called when the VM is destroyed.
func SetupNetwork(ctx context.Context, cfg NetworkConfig) (cleanup func() error, err error) {
	if cfg.TapName == "" {
		return nil, fmt.Errorf("tap device name is required")
	}

	// 1. Create TAP device
	if err := SetupTAP(ctx, cfg); err != nil {
		return nil, fmt.Errorf("setup tap: %w", err)
	}

	// 2. Apply Egress Rules
	if err := ApplyEgressRules(ctx, cfg); err != nil {
		// Rollback TAP if rules fail
		CleanupTAP(ctx, cfg.TapName)
		return nil, fmt.Errorf("apply egress: %w", err)
	}

	// Return the cleanup closure
	cleanup = func() error {
		RemoveEgressRules(ctx, cfg)
		return CleanupTAP(ctx, cfg.TapName)
	}
	return cleanup, nil
}

// SetupTAP creates and brings up a TAP network interface.
func SetupTAP(ctx context.Context, cfg NetworkConfig) error {
	tap := &netlink.Tuntap{
		LinkAttrs: netlink.LinkAttrs{Name: cfg.TapName},
		Mode:      netlink.TUNTAP_MODE_TAP,
		Flags:     netlink.TUNTAP_DEFAULTS,
	}
	if err := netlink.LinkAdd(tap); err != nil {
		return fmt.Errorf("add tap device %s: %w", cfg.TapName, err)
	}
	if err := netlink.LinkSetUp(tap); err != nil {
		return fmt.Errorf("bring up tap device %s: %w", cfg.TapName, err)
	}
	return nil
}

// CleanupTAP removes the TAP network interface.
func CleanupTAP(ctx context.Context, tapName string) error {
	link, err := netlink.LinkByName(tapName)
	if err != nil {
		return nil // Already gone
	}
	return netlink.LinkDel(link)
}

// ApplyEgressRules installs firewall rules to limit outbound traffic.
// CRITICAL: Rules are evaluated top-to-bottom. ACCEPT rules must come BEFORE the final DROP.
func ApplyEgressRules(ctx context.Context, cfg NetworkConfig) error {
	ipt, err := iptables.New()
	if err != nil {
		return fmt.Errorf("init iptables: %w", err)
	}

	chain := fmt.Sprintf("SHIYAO_%s", cfg.TapName)

	// 1. Create a custom chain for this VM
	if err := ipt.NewChain("filter", chain); err != nil {
		return fmt.Errorf("create chain %s: %w", chain, err)
	}

	// --- CORRECT ORDER: ACCEPT first, DROP last ---

	// 2. Allow established/related return traffic (Crucial for HTTP responses)
	if err := ipt.Append("filter", chain, "-m", "state", "--state", "ESTABLISHED,RELATED", "-j", "ACCEPT"); err != nil {
		return err
	}

	// 3. Allow Loopback (Recommended for internal agent tools)
	if err := ipt.Append("filter", chain, "-o", "lo", "-j", "ACCEPT"); err != nil {
		return err
	}

	// 4. Allow DNS (UDP & TCP 53) - AI Agents MUST resolve domains
	if err := ipt.Append("filter", chain, "-p", "udp", "--dport", "53", "-j", "ACCEPT"); err != nil {
		return err
	}
	if err := ipt.Append("filter", chain, "-p", "tcp", "--dport", "53", "-j", "ACCEPT"); err != nil {
		return err
	}

	// 5. Allow ICMP (Ping)
	if err := ipt.Append("filter", chain, "-p", "icmp", "-j", "ACCEPT"); err != nil {
		return err
	}

	// 6. Allow specific whitelisted ports
	for _, port := range cfg.AllowedPorts {
		if err := ipt.Append("filter", chain, "-p", "tcp", "--dport", fmt.Sprintf("%d", port), "-j", "ACCEPT"); err != nil {
			return err
		}
	}

	// 7. DEFAULT DROP: Catch-all at the very end (The "Stone" security boundary)
	if err := ipt.Append("filter", chain, "-j", "DROP"); err != nil {
		return err
	}

	// 8. Attach our custom chain to the FORWARD chain for this TAP device
	if err := ipt.Append("filter", "FORWARD", "-i", cfg.TapName, "-j", chain); err != nil {
		return err
	}

	// 9. NAT / Masquerade (Crucial for outbound internet access)
	// This translates the VM's private IP to the Host's public IP.
	// Note: "eth0" is the default host interface. In production, detect the default gateway interface.
	if err := ipt.Append("nat", "POSTROUTING", "-o", "eth0", "-j", "MASQUERADE"); err != nil {
		log.Printf("Warning: Could not add MASQUERADE rule, VM may not reach internet: %v", err)
	}

	return nil
}

// RemoveEgressRules cleans up the firewall rules and NAT configuration.
func RemoveEgressRules(ctx context.Context, cfg NetworkConfig) error {
	ipt, err := iptables.New()
	if err != nil {
		return err
	}
	chain := fmt.Sprintf("SHIYAO_%s", cfg.TapName)

	// Detach from FORWARD (Ignore error if it doesn't exist)
	_ = ipt.Delete("filter", "FORWARD", "-i", cfg.TapName, "-j", chain)

	// Remove NAT rule (Ignore error)
	_ = ipt.Delete("nat", "POSTROUTING", "-o", "eth0", "-j", "MASQUERADE")

	// Flush and delete chain
	_ = ipt.ClearChain("filter", chain)
	_ = ipt.DeleteChain("filter", chain)

	return nil
}
