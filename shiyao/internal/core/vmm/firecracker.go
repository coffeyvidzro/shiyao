package vmm

import (
	"crypto/sha256"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
)

func generateMAC(vmID string) string {
	hash := sha256.Sum256([]byte(vmID))
	mac := net.HardwareAddr{0x02, hash[0] & 0xFE, hash[1], hash[2], hash[3], hash[4]}
	return mac.String()
}

func guestKernelArgs(bootArgs, guestAgentPath string) (string, error) {
	guestAgentPath = strings.TrimSpace(guestAgentPath)
	if guestAgentPath == "" || !filepath.IsAbs(guestAgentPath) || strings.ContainsAny(guestAgentPath, "\t\r\n ") {
		return "", fmt.Errorf("guest agent path must be an absolute path without whitespace")
	}
	args := strings.Fields(strings.TrimSpace(bootArgs))
	trusted := make([]string, 0, len(args)+1)
	for _, arg := range args {
		if strings.HasPrefix(arg, "init=") {
			continue
		}
		trusted = append(trusted, arg)
	}
	trusted = append(trusted, "init="+guestAgentPath)
	return strings.Join(trusted, " "), nil
}

func validateVMID(vmID string) error {
	if vmID == "" {
		return fmt.Errorf("vm ID is required")
	}
	if len(vmID) > 64 {
		return fmt.Errorf("vm ID is too long")
	}
	if strings.ContainsAny(vmID, `/\\:*?"<>|`) {
		return fmt.Errorf("vm ID %q contains invalid characters", vmID)
	}
	return nil
}

func validateRuntimeAssets(cfg Config, snap SnapshotConfig) error {
	if snap.EnableResume {
		if err := validateRegularFile(snap.MemFilePath, "snapshot memory file"); err != nil {
			return err
		}
		if err := validateRegularFile(snap.StateFilePath, "snapshot state file"); err != nil {
			return err
		}
		return nil
	}
	if err := validateRegularFile(cfg.KernelPath, "kernel image"); err != nil {
		return err
	}
	if err := validateRegularFile(cfg.RootfsPath, "root filesystem"); err != nil {
		return err
	}
	return nil
}

func validateRegularFile(path, label string) error {
	if path == "" {
		return fmt.Errorf("%s path is required", label)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", label, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s must not be a symbolic link", label)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s must be a regular file", label)
	}
	return nil
}
