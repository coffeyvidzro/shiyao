package vmm

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

func (i *Instance) validateSnapshotIntegrity() error {
	if i.snapInteg.ExpectedMemHash == "" && i.snapInteg.ExpectedStateHash == "" {
		return nil
	}
	if i.snapInteg.ExpectedMemHash != "" {
		hash, err := computeFileHash(i.snapCfg.MemFilePath)
		if err != nil {
			return fmt.Errorf("compute memory file hash: %w", err)
		}
		if hash != i.snapInteg.ExpectedMemHash {
			return fmt.Errorf("memory file hash mismatch: expected %s, got %s", i.snapInteg.ExpectedMemHash, hash)
		}
	}
	if i.snapInteg.ExpectedStateHash != "" {
		hash, err := computeFileHash(i.snapCfg.StateFilePath)
		if err != nil {
			return fmt.Errorf("compute state file hash: %w", err)
		}
		if hash != i.snapInteg.ExpectedStateHash {
			return fmt.Errorf("state file hash mismatch: expected %s, got %s", i.snapInteg.ExpectedStateHash, hash)
		}
	}
	return nil
}

func computeFileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
