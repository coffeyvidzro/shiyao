package snapshot

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

const DefaultSchemaVersion = "v1alpha1"

// Config is the declarative snapshot definition loaded from shiyao.yaml.
type Config struct {
	Version      string            `yaml:"version"`
	Name         string            `yaml:"name"`
	Description  string            `yaml:"description"`
	Runtime      RuntimeConfig     `yaml:"runtime"`
	Language     LanguageConfig    `yaml:"language"`
	Dependencies Dependencies      `yaml:"dependencies"`
	Resources    Resources         `yaml:"resources"`
	Env          map[string]string `yaml:"env"`
	Network      NetworkPolicy     `yaml:"network"`
}

type RuntimeConfig struct {
	OS           string `yaml:"os"`
	Distro       string `yaml:"distro"`
	Architecture string `yaml:"architecture"`
}

type LanguageConfig struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

type Dependencies struct {
	System []string `yaml:"system"`
	Pip    []string `yaml:"pip"`
	NPM    []string `yaml:"npm"`
}

type Resources struct {
	VCPU     int `yaml:"vcpu"`
	MemoryMB int `yaml:"memory_mb"`
	DiskMB   int `yaml:"disk_mb"`
}

type NetworkPolicy struct {
	AllowedDomains  []string `yaml:"allowed_domains"`
	BlockPrivateIPs bool     `yaml:"block_private_ips"`
}

// Load reads and validates a shiyao.yaml file.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read snapshot config: %w", err)
	}

	var cfg Config
	if err := yaml.UnmarshalStrict(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse snapshot config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c *Config) applyDefaults() {
	if c.Version == "" {
		c.Version = DefaultSchemaVersion
	}
	if c.Runtime.OS == "" {
		c.Runtime.OS = "linux"
	}
	if c.Runtime.Distro == "" {
		c.Runtime.Distro = "ubuntu-22.04"
	}
	if c.Runtime.Architecture == "" {
		c.Runtime.Architecture = "x86_64"
	}
	if c.Language.Name == "" {
		c.Language.Name = "python"
	}
	if c.Language.Version == "" {
		c.Language.Version = "3.11"
	}
	if c.Env == nil {
		c.Env = map[string]string{}
	}
}
