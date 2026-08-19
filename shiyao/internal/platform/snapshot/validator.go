package snapshot

import "fmt"

// Validate applies defaults and checks the supported v1alpha1 schema.
func (c *Config) Validate() error {
	c.applyDefaults()
	if c.Name == "" {
		return fmt.Errorf("snapshot name is required")
	}
	if c.Version != DefaultSchemaVersion {
		return fmt.Errorf("unsupported snapshot schema version %q", c.Version)
	}
	if c.Runtime.OS != "linux" {
		return fmt.Errorf("unsupported runtime OS %q", c.Runtime.OS)
	}
	if c.Runtime.Architecture != "x86_64" {
		return fmt.Errorf("unsupported architecture %q", c.Runtime.Architecture)
	}
	if c.Resources.VCPU <= 0 {
		return fmt.Errorf("resources.vcpu must be greater than 0")
	}
	if c.Resources.MemoryMB < 128 {
		return fmt.Errorf("resources.memory_mb must be at least 128")
	}
	if c.Resources.DiskMB < 1024 {
		return fmt.Errorf("resources.disk_mb must be at least 1024")
	}
	for _, domain := range c.Network.AllowedDomains {
		if domain == "" {
			return fmt.Errorf("network.allowed_domains cannot contain empty domains")
		}
	}
	return nil
}
