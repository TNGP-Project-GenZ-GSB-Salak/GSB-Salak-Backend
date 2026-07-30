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

// BuySalak godoc
// @Summary      Buy Salak
// @Description  Transfers funds from a savings account into a Salak account, minting a lottery holding.
// @Tags         transactions
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      buySalakRequest  true  "Purchase details"
// @Success      201      {object}  httpserver.DataEnvelope{data=buySalakResponse}
// @Failure      400      {object}  httpserver.ErrorEnvelope
// @Failure      401      {object}  httpserver.ErrorEnvelope
// @Router       /api/v1/transactions/buy-salak [post]
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

// ListHistory godoc
// @Summary      List transaction history
// @Description  Lists ledger entries (debits/credits) for the given account.
// @Tags         transactions
// @Produce      json
// @Security     BearerAuth
// @Param        account_id  query     string  true   "Account ID (UUID)"
// @Param        limit       query     int     false  "Max number of entries to return"
// @Param        offset      query     int     false  "Number of entries to skip"
// @Success      200  {object}  httpserver.DataEnvelope{data=[]ledgerEntryResponse}
// @Failure      400  {object}  httpserver.ErrorEnvelope
// @Failure      401  {object}  httpserver.ErrorEnvelope
// @Router       /api/v1/transactions [get]
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
