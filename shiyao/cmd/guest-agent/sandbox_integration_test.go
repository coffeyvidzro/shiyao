//go:build integration

package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func requireRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Skip("sandbox integration tests require root")
	}
}

func TestCommandStartsInsideDedicatedCgroup(t *testing.T) {
	requireRoot(t)

	cmd := exec.Command("sleep", "2")
	cleanup, err := startWithResourceLimits(cmd)
	if err != nil {
		if errors.Is(err, unix.EACCES) || errors.Is(err, unix.EPERM) || errors.Is(err, unix.EROFS) {
			t.Skipf("cgroup v2 delegation unavailable: %v", err)
		}
		t.Fatalf("startWithResourceLimits: %v", err)
	}
	defer func() { _ = cleanup() }()

	cgroupData, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(cmd.Process.Pid), "cgroup"))
	if err != nil {
		t.Fatalf("read child cgroup: %v", err)
	}
	if !strings.Contains(string(cgroupData), "/shiyao/exec-") {
		t.Fatalf("child is not in a dedicated shiyao cgroup: %q", cgroupData)
	}

	if err := cmd.Process.Kill(); err != nil {
		t.Fatalf("kill test process: %v", err)
	}
	if err := cmd.Wait(); err == nil {
		t.Fatal("expected killed process to return an error")
	}
}

func TestEphemeralOverlayDiscardsWrites(t *testing.T) {
	requireRoot(t)

	base := t.TempDir()
	lower := filepath.Join(base, "lower")
	upper := filepath.Join(base, "upper")
	work := filepath.Join(base, "work")
	merged := filepath.Join(base, "merged")
	for _, path := range []string{lower, upper, work, merged} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}

	original := filepath.Join(lower, "immutable.txt")
	if err := os.WriteFile(original, []byte("base"), 0600); err != nil {
		t.Fatal(err)
	}

	mountOptions := "lowerdir=" + lower + ",upperdir=" + upper + ",workdir=" + work
	if err := unix.Mount("overlay", merged, "overlay", 0, mountOptions); err != nil {
		t.Skipf("overlayfs unavailable: %v", err)
	}

	mounted := true
	cleanup := func() {
		if mounted {
			_ = unix.Unmount(merged, unix.MNT_DETACH)
		}
	}
	defer cleanup()

	writePath := filepath.Join(merged, "created.txt")
	if err := os.WriteFile(writePath, []byte("ephemeral"), 0600); err != nil {
		t.Fatalf("write through overlay: %v", err)
	}

	updated := filepath.Join(merged, "immutable.txt")
	if err := os.WriteFile(updated, []byte("changed"), 0600); err != nil {
		t.Fatalf("modify through overlay: %v", err)
	}

	if _, err := os.Stat(filepath.Join(lower, "created.txt")); !os.IsNotExist(err) {
		t.Fatalf("created file leaked into lower layer: %v", err)
	}

	contents, err := os.ReadFile(original)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "base" {
		t.Fatalf("lower layer changed to %q", contents)
	}

	if err := unix.Unmount(merged, unix.MNT_DETACH); err != nil {
		t.Fatal(err)
	}
	mounted = false

	if _, err := os.Stat(writePath); !os.IsNotExist(err) {
		t.Fatalf("ephemeral file remained after unmount: %v", err)
	}
}
