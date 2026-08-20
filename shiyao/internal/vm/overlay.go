package vm

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

const overlayBaseDir = "/run/shiyao/overlays"

type rootfsOverlay struct {
	lowerDir string
	upperDir string
	workDir  string
	mergedDir string
}

// prepareEphemeralRootfs creates an overlayfs mount whose lower layer is the
// immutable VM rootfs and whose upper/work layers live on tmpfs. All guest
// writes therefore disappear when the overlay is unmounted.
func prepareEphemeralRootfs(ctx context.Context, rootfsPath, instanceID string) (*rootfsOverlay, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if rootfsPath == "" || instanceID == "" {
		return nil, fmt.Errorf("rootfs path and instance ID are required")
	}
	if !filepath.IsAbs(rootfsPath) {
		return nil, fmt.Errorf("rootfs path must be absolute")
	}

	base := filepath.Join(overlayBaseDir, instanceID)
	upper := filepath.Join(base, "upper")
	work := filepath.Join(base, "work")
	merged := filepath.Join(base, "merged")
	for _, p := range []string{upper, work, merged} {
		if err := os.MkdirAll(p, 0700); err != nil {
			return nil, fmt.Errorf("create overlay directory %s: %w", p, err)
		}
	}

	o := &rootfsOverlay{lowerDir: rootfsPath, upperDir: upper, workDir: work, mergedDir: merged}
	mountedTmpfs := false
	mountedOverlay := false
	cleanup := func() {
		if mountedOverlay {
			_ = unix.Unmount(merged, 0)
		}
		if mountedTmpfs {
			_ = unix.Unmount(base, 0)
		}
		_ = os.RemoveAll(base)
	}

	// Keep upper/work on tmpfs so no guest modifications survive the VM.
	if err := unix.Mount("tmpfs", base, "tmpfs", 0, "size=512M,mode=0700"); err != nil {
		_ = os.RemoveAll(base)
		return nil, fmt.Errorf("mount overlay tmpfs: %w", err)
	}
	mountedTmpfs = true
	for _, p := range []string{upper, work, merged} {
		if err := os.MkdirAll(p, 0700); err != nil {
			cleanup()
			return nil, fmt.Errorf("create tmpfs overlay directory %s: %w", p, err)
		}
	}

	options := strings.Join([]string{
		"lowerdir=" + rootfsPath,
		"upperdir=" + upper,
		"workdir=" + work,
	}, ",")
	if err := unix.Mount("overlay", merged, "overlay", 0, options); err != nil {
		cleanup()
		return nil, fmt.Errorf("mount rootfs overlay: %w", err)
	}
	mountedOverlay = true
	return o, nil
}

func (o *rootfsOverlay) path() string { return o.mergedDir }

func (o *rootfsOverlay) cleanup() error {
	var firstErr error
	if err := unix.Unmount(o.mergedDir, 0); err != nil && err != unix.EINVAL && err != unix.ENOENT {
		firstErr = fmt.Errorf("unmount rootfs overlay: %w", err)
	}
	if err := unix.Unmount(filepath.Dir(o.mergedDir), 0); err != nil && err != unix.EINVAL && err != unix.ENOENT && firstErr == nil {
		firstErr = fmt.Errorf("unmount overlay tmpfs: %w", err)
	}
	if err := os.RemoveAll(filepath.Dir(o.mergedDir)); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("remove overlay directory: %w", err)
	}
	return firstErr
}
