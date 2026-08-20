package main

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// setupEphemeralRootfs puts an overlay filesystem over the immutable root
// filesystem. The upper and work layers live on tmpfs, so all guest writes
// disappear when the VM stops. The original root mount is retained only until
// pivot_root completes and is then detached.
func setupEphemeralRootfs() error {
	base := "/run/shiyao-overlay"
	upper := filepath.Join(base, "upper")
	work := filepath.Join(base, "work")
	newRoot := filepath.Join(base, "newroot")
	oldRoot := filepath.Join(newRoot, ".oldroot")

	if err := os.MkdirAll(base, 0700); err != nil { return fmt.Errorf("create overlay base: %w", err) }
	if err := unix.Mount("tmpfs", base, "tmpfs", 0, "size=512M,mode=0700"); err != nil { return fmt.Errorf("mount overlay tmpfs: %w", err) }
	cleanup := func() { _ = unix.Unmount(base, unix.MNT_DETACH) }
	for _, p := range []string{upper, work, newRoot, oldRoot} {
		if err := os.MkdirAll(p, 0700); err != nil { cleanup(); return fmt.Errorf("create overlay directory %s: %w", p, err) }
	}

	options := "lowerdir=/,upperdir=" + upper + ",workdir=" + work
	if err := unix.Mount("overlay", newRoot, "overlay", 0, options); err != nil {
		cleanup()
		return fmt.Errorf("mount root overlay: %w", err)
	}
	if err := unix.PivotRoot(newRoot, oldRoot); err != nil {
		_ = unix.Unmount(newRoot, unix.MNT_DETACH)
		cleanup()
		return fmt.Errorf("pivot root to ephemeral overlay: %w", err)
	}
	if err := os.Chdir("/"); err != nil { return fmt.Errorf("chdir new root: %w", err) }

	// Preserve the kernel pseudo-filesystems from the original root mount.
	for _, name := range []string{"proc", "sys", "dev", "run"} {
		src := filepath.Join("/.oldroot", name)
		dst := filepath.Join("/", name)
		if err := os.MkdirAll(dst, 0755); err != nil { return fmt.Errorf("create %s: %w", dst, err) }
		if err := unix.Mount(src, dst, "", unix.MS_MOVE, ""); err != nil {
			return fmt.Errorf("move %s mount: %w", name, err)
		}
	}

	if err := unix.Unmount("/.oldroot", unix.MNT_DETACH); err != nil { return fmt.Errorf("detach immutable root: %w", err) }
	return nil
}
