package snapshot

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRegistry_PushAndPullArtifacts(t *testing.T) {
	ctx := context.Background()

	// 1. Setup temporary local storage backend
	storeDir, err := os.MkdirTemp("", "shiyao-store-*")
	if err != nil {
		t.Fatalf("failed to create store temp dir: %v", err)
	}
	defer os.RemoveAll(storeDir)

	store, err := NewLocalStore(storeDir)
	if err != nil {
		t.Fatalf("failed to create local store: %v", err)
	}

	reg, err := NewRegistry(store)
	if err != nil {
		t.Fatalf("failed to create registry: %v", err)
	}

	// 2. Prepare dummy artifact file
	sourceDir, err := os.MkdirTemp("", "shiyao-source-*")
	if err != nil {
		t.Fatalf("failed to create source temp dir: %v", err)
	}
	defer os.RemoveAll(sourceDir)

	sourceFile := filepath.Join(sourceDir, "test-rootfs.ext4")
	dummyData := []byte("dummy ext4 filesystem binary content")
	if err := os.WriteFile(sourceFile, dummyData, 0o644); err != nil {
		t.Fatalf("failed to write source artifact: %v", err)
	}

	snapshotName := "ubuntu-node-v1"

	// 3. Test PushArtifact
	if err := reg.PushArtifact(ctx, snapshotName, "rootfs.ext4", sourceFile); err != nil {
		t.Fatalf("PushArtifact failed: %v", err)
	}

	// 4. Test PullArtifact to target location
	targetDir, err := os.MkdirTemp("", "shiyao-target-*")
	if err != nil {
		t.Fatalf("failed to create target temp dir: %v", err)
	}
	defer os.RemoveAll(targetDir)

	targetFile := filepath.Join(targetDir, "downloaded-rootfs.ext4")
	if err := reg.PullArtifact(ctx, snapshotName, "rootfs.ext4", targetFile); err != nil {
		t.Fatalf("PullArtifact failed: %v", err)
	}

	// 5. Assert integrity
	retrievedData, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("failed to read pulled artifact: %v", err)
	}

	if string(retrievedData) != string(dummyData) {
		t.Errorf("pulled artifact content mismatch: got %q, want %q", string(retrievedData), string(dummyData))
	}
}
