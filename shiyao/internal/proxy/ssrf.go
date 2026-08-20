package proxy

import (
	"net"
	"strconv"
)

func TargetAddress(host string, port int) string {
	return net.JoinHostPort(host, strconv.Itoa(port))
}
