package sandbox

import (
	"context"
	"fmt"

	"github.com/coffeyvidzro/shiyao/internal/database/sqlc"
	apperrors "github.com/coffeyvidzro/shiyao/pkg/errors"
	"github.com/google/uuid"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(
	ctx context.Context,
	userID uuid.UUID,
) ([]sqlc.Sandbox, error) {
	if userID == uuid.Nil {
		return nil, apperrors.NewUnauthorized("authentication required")
	}

	return s.repo.ListByUserID(ctx, userID)
}

func (s *Service) Get(
	ctx context.Context,
	userID uuid.UUID,
	sandboxID uuid.UUID,
) (sqlc.Sandbox, error) {
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

func (s *Service) Create(
	ctx context.Context,
	userID uuid.UUID,
	req CreateRequest,
) (sqlc.Sandbox, error) {
	if userID == uuid.Nil {
		return sqlc.Sandbox{}, apperrors.NewUnauthorized("authentication required")
	}

	req = normalizeCreateRequest(req)
	if err := validateCreateRequest(req); err != nil {
		return sqlc.Sandbox{}, err
	}

	status := "pending"
	return s.repo.Create(ctx, sqlc.CreateSandboxParams{
		UserID:         userID,
		VmID:           newVMID(),
		Template:       req.Template,
		Vcpu:           req.VCPU,
		MemoryMb:       req.MemoryMB,
		TimeoutSeconds: req.TimeoutSeconds,
		AllowedHosts:   req.AllowedHosts,
		Status:         &status,
	})
}

func (s *Service) Delete(
	ctx context.Context,
	userID uuid.UUID,
	sandboxID uuid.UUID,
) error {
	if _, err := s.Get(ctx, userID, sandboxID); err != nil {
		return err
	}

	return s.repo.Delete(ctx, sandboxID)
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

func newVMID() string {
	return "sbx-" + uuid.NewString()
}
