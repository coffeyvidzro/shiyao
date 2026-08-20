package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"golang.org/x/sys/unix"
)

const (
	guestCgroupRoot = "/sys/fs/cgroup/shiyao"
	guestMemoryMax  = 384 << 20
	guestPidsMax    = 256
	guestCPUQuotaUS = 100000
	guestCPUPeriodUS = 100000
	guestFileMax    = 256 << 20
	guestNoFileMax  = 4096
)

func startWithResourceLimits(cmd *exec.Cmd) error {
	if cmd.SysProcAttr == nil { cmd.SysProcAttr = &syscall.SysProcAttr{} }
	cmd.SysProcAttr.Setpgid = true
	cmd.SysProcAttr.Pdeathsig = syscall.SIGKILL
	if err := cmd.Start(); err != nil { return fmt.Errorf("start command: %w", err) }

	pid := cmd.Process.Pid
	if err := applyRlimits(pid); err != nil {
		_ = unix.Kill(-pid, unix.SIGKILL)
		return fmt.Errorf("apply process limits: %w", err)
	}
	if err := addToCgroup(pid); err != nil {
		_ = unix.Kill(-pid, unix.SIGKILL)
		return fmt.Errorf("apply cgroup limits: %w", err)
	}
	return nil
}

func applyRlimits(pid int) error {
	limits := []struct{ resource int; value uint64 }{
		{unix.RLIMIT_CPU, 300},
		{unix.RLIMIT_FSIZE, guestFileMax},
		{unix.RLIMIT_NOFILE, guestNoFileMax},
	}
	for _, l := range limits {
		r := &unix.Rlimit{Cur: l.value, Max: l.value}
		if err := unix.Prlimit(pid, l.resource, r, nil); err != nil { return err }
	}
	return nil
}

func addToCgroup(pid int) error {
	if err := os.MkdirAll(guestCgroupRoot, 0755); err != nil { return err }
	settings := map[string]string{
		"memory.max": strconv.Itoa(guestMemoryMax),
		"pids.max": strconv.Itoa(guestPidsMax),
		"cpu.max": fmt.Sprintf("%d %d", guestCPUQuotaUS, guestCPUPeriodUS),
	}
	for name, value := range settings {
		if err := os.WriteFile(filepath.Join(guestCgroupRoot, name), []byte(value), 0644); err != nil { return err }
	}
	return os.WriteFile(filepath.Join(guestCgroupRoot, "cgroup.procs"), []byte(strconv.Itoa(pid)), 0644)
}
