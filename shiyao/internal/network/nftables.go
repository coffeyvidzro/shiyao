package network

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

const (
	nftTable        = "shiyao"
	HostProxyPort   = 8080
	CloudMetadataIP = "169.254.169.254"
)

var (
	nftMu       sync.Mutex
	tapNameExpr = regexp.MustCompile(`^[[:alnum:]_-]{1,15}$`)
	nftRun      = func(ctx context.Context, input string) error {
		cmd := exec.CommandContext(ctx, "nft", "-f", "-")
		cmd.Stdin = strings.NewReader(input)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("run nft: %w: %s", err, strings.TrimSpace(string(output)))
		}
		return nil
	}
	nftTableExists = func(ctx context.Context) bool {
		return exec.CommandContext(ctx, "nft", "list", "table", "inet", nftTable).Run() == nil
	}
)

// SetupFirewall installs a VM's policy as elements in shared nftables sets.
// The base chains are created once, so adding a VM requires only one atomic
// nft transaction instead of creating and linking several iptables chains.
func SetupFirewall(ctx context.Context, cfg Config) error {
	if err := validateNftConfig(cfg); err != nil {
		return err
	}
	nftMu.Lock()
	defer nftMu.Unlock()
	if !nftTableExists(ctx) {
		if err := nftRun(ctx, nftBaseRuleset()); err != nil {
			return fmt.Errorf("initialize nftables policy: %w", err)
		}
	}
	if err := nftRun(ctx, nftAddVM(cfg)); err != nil {
		return fmt.Errorf("add nftables policy for tap %s: %w", cfg.TapName, err)
	}
	return nil
}

// CleanupFirewall removes only the shared-set entries that belong to cfg.
func CleanupFirewall(ctx context.Context, cfg Config) error {
	if err := validateNftConfig(cfg); err != nil {
		return err
	}
	nftMu.Lock()
	defer nftMu.Unlock()
	if err := nftRun(ctx, nftDeleteVM(cfg)); err != nil {
		return fmt.Errorf("remove nftables policy for tap %s: %w", cfg.TapName, err)
	}
	return nil
}

func validateNftConfig(cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if !tapNameExpr.MatchString(cfg.TapName) {
		return fmt.Errorf("invalid tap name %q for nftables", cfg.TapName)
	}
	if ip := net.ParseIP(cfg.HostIP); ip == nil || ip.To4() == nil {
		return fmt.Errorf("IPv4 host gateway is required")
	}
	return nil
}

func nftBaseRuleset() string {
	return `add table inet shiyao
add set inet shiyao guest_taps { type ifname; }
add set inet shiyao guest_tcp_allow { type ifname . ipv4_addr . inet_service; }
add set inet shiyao guest_udp_allow { type ifname . ipv4_addr . inet_service; }
add chain inet shiyao forward { type filter hook forward priority filter; policy accept; }
add rule inet shiyao forward iifname @guest_taps ip daddr 169.254.169.254 drop
add rule inet shiyao forward iifname @guest_taps ct state established,related accept
add rule inet shiyao forward iifname . ip daddr . tcp dport @guest_tcp_allow accept
add rule inet shiyao forward iifname . ip daddr . udp dport @guest_udp_allow accept
add rule inet shiyao forward iifname @guest_taps drop
add rule inet shiyao forward oifname @guest_taps ct state established,related accept
`
}

func nftAddVM(cfg Config) string {
	return nftVMElements("add", cfg)
}

func nftDeleteVM(cfg Config) string {
	return nftVMElements("delete", cfg)
}

func nftVMElements(action string, cfg Config) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s element inet %s guest_taps { %s }\n", action, nftTable, cfg.TapName)
	fmt.Fprintf(&b, "%s element inet %s guest_udp_allow { %s . %s . 53 }\n", action, nftTable, cfg.TapName, cfg.HostIP)
	fmt.Fprintf(&b, "%s element inet %s guest_tcp_allow { %s . %s . %d }\n", action, nftTable, cfg.TapName, cfg.HostIP, HostProxyPort)
	for _, port := range cfg.AllowedPorts {
		if port != HostProxyPort {
			fmt.Fprintf(&b, "%s element inet %s guest_tcp_allow { %s . %s . %s }\n", action, nftTable, cfg.TapName, cfg.HostIP, strconv.Itoa(port))
		}
	}
	return b.String()
}
