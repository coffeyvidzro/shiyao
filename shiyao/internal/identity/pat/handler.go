package pat

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/coffeyvidzro/shiyao/internal/authn"
	apperrors "github.com/coffeyvidzro/shiyao/pkg/errors"
	"github.com/shiyao/pkg/httputil"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Create(c *gin.Context) {
	userID, ok := authn.UserIDFromContext(c.Request.Context())
	if !ok {
		httputil.Error(c, apperrors.NewUnauthorized("authentication required"))
		return
	}

	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, apperrors.NewBadRequest("invalid request body"))
		return
	}

	result, err := h.service.Create(c.Request.Context(), userID, req)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Created(c, result)
}

func (h *Handler) List(c *gin.Context) {
	userID, ok := authn.UserIDFromContext(c.Request.Context())
	if !ok {
		httputil.Error(c, apperrors.NewUnauthorized("authentication required"))
		return
	}

	items, err := h.service.List(c.Request.Context(), userID)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.OK(c, items)
}

func (h *Handler) Revoke(c *gin.Context) {
	userID, ok := authn.UserIDFromContext(c.Request.Context())
	if !ok {
		httputil.Error(c, apperrors.NewUnauthorized("authentication required"))
		return
	}

	tokenID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, apperrors.NewBadRequest("invalid token ID"))
		return
	}

	if err := h.service.Revoke(c.Request.Context(), userID, tokenID); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.OK(c, gin.H{"message": "token revoked"})
}
