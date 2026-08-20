package network

import "net"

// AllowsTCPDestination describes the firewall contract for configured guest
// egress: AllowedPorts are destinations on the configured host gateway only.
// They do not grant access to arbitrary hosts using the same destination port.
func (c Config) AllowsTCPDestination(destination net.IP, port int) bool {
	gateway := net.ParseIP(c.HostIP)
	if gateway == nil || destination == nil || !gateway.Equal(destination) { return false }
	for _, allowed := range c.AllowedPorts {
		if allowed == port { return true }
	}
	return false
}
