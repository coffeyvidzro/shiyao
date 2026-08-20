package snapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Builder creates the filesystem artifact and manifest for a snapshot.
// Guest package installation and Firecracker snapshot creation are deliberately
// separate concerns and will be added behind this boundary next.
type Builder struct {
	Registry *Registry
	Kernel   string
}

func NewBuilder(registry *Registry, kernel string) (*Builder, error) {
	if registry == nil {
		return nil, fmt.Errorf("snapshot registry is required")
	}
	if kernel == "" {
		return nil, fmt.Errorf("kernel path is required")
	}
	return &Builder{Registry: registry, Kernel: kernel}, nil
}

func (b *Builder) Build(cfg Config, rootfs string) (Manifest, error) {
	if err := cfg.Validate(); err != nil {
		return Manifest{}, err
	}
	if rootfs == "" {
		return Manifest{}, fmt.Errorf("rootfs path is required")
	}
	if _, err := os.Stat(rootfs); err != nil {
		return Manifest{}, fmt.Errorf("stat rootfs: %w", err)
	}
	if _, err := os.Stat(b.Kernel); err != nil {
		return Manifest{}, fmt.Errorf("stat kernel: %w", err)
	}

	digest := sha256.New()
	for _, value := range []string{cfg.Name, cfg.Version, cfg.Runtime.OS, cfg.Runtime.Distro, cfg.Runtime.Architecture} {
		_, _ = digest.Write([]byte(value))
	}
	configDigest := hex.EncodeToString(digest.Sum(nil))

	manifest := Manifest{
		Version:      ManifestVersion,
		Name:         cfg.Name,
		ConfigDigest: configDigest,
		KernelPath:   b.Kernel,
		RootfsPath:   filepath.Clean(rootfs),
		CreatedAt:    time.Now().UTC(),
	}
	if err := b.Registry.Put(manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}
