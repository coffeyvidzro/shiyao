package sandbox

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/coffeyvidzro/shiyao/internal/authn"
	"github.com/coffeyvidzro/shiyao/internal/database/sqlc"
	ws "github.com/coffeyvidzro/shiyao/internal/websocket"
	apperrors "github.com/coffeyvidzro/shiyao/pkg/errors"
	"github.com/shiyao/pkg/helper"
	"github.com/shiyao/pkg/httputil"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) userID(c *gin.Context) (uuid.UUID, bool) {
	userID, ok := authn.UserIDFromContext(c.Request.Context())
	if !ok {
		httputil.Error(c, apperrors.NewUnauthorized("authentication required"))
	}
	return userID, ok
}

func (h *Handler) List(c *gin.Context) {
	userID, ok := h.userID(c)
	if !ok {
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
	userID, ok := h.userID(c)
	if !ok {
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

	userID, ok := h.userID(c)
	if !ok {
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
	userID, ok := h.userID(c)
	if !ok {
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

func (h *Handler) ExecStreamWS(c *gin.Context) {
	userID, ok := h.userID(c)
	if !ok {
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
	if sandbox.Status != "running" {
		httputil.Error(c, apperrors.NewConflict("sandbox is not running"))
		return
	}

	conn, err := ws.Upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	wsConn := ws.NewConn(conn)
	wsConn.SetDeadlines()
	ws.BridgeExecStream(c.Request.Context(), wsConn, sandbox.VmID, h.service)
}

func parseSandboxID(value string) (uuid.UUID, error) {
	return uuid.Parse(value)
}

func newResponse(sandbox sqlc.Sandbox) Response {
	return Response{
		ID:             sandbox.ID,
		UserID:         sandbox.UserID,
		VMID:           sandbox.VmId,
		Template:       sandbox.Template,
		Status:         sandbox.Status,
		VCPU:           sandbox.Vcpu,
		MemoryMB:       sandbox.MemoryMb,
		TimeoutSeconds: sandbox.TimeoutSeconds,
		AllowedHosts:   sandbox.AllowedHosts,
		CreatedAt:      helper.FormatTime(sandbox.CreatedAt),
		StartedAt:      helper.FormatTime(sandbox.StartedAt),
		StoppedAt:      helper.FormatTime(sandbox.StoppedAt),
		UpdatedAt:      helper.FormatTime(sandbox.UpdatedAt),
	}
}
