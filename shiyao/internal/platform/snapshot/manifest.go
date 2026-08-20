package snapshot

import "time"

// Manifest describes an immutable snapshot artifact produced from a Shiyao
// configuration. The manifest is intentionally independent of Firecracker so
// it can later be stored in a local or remote registry.
type Manifest struct {
	Version      string    `yaml:"version" json:"version"`
	Name         string    `yaml:"name" json:"name"`
	ConfigDigest string    `yaml:"config_digest" json:"config_digest"`
	KernelPath   string    `yaml:"kernel_path" json:"kernel_path"`
	RootfsPath   string    `yaml:"rootfs_path" json:"rootfs_path"`
	CreatedAt    time.Time `yaml:"created_at" json:"created_at"`
}

const ManifestVersion = "v1alpha1"
