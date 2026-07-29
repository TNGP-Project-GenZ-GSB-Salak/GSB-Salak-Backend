package http

import (
	"net/http"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/httpserver"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/user"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service user.Service
}

func NewHandler(service user.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/auth/register", h.Register)
	rg.POST("/auth/login", h.Login)
}

func (h *Handler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	u, err := h.service.Register(c.Request.Context(), req.Username, req.Password, req.FullName)
	if err != nil {
		httpserver.Fail(c, err)
		return
	}
	httpserver.OK(c, http.StatusCreated, toUserResponse(u))
}

func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	u, token, err := h.service.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		httpserver.Fail(c, err)
		return
	}
	httpserver.OK(c, http.StatusOK, loginResponse{User: toUserResponse(u), Token: token})
}
