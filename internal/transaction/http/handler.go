package http

import (
	"net/http"
	"strconv"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/httpserver"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/transaction"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	service transaction.Service
}

func NewHandler(service transaction.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/transactions/buy-salak", h.BuySalak)
	r.Get("/transactions", h.ListHistory)
}

func (h *Handler) BuySalak(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpserver.RequireUserID(w, r)
	if !ok {
		return
	}

	var req buySalakRequest
	if err := httpserver.DecodeAndValidate(r, &req); err != nil {
		httpserver.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	receipt, err := h.service.BuySalak(r.Context(), userID, req.FundingAccountID, req.SalakAccountID, req.ProductID, req.Amount)
	if err != nil {
		httpserver.Fail(w, r, err)
		return
	}
	httpserver.OK(w, http.StatusCreated, toBuySalakResponse(receipt))
}

func (h *Handler) ListHistory(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpserver.RequireUserID(w, r)
	if !ok {
		return
	}

	accountID, err := uuid.Parse(r.URL.Query().Get("account_id"))
	if err != nil {
		httpserver.Error(w, http.StatusBadRequest, "valid account_id query param is required")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	entries, err := h.service.ListHistory(r.Context(), userID, accountID, limit, offset)
	if err != nil {
		httpserver.Fail(w, r, err)
		return
	}
	httpserver.OK(w, http.StatusOK, toLedgerEntryResponses(entries))
}
