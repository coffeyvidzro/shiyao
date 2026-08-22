//go:build cloud

package teamtoken

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/coffeyvidzro/shiyao/internal/authn"
	apperrors "github.com/coffeyvidzro/shiyao/pkg/errors"
	"github.com/coffeyvidzro/shiyao/pkg/httputil"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) Create(c *gin.Context) {
	teamID, userID, ok := requestIDs(c)
	if !ok {
		return
	}
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, apperrors.NewBadRequest("invalid request body"))
		return
	}
	result, err := h.service.Create(c.Request.Context(), teamID, userID, req)
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Created(c, result)
}
func (h *Handler) List(c *gin.Context) {
	teamID, userID, ok := requestIDs(c)
	if !ok {
		return
	}
	items, err := h.service.List(c.Request.Context(), teamID, userID)
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.OK(c, items)
}
func (h *Handler) Revoke(c *gin.Context) {
	teamID, userID, ok := requestIDs(c)
	if !ok {
		return
	}
	tokenID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, apperrors.NewBadRequest("invalid token ID"))
		return
	}
	if err := h.service.Revoke(c.Request.Context(), teamID, userID, tokenID); err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.OK(c, gin.H{"message": "token revoked"})
}
func requestIDs(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	userID, ok := authn.UserIDFromContext(c.Request.Context())
	if !ok {
		httputil.Error(c, apperrors.NewUnauthorized("authentication required"))
		return uuid.Nil, uuid.Nil, false
	}
	teamID, err := uuid.Parse(c.Param("team_id"))
	if err != nil {
		httputil.Error(c, apperrors.NewBadRequest("invalid team ID"))
		return uuid.Nil, uuid.Nil, false
	}
	return teamID, userID, true
}
