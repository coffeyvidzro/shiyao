package vmm

import (
	"crypto/sha256"
	"fmt"
	"net"
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
		if strings.HasPrefix(arg, "init=") { continue }
		trusted = append(trusted, arg)
	}
	trusted = append(trusted, "init="+guestAgentPath)
	return strings.Join(trusted, " "), nil
}

func validateVMID(vmID string) error {
	if vmID == "" { return fmt.Errorf("vm ID is required") }
	if len(vmID) > 64 { return fmt.Errorf("vm ID is too long") }
	if strings.ContainsAny(vmID, `/\\:*?"<>|`) { return fmt.Errorf("vm ID %q contains invalid characters", vmID) }
	return nil
}
