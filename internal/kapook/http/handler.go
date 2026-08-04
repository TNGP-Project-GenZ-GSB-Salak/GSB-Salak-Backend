package http

import (
	"net/http"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/kapook"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/httpserver"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
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
	httpserver.OK(w, http.StatusCreated, toGoalResponse(goal))
}

// GetActiveGoal godoc
// @Summary      Get the active Kapook goal for an account
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
	httpserver.OK(w, http.StatusOK, toGoalResponse(goal))
}
