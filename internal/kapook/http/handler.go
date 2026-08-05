package http

import (
	"net/http"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/kapook"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/httpserver"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Handler struct {
	service kapook.Service
}

func NewHandler(service kapook.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/kapook/terms/accept", h.AcceptTerms)
	r.Get("/kapook/terms", h.GetTermsStatus)
	r.Post("/kapook/goals", h.CreateGoal)
	r.Get("/kapook/goals/active", h.GetActiveGoal)
	r.Post("/kapook/goals/deposit", h.Deposit)
	r.Post("/kapook/goals/withdraw", h.Withdraw)
	r.Get("/kapook/goals/withdrawal-status", h.GetWithdrawalStatus)
	r.Post("/kapook/goals/buy", h.BuyFromGoal)
}

// AcceptTerms godoc
// @Summary      Accept the Kapook terms and conditions
// @Description  Records the authenticated user's acceptance. Idempotent - accepting twice is a no-op.
// @Tags         kapook
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  httpserver.DataEnvelope{data=termsStatusResponse}
// @Failure      401  {object}  httpserver.ErrorEnvelope
// @Router       /api/v1/kapook/terms/accept [post]
func (h *Handler) AcceptTerms(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpserver.RequireUserID(w, r)
	if !ok {
		return
	}

	if err := h.service.Accept(r.Context(), userID); err != nil {
		httpserver.Fail(w, r, err)
		return
	}
	httpserver.OK(w, http.StatusOK, termsStatusResponse{Accepted: true})
}

// GetTermsStatus godoc
// @Summary      Get the authenticated user's Kapook terms acceptance status
// @Tags         kapook
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  httpserver.DataEnvelope{data=termsStatusResponse}
// @Failure      401  {object}  httpserver.ErrorEnvelope
// @Router       /api/v1/kapook/terms [get]
func (h *Handler) GetTermsStatus(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpserver.RequireUserID(w, r)
	if !ok {
		return
	}

	accepted, err := h.service.HasAccepted(r.Context(), userID)
	if err != nil {
		httpserver.Fail(w, r, err)
		return
	}
	httpserver.OK(w, http.StatusOK, termsStatusResponse{Accepted: accepted})
}

// CreateGoal godoc
// @Summary      Create a Kapook savings goal
// @Description  Sets a target amount for the caller's kapook account, saving toward the given Salak product. Rejected if the terms aren't accepted, if an active goal already exists, or if the amount isn't a valid eventual purchase for the product.
// @Tags         kapook
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      createGoalRequest  true  "Goal details"
// @Success      201      {object}  httpserver.DataEnvelope{data=goalResponse}
// @Failure      400      {object}  httpserver.ErrorEnvelope
// @Failure      401      {object}  httpserver.ErrorEnvelope
// @Failure      403      {object}  httpserver.ErrorEnvelope
// @Failure      409      {object}  httpserver.ErrorEnvelope
// @Router       /api/v1/kapook/goals [post]
func (h *Handler) CreateGoal(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpserver.RequireUserID(w, r)
	if !ok {
		return
	}

	var req createGoalRequest
	if err := httpserver.DecodeAndValidate(r, &req); err != nil {
		httpserver.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	goal, err := h.service.CreateGoal(r.Context(), userID, req.AccountID, req.ProductID, req.GoalAmount)
	if err != nil {
		httpserver.Fail(w, r, err)
		return
	}
	snap, err := h.service.Snapshot(r.Context(), goal)
	if err != nil {
		httpserver.Fail(w, r, err)
		return
	}
	httpserver.OK(w, http.StatusCreated, toGoalResponse(snap))
}

// GetActiveGoal godoc
// @Summary      Get the active Kapook goal for an account
// @Description  Returns a null data payload (still 200) when accountID has no active goal - a normal empty state, not an error. 404 stays reserved for ownership failures.
// @Tags         kapook
// @Produce      json
// @Security     BearerAuth
// @Param        account_id  query     string  true  "Kapook account ID (UUID)"
// @Success      200  {object}  httpserver.DataEnvelope{data=goalResponse}
// @Failure      400  {object}  httpserver.ErrorEnvelope
// @Failure      401  {object}  httpserver.ErrorEnvelope
// @Failure      404  {object}  httpserver.ErrorEnvelope
// @Router       /api/v1/kapook/goals/active [get]
func (h *Handler) GetActiveGoal(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpserver.RequireUserID(w, r)
	if !ok {
		return
	}

	accountID, err := uuid.Parse(r.URL.Query().Get("account_id"))
	if err != nil {
		httpserver.Error(w, http.StatusBadRequest, "valid account_id query param is required")
		return
	}

	goal, err := h.service.GetActiveGoal(r.Context(), userID, accountID)
	if err != nil {
		httpserver.Fail(w, r, err)
		return
	}
	if goal == nil {
		httpserver.OK(w, http.StatusOK, nil)
		return
	}
	snap, err := h.service.Snapshot(r.Context(), *goal)
	if err != nil {
		httpserver.Fail(w, r, err)
		return
	}
	httpserver.OK(w, http.StatusOK, toGoalResponse(snap))
}

// Deposit godoc
// @Summary      Pay into a Kapook goal
// @Description  Moves any positive amount from a customer-chosen savings account into the kapook account, atomically, and bumps the active goal's saved amount. Rejected if that would exceed the goal's target.
// @Tags         kapook
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      depositRequest  true  "Deposit details"
// @Success      200      {object}  httpserver.DataEnvelope{data=goalResponse}
// @Failure      400      {object}  httpserver.ErrorEnvelope
// @Failure      401      {object}  httpserver.ErrorEnvelope
// @Failure      404      {object}  httpserver.ErrorEnvelope
// @Router       /api/v1/kapook/goals/deposit [post]
func (h *Handler) Deposit(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpserver.RequireUserID(w, r)
	if !ok {
		return
	}

	var req depositRequest
	if err := httpserver.DecodeAndValidate(r, &req); err != nil {
		httpserver.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	goal, err := h.service.Deposit(r.Context(), userID, req.KapookAccountID, req.SavingsAccountID, req.Amount)
	if err != nil {
		httpserver.Fail(w, r, err)
		return
	}
	snap, err := h.service.Snapshot(r.Context(), goal)
	if err != nil {
		httpserver.Fail(w, r, err)
		return
	}
	httpserver.OK(w, http.StatusOK, toGoalResponse(snap))
}

// Withdraw godoc
// @Summary      Withdraw from a Kapook goal
// @Description  Moves any amount up to the active goal's saved balance from the kapook account back to the customer's primary account (บัญชีคู่โอน) - never a customer-chosen destination. A customer with no primary account flagged gets a 404 with code kapook_no_primary_account. The first two withdrawals in the goal's rolling 12-month window are free; later ones carry a 2% fee, rounded to two decimal places, taken out of what reaches savings. The goal survives, even if this empties it.
// @Tags         kapook
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      withdrawRequest  true  "Withdrawal details"
// @Success      200      {object}  httpserver.DataEnvelope{data=withdrawResponse}
// @Failure      400      {object}  httpserver.ErrorEnvelope
// @Failure      401      {object}  httpserver.ErrorEnvelope
// @Failure      404      {object}  httpserver.ErrorEnvelope
// @Router       /api/v1/kapook/goals/withdraw [post]
func (h *Handler) Withdraw(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpserver.RequireUserID(w, r)
	if !ok {
		return
	}

	var req withdrawRequest
	if err := httpserver.DecodeAndValidate(r, &req); err != nil {
		httpserver.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.service.Withdraw(r.Context(), userID, req.KapookAccountID, req.Amount)
	if err != nil {
		httpserver.Fail(w, r, err)
		return
	}
	snap, err := h.service.Snapshot(r.Context(), result.Goal)
	if err != nil {
		httpserver.Fail(w, r, err)
		return
	}
	httpserver.OK(w, http.StatusOK, toWithdrawResponse(result, snap))
}

// GetWithdrawalStatus godoc
// @Summary      Preview a Kapook goal's free-withdrawal allowance
// @Description  Reports how many free withdrawals have been used in the goal's current rolling 12-month window and whether the next withdrawal would be free or carry the 2% fee - a preview only, not a lock; Withdraw re-checks under lock. When amount is given, also quotes the fee/net that amount would incur if withdrawn right now, from the same computation Withdraw itself uses.
// @Tags         kapook
// @Produce      json
// @Security     BearerAuth
// @Param        kapook_account_id  query     string  true   "Kapook account ID (UUID)"
// @Param        amount             query     string  false  "Candidate withdrawal amount to quote a fee/net for"
// @Success      200  {object}  httpserver.DataEnvelope{data=withdrawalStatusResponse}
// @Failure      400  {object}  httpserver.ErrorEnvelope
// @Failure      401  {object}  httpserver.ErrorEnvelope
// @Failure      404  {object}  httpserver.ErrorEnvelope
// @Router       /api/v1/kapook/goals/withdrawal-status [get]
func (h *Handler) GetWithdrawalStatus(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpserver.RequireUserID(w, r)
	if !ok {
		return
	}

	kapookAccountID, err := uuid.Parse(r.URL.Query().Get("kapook_account_id"))
	if err != nil {
		httpserver.Error(w, http.StatusBadRequest, "valid kapook_account_id query param is required")
		return
	}

	var amount *decimal.Decimal
	if raw := r.URL.Query().Get("amount"); raw != "" {
		parsed, err := decimal.NewFromString(raw)
		if err != nil {
			httpserver.Error(w, http.StatusBadRequest, "amount query param must be a valid decimal")
			return
		}
		amount = &parsed
	}

	status, err := h.service.GetWithdrawalStatus(r.Context(), userID, kapookAccountID, amount)
	if err != nil {
		httpserver.Fail(w, r, err)
		return
	}
	httpserver.OK(w, http.StatusOK, toWithdrawalStatusResponse(status))
}

// BuyFromGoal godoc
// @Summary      Buy Salak from a Kapook goal
// @Description  Converts any 1,000-multiple amount up to the goal's available balance (saved minus already converted) into the goal's own product, gated on that balance being at least the product's minimum purchase. A purchase that fully satisfies the goal's target deactivates it; a partial one leaves it active.
// @Tags         kapook
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      buyFromGoalRequest  true  "Purchase details"
// @Success      201      {object}  httpserver.DataEnvelope{data=buyFromGoalResponse}
// @Failure      400      {object}  httpserver.ErrorEnvelope
// @Failure      401      {object}  httpserver.ErrorEnvelope
// @Failure      404      {object}  httpserver.ErrorEnvelope
// @Router       /api/v1/kapook/goals/buy [post]
func (h *Handler) BuyFromGoal(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpserver.RequireUserID(w, r)
	if !ok {
		return
	}

	var req buyFromGoalRequest
	if err := httpserver.DecodeAndValidate(r, &req); err != nil {
		httpserver.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.service.BuyFromGoal(r.Context(), userID, req.KapookAccountID, req.SalakAccountID, req.Amount)
	if err != nil {
		httpserver.Fail(w, r, err)
		return
	}
	snap, err := h.service.Snapshot(r.Context(), result.Goal)
	if err != nil {
		httpserver.Fail(w, r, err)
		return
	}
	httpserver.OK(w, http.StatusCreated, toBuyFromGoalResponse(result, snap))
}
