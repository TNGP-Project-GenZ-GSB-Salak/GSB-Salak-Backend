package http

import (
	"net/http"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/httpserver"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/middleware"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/salak"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	service salak.Service
}

func NewHandler(service salak.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/salak/products", h.ListProducts)
	rg.GET("/salak/products/:id", h.GetProduct)
	rg.GET("/salak/holdings", h.ListHoldings)
}

func (h *Handler) ListProducts(c *gin.Context) {
	products, err := h.service.ListProducts(c.Request.Context())
	if err != nil {
		httpserver.Fail(c, err)
		return
	}
	httpserver.OK(c, http.StatusOK, toProductResponses(products))
}

func (h *Handler) GetProduct(c *gin.Context) {
	productID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product id"})
		return
	}

	p, err := h.service.GetProduct(c.Request.Context(), productID)
	if err != nil {
		httpserver.Fail(c, err)
		return
	}
	httpserver.OK(c, http.StatusOK, toProductResponse(p))
}

func (h *Handler) ListHoldings(c *gin.Context) {
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

	holdings, err := h.service.ListHoldingsByAccount(c.Request.Context(), userID, accountID)
	if err != nil {
		httpserver.Fail(c, err)
		return
	}

	products, err := h.service.ListProducts(c.Request.Context())
	if err != nil {
		httpserver.Fail(c, err)
		return
	}
	productNames := make(map[uuid.UUID]string, len(products))
	for _, p := range products {
		productNames[p.ID] = p.Name
	}

	httpserver.OK(c, http.StatusOK, toHoldingResponses(holdings, productNames))
}
