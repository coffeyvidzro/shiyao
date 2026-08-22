package sandbox

import (
	"fmt"
	"strings"

	apperrors "github.com/coffeyvidzro/shiyao/pkg/errors"
)

const (
	defaultTemplate       = "python-3.11"
	defaultVCPU           = int32(1)
	defaultMemoryMB       = int32(512)
	defaultTimeoutSeconds = int32(300)

	minVCPU           = int32(1)
	maxVCPU           = int32(8)
	minMemoryMB       = int32(128)
	maxMemoryMB       = int32(8192)
	minTimeoutSeconds = int32(30)
	maxTimeoutSeconds = int32(3600)
)

func normalizeCreateRequest(req CreateRequest) CreateRequest {
	req.Template = strings.TrimSpace(req.Template)
	if req.Template == "" {
		req.Template = defaultTemplate
	}

	if req.VCPU == 0 {
		req.VCPU = defaultVCPU
	}

	if req.MemoryMB == 0 {
		req.MemoryMB = defaultMemoryMB
	}

	if req.TimeoutSeconds == 0 {
		req.TimeoutSeconds = defaultTimeoutSeconds
	}

	allowedHosts := make([]string, 0, len(req.AllowedHosts))
	for _, host := range req.AllowedHosts {
		host = strings.ToLower(strings.TrimSpace(host))
		if host != "" {
			allowedHosts = append(allowedHosts, host)
		}
	}
	req.AllowedHosts = allowedHosts

	return req
}

func validateCreateRequest(req CreateRequest) error {
	if req.VCPU < minVCPU || req.VCPU > maxVCPU {
		return apperrors.NewBadRequest(
			fmt.Sprintf("vcpu must be between %d and %d", minVCPU, maxVCPU),
		)
	}
	if req.MemoryMB < minMemoryMB || req.MemoryMB > maxMemoryMB {
		return apperrors.NewBadRequest(
			fmt.Sprintf("memory_mb must be between %d and %d", minMemoryMB, maxMemoryMB),
		)
	}
	if req.TimeoutSeconds < minTimeoutSeconds || req.TimeoutSeconds > maxTimeoutSeconds {
		return apperrors.NewBadRequest(
			fmt.Sprintf("timeout_seconds must be between %d and %d", minTimeoutSeconds, maxTimeoutSeconds),
		)
	}
	return nil
}
