package sandbox

import (
	"net/http"

	"github.com/coffeyvidzro/shiyao/internal/database/sqlc"
	"github.com/coffeyvidzro/shiyao/internal/identity/session"
	apperrors "github.com/coffeyvidzro/shiyao/pkg/errors"
	"github.com/coffeyvidzro/shiyao/pkg/httputil"
	"github.com/coffeyvidzro/shiyao/pkg/pgconv"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) List(c *gin.Context) {
	userID, err := session.UserIDFromContext(c)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	sandboxes, err := h.service.List(c.Request.Context(), userID)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	responses := make([]Response, 0, len(sandboxes))
	for _, item := range sandboxes {
		responses = append(responses, newResponse(item))
	}

	httputil.OK(c, responses)
}

func (h *Handler) Get(c *gin.Context) {
	userID, err := session.UserIDFromContext(c)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	sandboxID, err := parseSandboxID(c.Param("id"))
	if err != nil {
		httputil.Error(c, apperrors.NewBadRequest("invalid sandbox ID"))
		return
	}

	sandbox, err := h.service.Get(c.Request.Context(), userID, sandboxID)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.OK(c, newResponse(sandbox))
}

func (h *Handler) Create(c *gin.Context) {
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, apperrors.NewBadRequest("invalid request body"))
		return
	}

	userID, err := session.UserIDFromContext(c)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	sandbox, err := h.service.Create(c.Request.Context(), userID, req)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Created(c, newResponse(sandbox))
}

func (h *Handler) Delete(c *gin.Context) {
	userID, err := session.UserIDFromContext(c)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	sandboxID, err := parseSandboxID(c.Param("id"))
	if err != nil {
		httputil.Error(c, apperrors.NewBadRequest("invalid sandbox ID"))
		return
	}

	if err := h.service.Delete(c.Request.Context(), userID, sandboxID); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.OK(c, gin.H{"message": "sandbox deleted"})
}

func parseSandboxID(value string) (uuid.UUID, error) {
	return uuid.Parse(value)
}

func newResponse(sandbox sqlc.Sandbox) Response {
	return Response{
		ID:             sandbox.ID,
		UserID:         sandbox.UserID,
		VMID:           sandbox.VmID,
		Template:       sandbox.Template,
		Status:         sandbox.Status,
		VCPU:           sandbox.Vcpu,
		MemoryMB:       sandbox.MemoryMb,
		TimeoutSeconds: sandbox.TimeoutSeconds,
		AllowedHosts:   sandbox.AllowedHosts,
		CreatedAt:      formatTime(sandbox.CreatedAt),
		StartedAt:      formatTime(sandbox.StartedAt),
		StoppedAt:      formatTime(sandbox.StoppedAt),
		UpdatedAt:      formatTime(sandbox.UpdatedAt),
	}
}

func formatTime(value pgtype.Timestamptz) string {
	if !value.Valid {
		return ""
	}

	return pgconv.TimestamptzToTime(value).Format(http.TimeFormat)
}
