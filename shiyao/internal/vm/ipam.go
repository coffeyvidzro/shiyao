package vm

import (
	"fmt"
	"net"
	"sync"
)

const (
	// Maximum supported /24 subnets in 172.16.0.0/12 space (0 to 255 for octet 3)
	MaxSubnets = 256
	// CID 0, 1 are reserved; CID 2 is reserved for host
	MinGuestCID = 3
	MaxGuestCID = 1024
)

// IPAMPool safely manages subnets and VSOCK CIDs.
type IPAMPool struct {
	mu          sync.Mutex
	freeSubnets []uint16
	usedSubnets map[uint16]bool
	freeCIDs    []uint32
	usedCIDs    map[uint32]bool
}

// NewIPAMPool initializes a fresh pool with all valid subnets and CIDs available.
func NewIPAMPool() *IPAMPool {
	p := &IPAMPool{
		usedSubnets: make(map[uint16]bool),
		usedCIDs:    make(map[uint32]bool),
	}

	// Populate available subnets (172.16.0.0/24 through 172.16.255.0/24)
	for i := uint16(0); i < MaxSubnets; i++ {
		p.freeSubnets = append(p.freeSubnets, i)
	}

	// Populate available guest CIDs
	for cid := uint32(MinGuestCID); cid <= MaxGuestCID; cid++ {
		p.freeCIDs = append(p.freeCIDs, cid)
	}

	return p
}

// Allocate assigns a unique NetworkConfig and GuestCID atomically.
func (p *IPAMPool) Allocate(vmID string, baseNet NetworkConfig) (NetworkConfig, uint32, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.freeSubnets) == 0 {
		return NetworkConfig{}, 0, fmt.Errorf("ipam: subnet pool exhausted (max %d active VMs)", MaxSubnets)
	}
	if len(p.freeCIDs) == 0 {
		return NetworkConfig{}, 0, fmt.Errorf("ipam: VSOCK CID pool exhausted")
	}

	// Pop subnet index
	subnetIdx := p.freeSubnets[len(p.freeSubnets)-1]
	p.freeSubnets = p.freeSubnets[:len(p.freeSubnets)-1]
	p.usedSubnets[subnetIdx] = true

	// Pop CID
	cid := p.freeCIDs[len(p.freeCIDs)-1]
	p.freeCIDs = p.freeCIDs[:len(p.freeCIDs)-1]
	p.usedCIDs[cid] = true

	netCfg := baseNet
	netCfg.HostIP = fmt.Sprintf("172.16.%d.1", subnetIdx)
	netCfg.GuestIP = fmt.Sprintf("172.16.%d.2/24", subnetIdx)
	netCfg.TapName = uniqueTapName(vmID)

	return netCfg, cid, nil
}

// Release returns an allocated subnet and CID back to the pool for reuse.
func (p *IPAMPool) Release(guestIP string, cid uint32) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Parse guest IP to extract third octet
	ip, _, err := net.ParseCIDR(guestIP)
	if err == nil && ip != nil {
		ip4 := ip.To4()
		if ip4 != nil {
			subnetIdx := uint16(ip4[2])
			if p.usedSubnets[subnetIdx] {
				delete(p.usedSubnets, subnetIdx)
				p.freeSubnets = append(p.freeSubnets, subnetIdx)
			}
		}
	}

	// Recycle CID
	if p.usedCIDs[cid] {
		delete(p.usedCIDs, cid)
		p.freeCIDs = append(p.freeCIDs, cid)
	}
}
