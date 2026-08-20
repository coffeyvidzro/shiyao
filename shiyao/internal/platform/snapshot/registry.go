package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Registry stores snapshot manifests and artifacts on the local filesystem.
// The storage boundary can later be replaced or extended with a remote backend
// without changing snapshot building or execution code.
type Registry struct {
	Root string
}

func NewRegistry(root string) (*Registry, error) {
	if root == "" {
		return nil, fmt.Errorf("snapshot registry root is required")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create snapshot registry: %w", err)
	}
	return &Registry{Root: root}, nil
}

func (r *Registry) manifestPath(name string) string {
	return filepath.Join(r.Root, name, "manifest.json")
}

func (r *Registry) Put(manifest Manifest) error {
	if manifest.Name == "" {
		return fmt.Errorf("snapshot manifest name is required")
	}

	path := r.manifestPath(manifest.Name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create snapshot directory: %w", err)
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode snapshot manifest: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write snapshot manifest: %w", err)
	}
	return nil
}

func (r *Registry) Get(name string) (Manifest, error) {
	data, err := os.ReadFile(r.manifestPath(name))
	if err != nil {
		return Manifest{}, fmt.Errorf("read snapshot manifest: %w", err)
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode snapshot manifest: %w", err)
	}
	return manifest, nil
}
