package proxy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"time"
)

// SafeTransport resolves every upstream hostname immediately before dialing
// and dials the selected validated IP directly, preventing DNS rebinding from
// turning a previously public hostname into an internal destination.
func SafeTransport() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	t.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("invalid upstream address: %w", err)
		}
		ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
		if err != nil {
			return nil, fmt.Errorf("resolve upstream %q: %w", host, err)
		}
		for _, ip := range ips {
			addr, err := netip.ParseAddr(ip.String())
			if err != nil || !isAllowedPublicIP(addr) {
				continue
			}
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(addr.String(), port))
			if err == nil {
				return conn, nil
			}
		}
		return nil, fmt.Errorf("upstream %q resolves only to blocked or unreachable addresses", host)
	}
	return t
}
