package network

import (
	"context"
	"errors"
	"net"
	"testing"
)

func TestAllocationReleaseRetainsIPAMUntilCleanupSucceeds(t *testing.T) {
	pool := NewIPAMPool()
	base := DefaultConfig("test-tap")
	allocation, err := Acquire("vm-1", base, pool)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	ip, _, err := net.ParseCIDR(allocation.cfg.GuestIP)
	if err != nil {
		t.Fatalf("parse allocated guest IP: %v", err)
	}
	subnetIdx := uint16(ip.To4()[2])
	cid := allocation.cid

	attempts := 0
	allocation.cleanup = []func(context.Context) error{
		func(context.Context) error {
			attempts++
			if attempts == 1 {
				return errors.New("simulated cleanup failure")
			}
			return nil
		},
	}

	if err := allocation.Release(context.Background()); err == nil {
		t.Fatal("Release succeeded despite simulated cleanup failure")
	}

	if !pool.usedSubnets[subnetIdx] {
		t.Fatalf("subnet %d was returned to IPAM after failed cleanup", subnetIdx)
	}
	if !pool.usedCIDs[cid] {
		t.Fatalf("CID %d was returned to IPAM after failed cleanup", cid)
	}

	if err := allocation.Release(context.Background()); err != nil {
		t.Fatalf("retry Release: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("cleanup attempts = %d, want 2", attempts)
	}
	if pool.usedSubnets[subnetIdx] {
		t.Fatalf("subnet %d remains allocated after successful cleanup", subnetIdx)
	}
	if pool.usedCIDs[cid] {
		t.Fatalf("CID %d remains allocated after successful cleanup", cid)
	}
}
