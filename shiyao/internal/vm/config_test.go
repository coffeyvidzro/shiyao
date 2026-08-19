package vm

import "testing"

func TestDefaultConfigIncludesFirecrackerAndKVMDefaults(t *testing.T) {
	cfg := DefaultConfig()
	cfg.KernelPath = "/tmp/vmlinux"
	cfg.RootfsPath = "/tmp/rootfs.ext4"

	if err := cfg.Validate(); err != nil {
		t.Fatalf("default config with kernel and rootfs should validate: %v", err)
	}
	if cfg.FirecrackerBin != defaultFirecrackerBinary {
		t.Fatalf("expected firecracker binary %q, got %q", defaultFirecrackerBinary, cfg.FirecrackerBin)
	}
	if cfg.KVMPath != defaultKVMDevice {
		t.Fatalf("expected KVM device %q, got %q", defaultKVMDevice, cfg.KVMPath)
	}
	if !cfg.EnablePCI {
		t.Fatal("expected PCI transport to be enabled by default")
	}
}

func TestConfigValidateRequiresFirecrackerAndKVMConfiguration(t *testing.T) {
	base := DefaultConfig()
	base.KernelPath = "/tmp/vmlinux"
	base.RootfsPath = "/tmp/rootfs.ext4"

	tests := []struct {
		name string
		edit func(*Config)
	}{
		{name: "missing firecracker binary", edit: func(c *Config) { c.FirecrackerBin = "" }},
		{name: "missing kvm path", edit: func(c *Config) { c.KVMPath = "" }},
		{name: "missing kernel path", edit: func(c *Config) { c.KernelPath = "" }},
		{name: "missing rootfs path", edit: func(c *Config) { c.RootfsPath = "" }},
		{name: "invalid vcpu count", edit: func(c *Config) { c.VCPUCount = 0 }},
		{name: "invalid memory", edit: func(c *Config) { c.MemSizeMB = 0 }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := base
			tt.edit(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestFirecrackerCommand(t *testing.T) {
	inst := NewInstance("vm-1", "/tmp/firecracker-vm-1.sock", Config{FirecrackerBin: "/usr/bin/firecracker", EnablePCI: true}, NetworkConfig{})
	cmd := inst.firecrackerCommand(t.Context())
	if cmd.Path != "/usr/bin/firecracker" {
		t.Fatalf("expected command path to be configured binary, got %q", cmd.Path)
	}
	want := []string{"/usr/bin/firecracker", "--api-sock", "/tmp/firecracker-vm-1.sock", "--enable-pci"}
	if len(cmd.Args) != len(want) {
		t.Fatalf("expected args %v, got %v", want, cmd.Args)
	}
	for idx := range want {
		if cmd.Args[idx] != want[idx] {
			t.Fatalf("expected args %v, got %v", want, cmd.Args)
		}
	}
}
