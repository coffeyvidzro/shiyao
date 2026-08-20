package vm

import (
	"context"
	"fmt"
	"strings"

	"github.com/coreos/go-iptables/iptables"
)

const (
	// Default port for host-side authenticating egress proxy (e.g., Envoy, Squid, or Tinyproxy)
	HostProxyPort = 8080
	// Cloud Metadata Service IP (AWS, GCP, Azure, OpenStack)
	CloudMetadataIP = "169.254.169.254/32"
)

// SetupFirewall installs per-VM security rules.
//
// Rules configured:
// 1. Drop traffic targeting 169.254.169.254 (SSRF protection).
// 2. Allow established/related return traffic.
// 3. Allow DNS requests to host interface.
// 4. Allow TCP traffic ONLY to the Host/Gateway Proxy Port.
// 5. Default DROP all direct internet egress attempts.
func SetupFirewall(ctx context.Context, cfg NetworkConfig) error {
	_ = ctx

	if cfg.TapName == "" {
		return fmt.Errorf("tap name is empty")
	}

	ipt, err := iptables.New()
	if err != nil {
		return fmt.Errorf("init iptables: %w", err)
	}

	chain := firewallChainName(cfg.TapName)

	// Create dedicated filter chain for this VM
	if err := ipt.NewChain("filter", chain); err != nil {
		return fmt.Errorf("create chain %s: %w", chain, err)
	}

	// Rollback cleanup helper
	cleanup := func() {
		_ = ipt.Delete("filter", "FORWARD", "-i", cfg.TapName, "-j", chain)
		_ = ipt.Delete(
			"filter",
			"FORWARD",
			"-o", cfg.TapName,
			"-m", "conntrack",
			"--ctstate", "ESTABLISHED,RELATED",
			"-j", "ACCEPT",
		)
		_ = ipt.ClearChain("filter", chain)
		_ = ipt.DeleteChain("filter", chain)
	}

	// 1. RULE: Block Cloud Metadata Endpoint (169.254.169.254) immediately
	if err := ipt.Append("filter", chain, "-d", CloudMetadataIP, "-j", "DROP"); err != nil {
		cleanup()
		return fmt.Errorf("add metadata block rule: %w", err)
	}

	// 2. RULE: Allow Established/Related traffic from guest
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

	// 3. RULE: Allow UDP DNS queries targeting Host IP (172.16.x.1)
	if err := ipt.Append(
		"filter",
		chain,
		"-p", "udp",
		"-d", cfg.HostIP,
		"--dport", "53",
		"-j", "ACCEPT",
	); err != nil {
		cleanup()
		return fmt.Errorf("allow host UDP DNS: %w", err)
	}

	// 4. RULE: Allow TCP connection targeting Host Proxy Port ONLY
	if err := ipt.Append(
		"filter",
		chain,
		"-p", "tcp",
		"-d", cfg.HostIP,
		"--dport", fmt.Sprintf("%d", HostProxyPort),
		"-j", "ACCEPT",
	); err != nil {
		cleanup()
		return fmt.Errorf("allow proxy port traffic: %w", err)
	}

	// 5. RULE: Default DROP for all other egress attempts
	if err := ipt.Append("filter", chain, "-j", "DROP"); err != nil {
		cleanup()
		return fmt.Errorf("add default drop rule: %w", err)
	}

	// 6. ATTACH: Send guest TAP ingress traffic to custom VM chain
	if err := ipt.Append("filter", "FORWARD", "-i", cfg.TapName, "-j", chain); err != nil {
		cleanup()
		return fmt.Errorf("attach guest ingress rule: %w", err)
	}

	// 7. ATTACH: Allow return traffic from host to guest TAP interface
	if err := ipt.Append(
		"filter",
		"FORWARD",
		"-o", cfg.TapName,
		"-m", "conntrack",
		"--ctstate", "ESTABLISHED,RELATED",
		"-j", "ACCEPT",
	); err != nil {
		cleanup()
		return fmt.Errorf("allow return traffic rule: %w", err)
	}

	// NOTE: MASQUERADE NAT rule is intentionally omitted to disable direct guest-to-internet routing.
	return nil
}

// CleanupFirewall removes the VM-specific filtering chain and forward attachments.
func CleanupFirewall(ctx context.Context, cfg NetworkConfig) error {
	_ = ctx

	ipt, err := iptables.New()
	if err != nil {
		return fmt.Errorf("init iptables: %w", err)
	}

	chain := firewallChainName(cfg.TapName)
	var errs []string

	// Delete forward hook rules
	if err := ipt.Delete("filter", "FORWARD", "-i", cfg.TapName, "-j", chain); err != nil && !isRuleNotFound(err) {
		errs = append(errs, fmt.Sprintf("delete ingress hook: %v", err))
	}

	if err := ipt.Delete(
		"filter",
		"FORWARD",
		"-o", cfg.TapName,
		"-m", "conntrack",
		"--ctstate", "ESTABLISHED,RELATED",
		"-j", "ACCEPT",
	); err != nil && !isRuleNotFound(err) {
		errs = append(errs, fmt.Sprintf("delete egress return hook: %v", err))
	}

	// Flush and delete isolated chain
	if err := ipt.ClearChain("filter", chain); err != nil && !isChainNotFound(err) {
		errs = append(errs, fmt.Sprintf("clear chain %s: %v", chain, err))
	}

	if err := ipt.DeleteChain("filter", chain); err != nil && !isChainNotFound(err) {
		errs = append(errs, fmt.Sprintf("delete chain %s: %v", chain, err))
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
