package proxy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"
)

// SafeTransport prevents proxy egress from reaching loopback, private,
// link-local, multicast, unspecified, or carrier-grade NAT addresses. DNS is
// resolved for every connection and the selected IP is dialed directly, which
// prevents DNS rebinding between validation and connect.
func SafeTransport() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	t.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil { return nil, fmt.Errorf("invalid upstream address: %w", err) }
		ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
		if err != nil { return nil, fmt.Errorf("resolve upstream %q: %w", host, err) }
		for _, ip := range ips {
			addr, ok := netip.ParseAddr(ip.String())
			if !ok || !isAllowedPublicIP(addr) { continue }
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(addr.String(), port))
			if err == nil { return conn, nil }
		}
		return nil, fmt.Errorf("upstream %q resolves only to blocked or unreachable addresses", host)
	}
	return t
}

func isAllowedPublicIP(addr netip.Addr) bool {
	if !addr.IsValid() || !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsMulticast() || addr.IsUnspecified() { return false }
	// RFC 6598 shared address space is not suitable for untrusted proxy egress.
	if addr.Is4() {
		n := addr.As4()
		if n[0] == 100 && n[1] >= 64 && n[1] <= 127 { return false }
	}
	return true
}

// ValidateProxyTarget applies the same destination policy before an HTTP
// CONNECT or application-level proxy request is forwarded.
func ValidateProxyTarget(host string, port int) error {
	if host == "" || port < 1 || port > 65535 { return fmt.Errorf("invalid proxy target") }
	if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".localhost") { return fmt.Errorf("localhost target is blocked") }
	if ip := net.ParseIP(host); ip != nil {
		addr, ok := netip.ParseAddr(ip.String())
		if !ok || !isAllowedPublicIP(addr) { return fmt.Errorf("private or non-global target %s is blocked", host) }
		return nil
	}
	return nil
}

func TargetAddress(host string, port int) string { return net.JoinHostPort(host, strconv.Itoa(port)) }
