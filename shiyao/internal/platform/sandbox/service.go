package sandbox

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"

	"github.com/coffeyvidzro/shiyao/internal/core/vsock"
	"github.com/coffeyvidzro/shiyao/internal/database/sqlc"
	apperrors "github.com/coffeyvidzro/shiyao/pkg/errors"
)

type repository interface {
	Create(context.Context, sqlc.CreateSandboxParams) (sqlc.Sandbox, error)
	GetByID(context.Context, uuid.UUID) (sqlc.Sandbox, error)
	ListByUserID(context.Context, uuid.UUID) ([]sqlc.Sandbox, error)
	UpdateStatus(context.Context, uuid.UUID, string) (sqlc.Sandbox, error)
	Delete(context.Context, uuid.UUID) error
}

type Service struct {
	repo       repository
	dispatcher LifecycleDispatcher
}

func NewService(repo repository, dispatcher LifecycleDispatcher) *Service {
	return &Service{repo: repo, dispatcher: dispatcher}
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
	if s.dispatcher == nil {
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

	if err := s.dispatcher.DispatchCreate(ctx, LifecycleEvent{SandboxID: sandbox.ID, UserID: userID, VMID: sandbox.VmID}); err != nil {
		if _, statusErr := s.repo.UpdateStatus(ctx, sandbox.ID, "failed"); statusErr != nil {
			return sandbox, apperrors.NewInternal("sandbox provisioning failed and status update failed", fmt.Errorf("provision: %w; status update: %v", err, statusErr))
		}
		return sandbox, apperrors.NewInternal("sandbox provisioning failed", err)
	}

	return sandbox, nil
}

func (s *Service) Delete(ctx context.Context, userID, sandboxID uuid.UUID) error {
	if userID == uuid.Nil {
		return apperrors.NewUnauthorized("authentication required")
	}
	if s.dispatcher == nil {
		return apperrors.NewServiceUnavailable("sandbox execution is unavailable", nil)
	}

	sandbox, err := s.Get(ctx, userID, sandboxID)
	if err != nil {
		return err
	}

	if _, err := s.repo.UpdateStatus(ctx, sandbox.ID, "stopping"); err != nil {
		return apperrors.NewInternal("sandbox stop status update failed", err)
	}

	if err := s.dispatcher.DispatchDestroy(ctx, LifecycleEvent{SandboxID: sandbox.ID, UserID: userID, VMID: sandbox.VmID}); err != nil {
		if _, statusErr := s.repo.UpdateStatus(ctx, sandbox.ID, "cleanup_failed"); statusErr != nil {
			return apperrors.NewInternal("sandbox cleanup failed and status update failed", fmt.Errorf("destroy: %w; status update: %v", err, statusErr))
		}
		return apperrors.NewInternal("sandbox cleanup failed", err)
	}

	return nil
}

func (s *Service) ExecStream(ctx context.Context, sandboxID string, req vsock.ExecRequest, receive func(vsock.ExecFrame) error) (vsock.ExecResult, error) {
	return vsock.ExecStream(ctx, socketPathForSandbox(sandboxID), req, receive)
}

func newVMID() string { return "sbx-" + uuid.NewString() }

func socketPathForSandbox(sandboxID string) string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("firecracker-%s.sock", sandboxID))
}
