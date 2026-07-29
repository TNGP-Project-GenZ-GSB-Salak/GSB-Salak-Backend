package http

import (
	"net/http"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/account"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/httpserver"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	service account.Service
}

func NewHandler(service account.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/accounts", h.ListMine)
	rg.GET("/accounts/:id", h.GetByID)
}

func (h *Handler) ListMine(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	accounts, err := h.service.ListByUser(c.Request.Context(), userID)
	if err != nil {
		httpserver.Fail(c, err)
		return
	}
	httpserver.OK(c, http.StatusOK, toAccountResponses(accounts))
}

func (h *Handler) GetByID(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	accountID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}

	a, err := h.service.GetByID(c.Request.Context(), userID, accountID)
	if err != nil {
		httpserver.Fail(c, err)
		return
	}
	httpserver.OK(c, http.StatusOK, toAccountResponse(a))
}
