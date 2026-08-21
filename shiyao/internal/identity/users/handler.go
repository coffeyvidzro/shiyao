package users

import (
	"github.com/coffeyvidzro/shiyao/internal/database/sqlc"
	"github.com/coffeyvidzro/shiyao/internal/identity/session"
	"github.com/coffeyvidzro/shiyao/pkg/helper"
	"github.com/coffeyvidzro/shiyao/pkg/httputil"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{
		service: service,
	}
}

func (h *Handler) GetMe(c *gin.Context) {
	userID, err := session.UserIDFromContext(c)
	if err != nil {
		httputil.Error(c, err)
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

	userID, err := session.UserIDFromContext(c)
	if err != nil {
		httputil.Error(c, err)
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
	userID, err := session.UserIDFromContext(c)
	if err != nil {
		httputil.Error(c, err)
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
