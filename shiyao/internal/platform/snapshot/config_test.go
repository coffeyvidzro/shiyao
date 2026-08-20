package snapshot

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAppliesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shiyao.yaml")
	config := []byte(`
name: coding-agent
resources:
  vcpu: 2
  memory_mb: 512
  disk_mb: 1024
`)
	if err := os.WriteFile(path, config, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Version != DefaultSchemaVersion || got.Runtime.OS != "linux" {
		t.Fatalf("defaults not applied: %+v", got)
	}
}

func TestValidateRejectsInvalidResources(t *testing.T) {
	cfg := Config{Name: "bad", Resources: Resources{VCPU: 1, MemoryMB: 64, DiskMB: 1024}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected memory validation error")
	}
}
