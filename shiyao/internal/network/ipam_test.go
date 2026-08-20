package network

import (
	"fmt"
	"testing"
)

func TestIPAMPool_AllocateAndRelease(t *testing.T) {
	ipam := NewIPAMPool()
	baseNet := Config{UplinkInterface: "eth0"}
	netCfg1, cid1, err := ipam.Allocate("vm-1", baseNet)
	if err != nil { t.Fatalf("unexpected allocation error: %v", err) }
	if cid1 < MinGuestCID || cid1 > MaxGuestCID { t.Errorf("expected CID between %d and %d, got %d", MinGuestCID, MaxGuestCID, cid1) }
	if netCfg1.HostIP == "" || netCfg1.GuestIP == "" { t.Errorf("invalid allocated IPs: host=%q, guest=%q", netCfg1.HostIP, netCfg1.GuestIP) }
	netCfg2, cid2, err := ipam.Allocate("vm-2", baseNet)
	if err != nil { t.Fatalf("unexpected allocation error: %v", err) }
	if cid1 == cid2 { t.Errorf("expected distinct CIDs, got %d for both", cid1) }
	if netCfg1.HostIP == netCfg2.HostIP { t.Errorf("expected distinct host IPs, got %q", netCfg1.HostIP) }
	ipam.Release(netCfg1.GuestIP, cid1)
	_, cid3, err := ipam.Allocate("vm-3", baseNet)
	if err != nil { t.Fatalf("unexpected allocation error: %v", err) }
	if cid3 != cid1 { t.Logf("allocated CID %d after releasing %d", cid3, cid1) }
}

func TestIPAMPool_Exhaustion(t *testing.T) {
	ipam := NewIPAMPool()
	baseNet := Config{UplinkInterface: "eth0"}
	for i := 0; i < MaxSubnets; i++ {
		if _, _, err := ipam.Allocate(fmt.Sprintf("vm-exhaust-%d", i), baseNet); err != nil { t.Fatalf("unexpected failure at index %d: %v", i, err) }
	}
	if _, _, err := ipam.Allocate("vm-overflow", baseNet); err == nil { t.Fatal("expected error due to pool exhaustion, got nil") }
}
