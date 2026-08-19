package vm

import (
	"fmt"
	"os"
)

const (
	defaultFirecrackerBinary = "firecracker"
	defaultKVMDevice         = "/dev/kvm"
)

// Config contains the VM runtime configuration used to build the Firecracker instance.
type Config struct {
	FirecrackerBin string
	KVMPath        string
	EnablePCI      bool
	KernelPath     string
	RootfsPath     string
	VCPUCount      int
	MemSizeMB      int
	BootArgs       string
}

// DefaultConfig builds a sane default runtime configuration.
func DefaultConfig() Config {
	return Config{
		FirecrackerBin: defaultFirecrackerBinary,
		KVMPath:        defaultKVMDevice,
		EnablePCI:      true,
		VCPUCount:      2,
		MemSizeMB:      512,
		BootArgs:       "console=ttyS0 reboot=k panic=1 pci=off",
	}
}

// Validate checks if the configuration is valid for booting a VM.
func (c Config) Validate() error {
	if c.FirecrackerBin == "" {
		return fmt.Errorf("firecracker binary path is required")
	}
	if c.KVMPath == "" {
		return fmt.Errorf("kvm device path is required")
	}
	if c.KernelPath == "" {
		return fmt.Errorf("kernel path is required")
	}
	if c.RootfsPath == "" {
		return fmt.Errorf("rootfs path is required")
	}
	if c.VCPUCount <= 0 {
		return fmt.Errorf("vcpu count must be greater than 0")
	}
	if c.MemSizeMB <= 0 {
		return fmt.Errorf("memory size must be greater than 0")
	}
	return nil
}

// CheckKVMAccess verifies that the configured KVM device exists and is readable
// and writable by the current process, as required before launching Firecracker.
func (c Config) CheckKVMAccess() error {
	kvmPath := c.KVMPath
	if kvmPath == "" {
		kvmPath = defaultKVMDevice
	}

	info, err := os.Stat(kvmPath)
	if err != nil {
		return fmt.Errorf("stat kvm device %s: %w", kvmPath, err)
	}
	if info.IsDir() {
		return fmt.Errorf("kvm device %s is a directory", kvmPath)
	}

	file, err := os.OpenFile(kvmPath, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open kvm device %s read/write: %w", kvmPath, err)
	}
	return file.Close()
}
