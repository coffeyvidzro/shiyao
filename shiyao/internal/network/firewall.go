package network

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/coreos/go-iptables/iptables"
)

var firewallMu sync.Mutex

const (
	HostProxyPort   = 8080
	CloudMetadataIP = "169.254.169.254/32"
)

func SetupFirewall(ctx context.Context, cfg Config) error {
	_ = ctx
	if cfg.TapName == "" {
		return fmt.Errorf("tap name is empty")
	}
	if cfg.HostIP == "" {
		return fmt.Errorf("host IP is empty")
	}
	if strings.Contains(cfg.HostIP, ":") {
		return fmt.Errorf("IPv6 host gateways are not supported")
	}
	firewallMu.Lock()
	defer firewallMu.Unlock()
	ipt, err := iptables.New()
	if err != nil {
		return fmt.Errorf("init iptables: %w", err)
	}
	chain := firewallChainName(cfg.TapName)
	if err := ipt.NewChain("filter", chain); err != nil {
		return fmt.Errorf("create chain %s: %w", chain, err)
	}
	cleanup := func() {
		_ = ipt.Delete("filter", "FORWARD", "-i", cfg.TapName, "-j", chain)
		_ = ipt.Delete("filter", "FORWARD", "-o", cfg.TapName, "-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED", "-j", "ACCEPT")
		_ = ipt.ClearChain("filter", chain)
		_ = ipt.DeleteChain("filter", chain)
	}
	if err := ipt.Append("filter", chain, "-d", CloudMetadataIP, "-j", "DROP"); err != nil { cleanup(); return fmt.Errorf("add metadata block rule: %w", err) }
	if err := ipt.Append("filter", chain, "-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED", "-j", "ACCEPT"); err != nil { cleanup(); return fmt.Errorf("allow established traffic: %w", err) }
	if err := ipt.Append("filter", chain, "-p", "udp", "-d", cfg.HostIP, "--dport", "53", "-j", "ACCEPT"); err != nil { cleanup(); return fmt.Errorf("allow host UDP DNS: %w", err) }
	if err := ipt.Append("filter", chain, "-p", "tcp", "-d", cfg.HostIP, "--dport", fmt.Sprintf("%d", HostProxyPort), "-j", "ACCEPT"); err != nil { cleanup(); return fmt.Errorf("allow proxy port traffic: %w", err) }
	for _, port := range cfg.AllowedPorts {
		if port < 1 || port > 65535 { cleanup(); return fmt.Errorf("invalid allowed port %d", port) }
		if port == HostProxyPort { continue }
		if err := ipt.Append("filter", chain, "-p", "tcp", "-d", cfg.HostIP, "--dport", fmt.Sprintf("%d", port), "-j", "ACCEPT"); err != nil { cleanup(); return fmt.Errorf("allow configured host port %d traffic: %w", port, err) }
	}
	if err := ipt.Append("filter", chain, "-j", "DROP"); err != nil { cleanup(); return fmt.Errorf("add default drop rule: %w", err) }
	if err := ipt.Append("filter", "FORWARD", "-i", cfg.TapName, "-j", chain); err != nil { cleanup(); return fmt.Errorf("attach guest ingress rule: %w", err) }
	if err := ipt.Append("filter", "FORWARD", "-o", cfg.TapName, "-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED", "-j", "ACCEPT"); err != nil { cleanup(); return fmt.Errorf("allow return traffic rule: %w", err) }

	ip6t, err := iptables.NewWithProtocol(iptables.ProtocolIPv6)
	if err != nil { cleanup(); return fmt.Errorf("init ip6tables: %w", err) }
	v6Chain := firewallIPv6ChainName(cfg.TapName)
	if err := ip6t.NewChain("filter", v6Chain); err != nil { cleanup(); return fmt.Errorf("create IPv6 chain %s: %w", v6Chain, err) }
	v6Cleanup := func() { _ = ip6t.Delete("filter", "FORWARD", "-i", cfg.TapName, "-j", v6Chain); _ = ip6t.ClearChain("filter", v6Chain); _ = ip6t.DeleteChain("filter", v6Chain) }
	if err := ip6t.Append("filter", v6Chain, "-j", "DROP"); err != nil { v6Cleanup(); cleanup(); return fmt.Errorf("add IPv6 default drop: %w", err) }
	if err := ip6t.Append("filter", "FORWARD", "-i", cfg.TapName, "-j", v6Chain); err != nil { v6Cleanup(); cleanup(); return fmt.Errorf("attach IPv6 guest rule: %w", err) }
	return nil
}

func CleanupFirewall(ctx context.Context, cfg Config) error {
	_ = ctx
	firewallMu.Lock()
	defer firewallMu.Unlock()
	ipt, err := iptables.New()
	if err != nil { return fmt.Errorf("init iptables: %w", err) }
	chain := firewallChainName(cfg.TapName)
	var errs []string
	if err := ipt.Delete("filter", "FORWARD", "-i", cfg.TapName, "-j", chain); err != nil && !isRuleNotFound(err) { errs = append(errs, fmt.Sprintf("delete ingress hook: %v", err)) }
	if err := ipt.Delete("filter", "FORWARD", "-o", cfg.TapName, "-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED", "-j", "ACCEPT"); err != nil && !isRuleNotFound(err) { errs = append(errs, fmt.Sprintf("delete egress return hook: %v", err)) }
	if err := ipt.ClearChain("filter", chain); err != nil && !isChainNotFound(err) { errs = append(errs, fmt.Sprintf("clear chain %s: %v", chain, err)) }
	if err := ipt.DeleteChain("filter", chain); err != nil && !isChainNotFound(err) { errs = append(errs, fmt.Sprintf("delete chain %s: %v", chain, err)) }
	ip6t, err := iptables.NewWithProtocol(iptables.ProtocolIPv6)
	if err == nil {
		v6Chain := firewallIPv6ChainName(cfg.TapName)
		if err := ip6t.Delete("filter", "FORWARD", "-i", cfg.TapName, "-j", v6Chain); err != nil && !isRuleNotFound(err) { errs = append(errs, fmt.Sprintf("delete IPv6 hook: %v", err)) }
		if err := ip6t.ClearChain("filter", v6Chain); err != nil && !isChainNotFound(err) { errs = append(errs, fmt.Sprintf("clear IPv6 chain %s: %v", v6Chain, err)) }
		if err := ip6t.DeleteChain("filter", v6Chain); err != nil && !isChainNotFound(err) { errs = append(errs, fmt.Sprintf("delete IPv6 chain %s: %v", v6Chain, err)) }
	} else { errs = append(errs, fmt.Sprintf("init ip6tables during cleanup: %v", err)) }
	if len(errs) > 0 { return fmt.Errorf("firewall cleanup failed: %s", strings.Join(errs, "; ")) }
	return nil
}

func firewallChainName(tapName string) string { return fmt.Sprintf("SHIYAO_%s", tapName) }
func firewallIPv6ChainName(tapName string) string { return fmt.Sprintf("SHIYAO6_%s", tapName) }
func isRuleNotFound(err error) bool { return err != nil && strings.Contains(strings.ToLower(err.Error()), "does a matching rule exist") }
func isChainNotFound(err error) bool { return err != nil && strings.Contains(strings.ToLower(err.Error()), "no chain") }
