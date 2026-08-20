package network

import (
	"fmt"
	"net"
	"sync"
)

const (
	MaxSubnets  = 256
	MinGuestCID = 3
	MaxGuestCID = 1024
)

type IPAMPool struct {
	mu          sync.Mutex
	freeSubnets []uint16
	usedSubnets map[uint16]bool
	freeCIDs    []uint32
	usedCIDs    map[uint32]bool
}

func NewIPAMPool() *IPAMPool {
	p := &IPAMPool{usedSubnets: make(map[uint16]bool), usedCIDs: make(map[uint32]bool)}
	for i := uint16(0); i < MaxSubnets; i++ {
		p.freeSubnets = append(p.freeSubnets, i)
	}
	for cid := uint32(MinGuestCID); cid <= MaxGuestCID; cid++ {
		p.freeCIDs = append(p.freeCIDs, cid)
	}
	return p
}

func (p *IPAMPool) Allocate(vmID string, baseNet Config) (Config, uint32, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.freeSubnets) == 0 {
		return Config{}, 0, fmt.Errorf("ipam: subnet pool exhausted (max %d active VMs)", MaxSubnets)
	}
	if len(p.freeCIDs) == 0 {
		return Config{}, 0, fmt.Errorf("ipam: VSOCK CID pool exhausted")
	}
	subnetIdx := p.freeSubnets[len(p.freeSubnets)-1]
	p.freeSubnets = p.freeSubnets[:len(p.freeSubnets)-1]
	p.usedSubnets[subnetIdx] = true
	cid := p.freeCIDs[len(p.freeCIDs)-1]
	p.freeCIDs = p.freeCIDs[:len(p.freeCIDs)-1]
	p.usedCIDs[cid] = true
	cfg := baseNet
	cfg.HostIP = fmt.Sprintf("172.16.%d.1", subnetIdx)
	cfg.GuestIP = fmt.Sprintf("172.16.%d.2/24", subnetIdx)
	cfg.TapName = uniqueTapName(vmID)
	return cfg, cid, nil
}

func (p *IPAMPool) Release(guestIP string, cid uint32) {
	p.mu.Lock()
	defer p.mu.Unlock()
	ip, _, err := net.ParseCIDR(guestIP)
	if err == nil && ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			subnetIdx := uint16(ip4[2])
			if p.usedSubnets[subnetIdx] {
				delete(p.usedSubnets, subnetIdx)
				p.freeSubnets = append(p.freeSubnets, subnetIdx)
			}
		}
	}
	if p.usedCIDs[cid] {
		delete(p.usedCIDs, cid)
		p.freeCIDs = append(p.freeCIDs, cid)
	}
}

func uniqueTapName(vmID string) string {
	// Keep resource naming internal to network; callers provide validated IDs.
	return fmt.Sprintf("shy%x", vmID)
}
