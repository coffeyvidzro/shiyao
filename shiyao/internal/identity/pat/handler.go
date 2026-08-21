package pat

import (
	"github.com/coffeyvidzro/shiyao/internal/identity/session"
	"github.com/coffeyvidzro/shiyao/pkg/errors"
	"github.com/coffeyvidzro/shiyao/pkg/httputil"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Create(c *gin.Context) {
	userID, err := session.UserIDFromContext(c)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, errors.NewBadRequest("invalid request body"))
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
	userID, err := session.UserIDFromContext(c)
	if err != nil {
		httputil.Error(c, err)
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
	userID, err := session.UserIDFromContext(c)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	tokenID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, errors.NewBadRequest("invalid token ID"))
		return
	}

	if err := h.service.Revoke(c.Request.Context(), userID, tokenID); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.OK(c, gin.H{"message": "token revoked"})
}
