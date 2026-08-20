package network

import (
	"context"
	"fmt"

	"github.com/vishvananda/netlink"
)

func SetupTAP(ctx context.Context, cfg Config) error {
	_ = ctx
	tap := &netlink.Tuntap{
		LinkAttrs: netlink.LinkAttrs{Name: cfg.TapName},
		Mode:      netlink.TUNTAP_MODE_TAP,
		Flags:     netlink.TUNTAP_DEFAULTS,
	}
	if err := netlink.LinkAdd(tap); err != nil {
		return fmt.Errorf("add tap device %s: %w", cfg.TapName, err)
	}
	if err := netlink.LinkSetUp(tap); err != nil {
		if cleanupErr := netlink.LinkDel(tap); cleanupErr != nil {
			return fmt.Errorf("bring up tap device %s: %w (also failed to remove tap: %v)", cfg.TapName, err, cleanupErr)
		}
		return fmt.Errorf("bring up tap device %s: %w", cfg.TapName, err)
	}
	return nil
}

func CleanupTAP(ctx context.Context, tapName string) error {
	_ = ctx
	link, err := netlink.LinkByName(tapName)
	if err != nil {
		if _, ok := err.(netlink.LinkNotFoundError); ok {
			return nil
		}
		return fmt.Errorf("find tap device %s: %w", tapName, err)
	}
	if err := netlink.LinkDel(link); err != nil {
		return fmt.Errorf("remove tap device %s: %w", tapName, err)
	}
	return nil
}
