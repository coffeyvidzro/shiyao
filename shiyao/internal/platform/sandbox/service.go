package sandbox

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"

	"github.com/coffeyvidzro/shiyao/internal/database/sqlc"
	"github.com/coffeyvidzro/shiyao/internal/vsock"
	apperrors "github.com/coffeyvidzro/shiyao/pkg/errors"
)

type Service struct {
	repo      *Repository
	vmManager VMManager
}

func NewService(repo *Repository, vmManager VMManager) *Service {
	return &Service{
		repo:      repo,
		vmManager: vmManager,
	}
}

func (s *Service) List(ctx context.Context, userID uuid.UUID) ([]sqlc.Sandbox, error) {
	if userID == uuid.Nil {
		return nil, apperrors.NewUnauthorized("authentication required")
	}

	return s.repo.ListByUserID(ctx, userID)
}

func (s *Service) Get(ctx context.Context, userID, sandboxID uuid.UUID) (sqlc.Sandbox, error) {
	if userID == uuid.Nil {
		return sqlc.Sandbox{}, apperrors.NewUnauthorized("authentication required")
	}

	if sandboxID == uuid.Nil {
		return sqlc.Sandbox{}, apperrors.NewBadRequest("invalid sandbox ID")
	}

	sandbox, err := s.repo.GetByID(ctx, sandboxID)
	if err != nil {
		return sqlc.Sandbox{}, apperrors.NewNotFound("sandbox not found")
	}

	if sandbox.UserID != userID {
		return sqlc.Sandbox{}, apperrors.NewNotFound("sandbox not found")
	}

	return sandbox, nil
}

func (s *Service) Create(ctx context.Context, userID uuid.UUID, req CreateRequest) (sqlc.Sandbox, error) {
	if userID == uuid.Nil {
		return sqlc.Sandbox{}, apperrors.NewUnauthorized("authentication required")
	}
	if s.vmManager == nil {
		return sqlc.Sandbox{}, apperrors.NewServiceUnavailable("sandbox execution is unavailable", nil)
	}

	req = normalizeCreateRequest(req)
	if err := validateCreateRequest(req); err != nil {
		return sqlc.Sandbox{}, err
	}

	status := "pending"
	sandbox, err := s.repo.Create(ctx, sqlc.CreateSandboxParams{
		UserID:         userID,
		VmID:           newVMID(),
		Template:       req.Template,
		Vcpu:           req.VCPU,
		MemoryMb:       req.MemoryMB,
		TimeoutSeconds: req.TimeoutSeconds,
		AllowedHosts:   req.AllowedHosts,
		Status:         &status,
	})
	if err != nil {
		return sqlc.Sandbox{}, err
	}

	if err := s.vmManager.ProvisionVM(ctx, sandbox.VmID); err != nil {
		if _, statusErr := s.repo.UpdateStatus(ctx, sandbox.ID, "failed"); statusErr != nil {
			return sandbox, apperrors.NewInternal("sandbox provisioning failed and status update failed", fmt.Errorf("provision: %w; status update: %v", err, statusErr))
		}
		return sandbox, apperrors.NewInternal("sandbox provisioning failed", err)
	}

	sandbox, err = s.repo.UpdateStatus(ctx, sandbox.ID, "running")
	if err != nil {
		return sandbox, apperrors.NewInternal("sandbox started but status update failed", err)
	}

	return sandbox, nil
}

func (s *Service) Delete(ctx context.Context, userID, sandboxID uuid.UUID) error {
	if userID == uuid.Nil {
		return apperrors.NewUnauthorized("authentication required")
	}
	if s.vmManager == nil {
		return apperrors.NewServiceUnavailable("sandbox execution is unavailable", nil)
	}

	sandbox, err := s.Get(ctx, userID, sandboxID)
	if err != nil {
		return err
	}

	if err := s.vmManager.DestroyVM(ctx, sandbox.VmID); err != nil {
		if _, statusErr := s.repo.UpdateStatus(ctx, sandbox.ID, "cleanup_failed"); statusErr != nil {
			return apperrors.NewInternal("sandbox cleanup failed and status update failed", fmt.Errorf("destroy: %w; status update: %v", err, statusErr))
		}
		return apperrors.NewInternal("sandbox cleanup failed", err)
	}

	if _, err := s.repo.UpdateStatus(ctx, sandbox.ID, "stopped"); err != nil {
		return apperrors.NewInternal("sandbox stopped but status update failed", err)
	}

	if err := s.repo.Delete(ctx, sandbox.ID); err != nil {
		return apperrors.NewInternal("sandbox stopped but database cleanup failed", err)
	}

	return nil
}

func (s *Service) ExecStream(ctx context.Context, sandboxID string, req vsock.ExecRequest, receive func(vsock.ExecFrame) error) (vsock.ExecResult, error) {
	return vsock.ExecStream(ctx, socketPathForSandbox(sandboxID), req, receive)
}

func validateCreateRequest(req CreateRequest) error {
	if req.VCPU < minVCPU || req.VCPU > maxVCPU {
		return apperrors.NewBadRequest(fmt.Sprintf("vcpu must be between %d and %d", minVCPU, maxVCPU))
	}
	if req.MemoryMB < minMemoryMB || req.MemoryMB > maxMemoryMB {
		return apperrors.NewBadRequest(fmt.Sprintf("memory_mb must be between %d and %d", minMemoryMB, maxMemoryMB))
	}
	if req.TimeoutSeconds < minTimeoutSeconds || req.TimeoutSeconds > maxTimeoutSeconds {
		return apperrors.NewBadRequest(fmt.Sprintf("timeout_seconds must be between %d and %d", minTimeoutSeconds, maxTimeoutSeconds))
	}
	return nil
}

func newVMID() string {
	return "sbx-" + uuid.NewString()
}

func socketPathForSandbox(sandboxID string) string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("firecracker-%s.sock", sandboxID))
}
