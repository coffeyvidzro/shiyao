package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"golang.org/x/sys/unix"
)

const (
	guestCgroupRoot  = "/sys/fs/cgroup/shiyao"
	guestMemoryMax   = 384 << 20
	guestPidsMax     = 256
	guestCPUQuotaUS  = 100000
	guestCPUPeriodUS = 100000
	guestFileMax     = 256 << 20
	guestNoFileMax   = 4096
)

func startWithResourceLimits(cmd *exec.Cmd) error {
	cgroup, err := prepareCgroup()
	if err != nil {
		return fmt.Errorf("prepare cgroup: %w", err)
	}
	defer func() { _ = cgroup.Close() }()

	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	cmd.SysProcAttr.Pdeathsig = syscall.SIGKILL
	cmd.SysProcAttr.UseCgroupFD = true
	cmd.SysProcAttr.CgroupFD = int(cgroup.Fd())

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start command: %w", err)
	}

	pid := cmd.Process.Pid
	killAndReap := func() {
		_ = unix.Kill(-pid, unix.SIGKILL)
		_ = cmd.Wait()
	}

	if err := applyRlimits(pid); err != nil {
		killAndReap()
		return fmt.Errorf("apply process limits: %w", err)
	}
	return nil
}

func prepareCgroup() (*os.File, error) {
	if err := os.MkdirAll(guestCgroupRoot, 0755); err != nil {
		return nil, err
	}

	settings := map[string]string{
		"memory.max": strconv.Itoa(guestMemoryMax),
		"pids.max":   strconv.Itoa(guestPidsMax),
		"cpu.max":    fmt.Sprintf("%d %d", guestCPUQuotaUS, guestCPUPeriodUS),
	}
	for name, value := range settings {
		if err := os.WriteFile(filepath.Join(guestCgroupRoot, name), []byte(value), 0644); err != nil {
			return nil, err
		}
	}

	fd, err := unix.Open(guestCgroupRoot, unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), guestCgroupRoot), nil
}

func applyRlimits(pid int) error {
	limits := []struct {
		resource int
		value    uint64
	}{
		{unix.RLIMIT_CPU, 300},
		{unix.RLIMIT_FSIZE, guestFileMax},
		{unix.RLIMIT_NOFILE, guestNoFileMax},
	}
	for _, limit := range limits {
		rlimit := &unix.Rlimit{Cur: limit.value, Max: limit.value}
		if err := unix.Prlimit(pid, limit.resource, rlimit, nil); err != nil {
			return err
		}
	}
	return nil
}
