package users

import (
	"github.com/gin-gonic/gin"

	"github.com/coffeyvidzro/shiyao/internal/authn"
	"github.com/coffeyvidzro/shiyao/internal/database/sqlc"
	"github.com/coffeyvidzro/shiyao/internal/identity/session"
	apperrors "github.com/coffeyvidzro/shiyao/pkg/errors"
	"github.com/shiyao/shiyao/pkg/helper"
	"github.com/shiyao/shiyao/pkg/httputil"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) GetMe(c *gin.Context) {
	userID, ok := authn.UserIDFromContext(c.Request.Context())
	if !ok {
		httputil.Error(c, apperrors.NewUnauthorized("authentication required"))
		return
	}

	user, err := h.service.GetMe(c.Request.Context(), userID)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.OK(c, newResponse(user))
}

func (h *Handler) UpdateMe(c *gin.Context) {
	var req UpdateMeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, err)
		return
	}

	userID, ok := authn.UserIDFromContext(c.Request.Context())
	if !ok {
		httputil.Error(c, apperrors.NewUnauthorized("authentication required"))
		return
	}

	user, err := h.service.UpdateMe(c.Request.Context(), userID, req)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.OK(c, newResponse(user))
}

func (h *Handler) DeleteMe(c *gin.Context) {
	userID, ok := authn.UserIDFromContext(c.Request.Context())
	if !ok {
		httputil.Error(c, apperrors.NewUnauthorized("authentication required"))
		return
	}

	if err := h.service.DeleteMe(c.Request.Context(), userID); err != nil {
		httputil.Error(c, err)
		return
	}

	session.ClearCookie(c)

	httputil.OK(c, gin.H{"message": "user deleted"})
}

func newResponse(user sqlc.User) Response {
	return Response{
		ID:            user.ID,
		Email:         user.Email,
		EmailVerified: user.EmailVerified,
		Name:          user.Name,
		CreatedAt:     helper.FormatTime(user.CreatedAt),
		UpdatedAt:     helper.FormatTime(user.UpdatedAt),
	}
}
