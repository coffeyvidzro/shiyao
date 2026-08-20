package main

import (
	"crypto/rand"
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

func startWithResourceLimits(cmd *exec.Cmd) (func() error, error) {
	cgroup, cleanupCgroup, err := prepareCgroup()
	if err != nil {
		return nil, fmt.Errorf("prepare cgroup: %w", err)
	}

	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	cmd.SysProcAttr.Pdeathsig = syscall.SIGKILL
	cmd.SysProcAttr.UseCgroupFD = true
	cmd.SysProcAttr.CgroupFD = int(cgroup.Fd())

	if err := cmd.Start(); err != nil {
		_ = cgroup.Close()
		_ = cleanupCgroup()
		return nil, fmt.Errorf("start command: %w", err)
	}
	_ = cgroup.Close()

	pid := cmd.Process.Pid
	killAndReap := func() {
		_ = unix.Kill(-pid, unix.SIGKILL)
		_ = cmd.Wait()
	}

	if err := applyRlimits(pid); err != nil {
		killAndReap()
		_ = cleanupCgroup()
		return nil, fmt.Errorf("apply process limits: %w", err)
	}
	return cleanupCgroup, nil
}

func prepareCgroup() (*os.File, func() error, error) {
	if err := os.MkdirAll(guestCgroupRoot, 0755); err != nil {
		return nil, nil, err
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(guestCgroupRoot), "cgroup.subtree_control"), []byte("+cpu +memory +pids"), 0644); err != nil {
		return nil, nil, fmt.Errorf("enable cgroup controllers: %w", err)
	}

	name, err := randomCgroupName()
	if err != nil {
		return nil, nil, err
	}
	path := filepath.Join(guestCgroupRoot, name)
	if err := os.Mkdir(path, 0755); err != nil {
		return nil, nil, err
	}

	settings := map[string]string{
		"memory.max": strconv.Itoa(guestMemoryMax),
		"pids.max":   strconv.Itoa(guestPidsMax),
		"cpu.max":    fmt.Sprintf("%d %d", guestCPUQuotaUS, guestCPUPeriodUS),
	}
	for setting, value := range settings {
		if err := os.WriteFile(filepath.Join(path, setting), []byte(value), 0644); err != nil {
			_ = os.Remove(path)
			return nil, nil, err
		}
	}

	fd, err := unix.Open(path, unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		_ = os.Remove(path)
		return nil, nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	cleanup := func() error {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return file, cleanup, nil
}

func randomCgroupName() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("exec-%x", b[:]), nil
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
