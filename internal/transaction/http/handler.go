package http

import (
	"net/http"
	"strconv"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/httpserver"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/middleware"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/transaction"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	service transaction.Service
}

func NewHandler(service transaction.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/transactions/buy-salak", h.BuySalak)
	rg.GET("/transactions", h.ListHistory)
}

func (h *Handler) BuySalak(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req buySalakRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	receipt, err := h.service.BuySalak(c.Request.Context(), userID, req.FundingAccountID, req.SalakAccountID, req.ProductID, req.Amount)
	if err != nil {
		httpserver.Fail(c, err)
		return
	}
	httpserver.OK(c, http.StatusCreated, toBuySalakResponse(receipt))
}

func (h *Handler) ListHistory(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	accountID, err := uuid.Parse(c.Query("account_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid account_id query param is required"})
		return
	}

	limit, _ := strconv.Atoi(c.Query("limit"))
	offset, _ := strconv.Atoi(c.Query("offset"))

	entries, err := h.service.ListHistory(c.Request.Context(), userID, accountID, limit, offset)
	if err != nil {
		httpserver.Fail(c, err)
		return
	}
	httpserver.OK(c, http.StatusOK, toLedgerEntryResponses(entries))
}
