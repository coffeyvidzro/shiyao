package vmm

import "fmt"

// Config contains core microVM runtime configuration.
type Config struct {
	KernelPath     string
	RootfsPath     string
	VCPUCount      int
	MemSizeMB      int
	BootArgs       string
	GuestAgentPath string
}

type SnapshotConfig struct {
	MemFilePath   string
	StateFilePath string
	EnableResume  bool
}

type SnapshotIntegrity struct {
	ExpectedMemHash   string
	ExpectedStateHash string
	KernelHash        string
	RootfsHash        string
}

func DefaultConfig() Config {
	return Config{VCPUCount: 2, MemSizeMB: 512, BootArgs: "console=ttyS0 reboot=k panic=1 pci=off", GuestAgentPath: "/usr/local/bin/shiyao-agent"}
}

func (c *Config) Validate() error {
	if c.KernelPath == "" { return fmt.Errorf("kernel path is required") }
	if c.RootfsPath == "" { return fmt.Errorf("rootfs path is required") }
	if c.VCPUCount <= 0 { return fmt.Errorf("vcpu count must be greater than 0") }
	if c.MemSizeMB <= 0 { return fmt.Errorf("memory size must be greater than 0") }
	if c.GuestAgentPath == "" { return fmt.Errorf("guest agent path is required") }
	return nil
}
