package main

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

const (
	overlayBase = "/dev/shm/shiyao-overlay"
	overlaySize = "size=512M,mode=0700"
)

// setupEphemeralRootfs replaces the guest's immutable root with an OverlayFS.
// The lower layer is the original rootfs and the upper/work layers live on a
// tmpfs. The tmpfs is nested under /dev so moving /dev into the new root also
// preserves the upper/work mount after the old root is detached.
func setupEphemeralRootfs() error {
	upper := filepath.Join(overlayBase, "upper")
	work := filepath.Join(overlayBase, "work")
	newRoot := filepath.Join(overlayBase, "newroot")
	oldRoot := filepath.Join(newRoot, ".oldroot")

	if err := os.MkdirAll(overlayBase, 0700); err != nil {
		return fmt.Errorf("create overlay base: %w", err)
	}
	if err := unix.Mount("tmpfs", overlayBase, "tmpfs", 0, overlaySize); err != nil {
		return fmt.Errorf("mount overlay tmpfs: %w", err)
	}

	mountedTmpfs := true
	mountedOverlay := false
	cleanupBeforePivot := func() {
		if mountedOverlay {
			_ = unix.Unmount(newRoot, unix.MNT_DETACH)
		}
		if mountedTmpfs {
			_ = unix.Unmount(overlayBase, unix.MNT_DETACH)
		}
	}

	for _, path := range []string{upper, work, newRoot, oldRoot} {
		if err := os.MkdirAll(path, 0700); err != nil {
			cleanupBeforePivot()
			return fmt.Errorf("create overlay directory %s: %w", path, err)
		}
	}

	// OverlayFS requires upperdir and workdir to reside on the same filesystem.
	options := "lowerdir=/,upperdir=" + upper + ",workdir=" + work
	if err := unix.Mount("overlay", newRoot, "overlay", 0, options); err != nil {
		cleanupBeforePivot()
		return fmt.Errorf("mount root overlay: %w", err)
	}
	mountedOverlay = true

	if err := unix.PivotRoot(newRoot, oldRoot); err != nil {
		cleanupBeforePivot()
		return fmt.Errorf("pivot root to ephemeral overlay: %w", err)
	}
	mountedOverlay = false

	if err := os.Chdir("/"); err != nil {
		return fmt.Errorf("chdir new root: %w", err)
	}

	// Move the existing kernel/filesystem mounts out of the detached root. The
	// mounts are optional because some minimal guest images do not have all of
	// them mounted at boot.
	for _, name := range []string{"proc", "sys", "dev", "run"} {
		src := filepath.Join("/.oldroot", name)
		dst := filepath.Join("/", name)
		if err := moveRootMount(src, dst); err != nil {
			return fmt.Errorf("move %s mount: %w", name, err)
		}
	}

	if err := unix.Unmount("/.oldroot", unix.MNT_DETACH); err != nil {
		return fmt.Errorf("detach immutable root: %w", err)
	}
	return nil
}

func moveRootMount(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("source is not a directory")
	}
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	return unix.Mount(src, dst, "", unix.MS_MOVE, "")
}
