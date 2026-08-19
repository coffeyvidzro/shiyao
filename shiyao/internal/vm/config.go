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
	// FirecrackerBin is the firecracker VMM executable used by the SDK process runner.
	FirecrackerBin string
	// KVMPath is the KVM device Firecracker requires for virtualization.
	KVMPath string
	// EnablePCI passes --enable-pci when launching Firecracker to use PCI VirtIO transport.
	EnablePCI bool
	// KernelPath is the absolute path to the guest kernel image.
	KernelPath string
	// RootfsPath is the absolute path to the guest root filesystem.
	RootfsPath string
	// VCPUCount is the number of virtual CPUs allocated to the VM.
	VCPUCount int
	// MemSizeMB is the amount of memory in MiB allocated to the VM.
	MemSizeMB int
	// BootArgs are the kernel command-line arguments passed to the guest.
	BootArgs string
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
// It ensures that critical paths and resource limits are properly set.
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
