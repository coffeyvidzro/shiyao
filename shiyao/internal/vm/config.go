package vm

import "fmt"

// Config contains the VM runtime configuration used to build the Firecracker instance.
type Config struct {
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
		VCPUCount: 2,
		MemSizeMB: 512,
		BootArgs:  "console=ttyS0 reboot=k panic=1 pci=off",
	}
}

// Validate checks if the configuration is valid for booting a VM.
// It ensures that critical paths and resource limits are properly set.
func (c *Config) Validate() error {
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
