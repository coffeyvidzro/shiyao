package vm

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/coreos/go-iptables/iptables"
)

// SetupFirewall installs per-VM forwarding and NAT rules.
//
// Traffic originating from the VM is passed through a dedicated chain.
// The chain allows established/related traffic, DNS, ICMP, and explicitly
// allowed TCP ports, then drops everything else.
//
// Return traffic is explicitly allowed when it is established/related and
// is leaving through this VM's TAP interface.
func SetupFirewall(ctx context.Context, cfg NetworkConfig) error {
	_ = ctx

	if cfg.TapName == "" {
		return fmt.Errorf("tap name is empty")
	}

	if cfg.UplinkInterface == "" {
		return fmt.Errorf("uplink interface is empty")
	}

	_, guestNet, err := net.ParseCIDR(cfg.GuestIP)
	if err != nil {
		return fmt.Errorf("parse guest network %q: %w", cfg.GuestIP, err)
	}

	ipt, err := iptables.New()
	if err != nil {
		return fmt.Errorf("init iptables: %w", err)
	}

	chain := firewallChainName(cfg.TapName)

	// Create a dedicated chain for this VM.
	if err := ipt.NewChain("filter", chain); err != nil {
		return fmt.Errorf("create chain %s: %w", chain, err)
	}

	// If anything below fails, remove everything installed so far.
	cleanup := func() {
		_ = ipt.Delete(
			"filter",
			"FORWARD",
			"-i", cfg.TapName,
			"-j", chain,
		)

		_ = ipt.Delete(
			"filter",
			"FORWARD",
			"-o", cfg.TapName,
			"-m", "conntrack",
			"--ctstate", "ESTABLISHED,RELATED",
			"-j", "ACCEPT",
		)

		_ = ipt.Delete(
			"nat",
			"POSTROUTING",
			"-s", guestNet.String(),
			"-o", cfg.UplinkInterface,
			"-j", "MASQUERADE",
		)

		_ = ipt.ClearChain("filter", chain)
		_ = ipt.DeleteChain("filter", chain)
	}

	// Established connections originating from the guest are accepted.
	if err := ipt.Append(
		"filter",
		chain,
		"-m", "conntrack",
		"--ctstate", "ESTABLISHED,RELATED",
		"-j", "ACCEPT",
	); err != nil {
		cleanup()
		return fmt.Errorf("allow established traffic: %w", err)
	}

	// DNS.
	if err := ipt.Append(
		"filter",
		chain,
		"-p", "udp",
		"--dport", "53",
		"-j", "ACCEPT",
	); err != nil {
		cleanup()
		return fmt.Errorf("allow UDP DNS: %w", err)
	}

	if err := ipt.Append(
		"filter",
		chain,
		"-p", "tcp",
		"--dport", "53",
		"-j", "ACCEPT",
	); err != nil {
		cleanup()
		return fmt.Errorf("allow TCP DNS: %w", err)
	}

	// ICMP.
	if err := ipt.Append(
		"filter",
		chain,
		"-p", "icmp",
		"-j", "ACCEPT",
	); err != nil {
		cleanup()
		return fmt.Errorf("allow ICMP: %w", err)
	}

	// Explicitly allowed TCP ports.
	for _, port := range cfg.AllowedPorts {
		if port < 1 || port > 65535 {
			cleanup()
			return fmt.Errorf("invalid allowed TCP port %d", port)
		}

		if err := ipt.Append(
			"filter",
			chain,
			"-p", "tcp",
			"--dport", fmt.Sprintf("%d", port),
			"-j", "ACCEPT",
		); err != nil {
			cleanup()
			return fmt.Errorf("allow TCP port %d: %w", port, err)
		}
	}

	// Default deny for traffic originating from this VM.
	if err := ipt.Append(
		"filter",
		chain,
		"-j", "DROP",
	); err != nil {
		cleanup()
		return fmt.Errorf("add default drop: %w", err)
	}

	// Send guest-originated packets through the VM-specific chain.
	if err := ipt.Append(
		"filter",
		"FORWARD",
		"-i", cfg.TapName,
		"-j", chain,
	); err != nil {
		cleanup()
		return fmt.Errorf("attach guest ingress rule: %w", err)
	}

	// Allow return traffic to this VM.
	if err := ipt.Append(
		"filter",
		"FORWARD",
		"-o", cfg.TapName,
		"-m", "conntrack",
		"--ctstate", "ESTABLISHED,RELATED",
		"-j", "ACCEPT",
	); err != nil {
		cleanup()
		return fmt.Errorf("allow return traffic: %w", err)
	}

	// NAT only traffic belonging to this VM's guest network.
	if err := ipt.Append(
		"nat",
		"POSTROUTING",
		"-s", guestNet.String(),
		"-o", cfg.UplinkInterface,
		"-j", "MASQUERADE",
	); err != nil {
		cleanup()
		return fmt.Errorf("add NAT rule: %w", err)
	}

	return nil
}

// CleanupFirewall removes only this VM's firewall and NAT rules.
func CleanupFirewall(ctx context.Context, cfg NetworkConfig) error {
	_ = ctx

	ipt, err := iptables.New()
	if err != nil {
		return fmt.Errorf("init iptables: %w", err)
	}

	chain := firewallChainName(cfg.TapName)

	_, guestNet, err := net.ParseCIDR(cfg.GuestIP)
	if err != nil {
		return fmt.Errorf("parse guest network %q: %w", cfg.GuestIP, err)
	}

	var errs []string

	if err := ipt.Delete(
		"filter",
		"FORWARD",
		"-i", cfg.TapName,
		"-j", chain,
	); err != nil && !isRuleNotFound(err) {
		errs = append(errs, fmt.Sprintf("delete ingress rule: %v", err))
	}

	if err := ipt.Delete(
		"filter",
		"FORWARD",
		"-o", cfg.TapName,
		"-m", "conntrack",
		"--ctstate", "ESTABLISHED,RELATED",
		"-j", "ACCEPT",
	); err != nil && !isRuleNotFound(err) {
		errs = append(errs, fmt.Sprintf("delete return rule: %v", err))
	}

	if err := ipt.Delete(
		"nat",
		"POSTROUTING",
		"-s", guestNet.String(),
		"-o", cfg.UplinkInterface,
		"-j", "MASQUERADE",
	); err != nil && !isRuleNotFound(err) {
		errs = append(errs, fmt.Sprintf("delete NAT rule: %v", err))
	}

	if err := ipt.ClearChain("filter", chain); err != nil && !isChainNotFound(err) {
		errs = append(errs, fmt.Sprintf("clear chain: %v", err))
	}

	if err := ipt.DeleteChain("filter", chain); err != nil && !isChainNotFound(err) {
		errs = append(errs, fmt.Sprintf("delete chain: %v", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("firewall cleanup failed: %s", strings.Join(errs, "; "))
	}

	return nil
}

func firewallChainName(tapName string) string {
	return fmt.Sprintf("SHIYAO_%s", tapName)
}

func isRuleNotFound(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "does a matching rule exist")
}

func isChainNotFound(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "no chain")
}
