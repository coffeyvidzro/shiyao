package proxy

import (
	"fmt"
	"net"
	"net/netip"
	"strings"
)

func isAllowedPublicIP(addr netip.Addr) bool {
	if !addr.IsValid() || !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsMulticast() || addr.IsUnspecified() {
		return false
	}
	if addr.Is4() {
		n := addr.As4()
		if n[0] == 100 && n[1] >= 64 && n[1] <= 127 {
			return false
		}
	}
	return true
}

// ValidateProxyTarget rejects explicit internal destinations. Hostnames are
// resolved by SafeTransport at connect time so DNS answers cannot change after
// a separate validation step.
func ValidateProxyTarget(host string, port int) error {
	if host == "" || port < 1 || port > 65535 {
		return fmt.Errorf("invalid proxy target")
	}
	if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".localhost") {
		return fmt.Errorf("localhost target is blocked")
	}
	if ip := net.ParseIP(host); ip != nil {
		addr, err := netip.ParseAddr(ip.String())
		if err != nil || !isAllowedPublicIP(addr) {
			return fmt.Errorf("private or non-global target %s is blocked", host)
		}
	}
	return nil
}
