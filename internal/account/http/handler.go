package http

import (
	"net/http"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/account"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/httpserver"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	service account.Service
}

func NewHandler(service account.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/accounts", h.ListMine)
	r.Get("/accounts/{id}", h.GetByID)
}

// ListMine godoc
// @Summary      List my accounts
// @Description  Lists all accounts (savings and Salak) owned by the authenticated user.
// @Tags         accounts
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  httpserver.DataEnvelope{data=[]accountResponse}
// @Failure      401  {object}  httpserver.ErrorEnvelope
// @Router       /api/v1/accounts [get]
func (h *Handler) ListMine(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpserver.RequireUserID(w, r)
	if !ok {
		return
	}

	accounts, err := h.service.ListByUser(r.Context(), userID)
	if err != nil {
		httpserver.Fail(w, r, err)
		return
	}
	httpserver.OK(w, http.StatusOK, toAccountResponses(accounts))
}

// GetByID godoc
// @Summary      Get an account by id
// @Description  Fetches a single account owned by the authenticated user.
// @Tags         accounts
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Account ID (UUID)"
// @Success      200  {object}  httpserver.DataEnvelope{data=accountResponse}
// @Failure      400  {object}  httpserver.ErrorEnvelope
// @Failure      401  {object}  httpserver.ErrorEnvelope
// @Failure      404  {object}  httpserver.ErrorEnvelope
// @Router       /api/v1/accounts/{id} [get]
func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpserver.RequireUserID(w, r)
	if !ok {
		return
	}

	accountID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpserver.Error(w, http.StatusBadRequest, "invalid account id")
		return
	}

	a, err := h.service.GetByID(r.Context(), userID, accountID)
	if err != nil {
		httpserver.Fail(w, r, err)
		return
	}
	httpserver.OK(w, http.StatusOK, toAccountResponse(a))
}
