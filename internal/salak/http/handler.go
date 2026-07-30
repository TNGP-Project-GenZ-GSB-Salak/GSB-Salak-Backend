package http

import (
	"net/http"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/httpserver"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/salak"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	service salak.Service
}

func NewHandler(service salak.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/salak/products", h.ListProducts)
	r.Get("/salak/products/{id}", h.GetProduct)
	r.Get("/salak/holdings", h.ListHoldings)
}

// ListProducts godoc
// @Summary      List Salak products
// @Description  Lists all available Salak lottery-savings-bond products.
// @Tags         salak
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  httpserver.DataEnvelope{data=[]productResponse}
// @Failure      401  {object}  httpserver.ErrorEnvelope
// @Router       /api/v1/salak/products [get]
func (h *Handler) ListProducts(w http.ResponseWriter, r *http.Request) {
	products, err := h.service.ListProducts(r.Context())
	if err != nil {
		httpserver.Fail(w, r, err)
		return
	}
	httpserver.OK(w, http.StatusOK, toProductResponses(products))
}

// GetProduct godoc
// @Summary      Get a Salak product by id
// @Tags         salak
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Product ID (UUID)"
// @Success      200  {object}  httpserver.DataEnvelope{data=productResponse}
// @Failure      400  {object}  httpserver.ErrorEnvelope
// @Failure      401  {object}  httpserver.ErrorEnvelope
// @Failure      404  {object}  httpserver.ErrorEnvelope
// @Router       /api/v1/salak/products/{id} [get]
func (h *Handler) GetProduct(w http.ResponseWriter, r *http.Request) {
	productID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpserver.Error(w, http.StatusBadRequest, "invalid product id")
		return
	}

	p, err := h.service.GetProduct(r.Context(), productID)
	if err != nil {
		httpserver.Fail(w, r, err)
		return
	}
	httpserver.OK(w, http.StatusOK, toProductResponse(p))
}

// ListHoldings godoc
// @Summary      List Salak holdings for an account
// @Description  Lists the lottery ticket holdings minted for the given Salak account.
// @Tags         salak
// @Produce      json
// @Security     BearerAuth
// @Param        account_id  query     string  true  "Salak account ID (UUID)"
// @Success      200  {object}  httpserver.DataEnvelope{data=[]holdingResponse}
// @Failure      400  {object}  httpserver.ErrorEnvelope
// @Failure      401  {object}  httpserver.ErrorEnvelope
// @Router       /api/v1/salak/holdings [get]
func (h *Handler) ListHoldings(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpserver.RequireUserID(w, r)
	if !ok {
		return
	}

	accountID, err := uuid.Parse(r.URL.Query().Get("account_id"))
	if err != nil {
		httpserver.Error(w, http.StatusBadRequest, "valid account_id query param is required")
		return
	}

	holdings, err := h.service.ListHoldingsByAccount(r.Context(), userID, accountID)
	if err != nil {
		httpserver.Fail(w, r, err)
		return
	}

	products, err := h.service.ListProducts(r.Context())
	if err != nil {
		httpserver.Fail(w, r, err)
		return
	}
	productNames := make(map[uuid.UUID]string, len(products))
	for _, p := range products {
		productNames[p.ID] = p.Name
	}

	httpserver.OK(w, http.StatusOK, toHoldingResponses(holdings, productNames))
}
