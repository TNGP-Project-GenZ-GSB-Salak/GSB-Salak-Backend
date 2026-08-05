package http

import (
	"net/http"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/admin"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/kapook"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/httpserver"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Handler owns both the admin login route (public) and the admin-gated
// action routes. The action(s) themselves delegate to whichever domain's
// Service actually owns the business logic (kapook.Service, here) - admin
// is an identity/authorization gate, not a place business rules live.
type Handler struct {
	service admin.Service
	kapook  kapook.Service
}

func NewHandler(service admin.Service, kapookSvc kapook.Service) *Handler {
	return &Handler{service: service, kapook: kapookSvc}
}

// RegisterPublicRoutes registers the routes reachable with no credential at
// all - just admin/login.
func (h *Handler) RegisterPublicRoutes(r chi.Router) {
	r.Post("/admin/login", h.Login)
}

// RegisterAdminRoutes registers the routes the caller has already wrapped
// in middleware.AdminAuth.
func (h *Handler) RegisterAdminRoutes(r chi.Router) {
	r.Post("/admin/holdings/{id}/settle", h.SettleHolding)
}

// Login godoc
// @Summary      Admin login
// @Description  Authenticates an admin and returns a JWT bearer token, separate from customer auth.
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        request  body      loginRequest  true  "Admin credentials"
// @Success      200      {object}  httpserver.DataEnvelope{data=loginResponse}
// @Failure      401      {object}  httpserver.ErrorEnvelope
// @Router       /api/v1/admin/login [post]
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httpserver.DecodeAndValidate(r, &req); err != nil {
		httpserver.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	_, token, err := h.service.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		httpserver.Fail(w, r, err)
		return
	}
	httpserver.OK(w, http.StatusOK, loginResponse{Token: token})
}

// SettleHolding godoc
// @Summary      Force-settle a Salak holding
// @Description  Immediately pays out a holding's principal + interest to its owning user's primary account, regardless of its real maturity date. Admin-only; bypasses the maturity date entirely - a demo/ops action, not the eventual production trigger.
// @Tags         admin
// @Produce      json
// @Param        id  path      string  true  "Holding ID"
// @Success      200 {object}  httpserver.DataEnvelope{data=settlementResponse}
// @Failure      404 {object}  httpserver.ErrorEnvelope
// @Failure      409 {object}  httpserver.ErrorEnvelope
// @Router       /api/v1/admin/holdings/{id}/settle [post]
func (h *Handler) SettleHolding(w http.ResponseWriter, r *http.Request) {
	holdingID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpserver.Error(w, http.StatusBadRequest, "invalid holding id")
		return
	}

	receipt, err := h.kapook.SettleMaturedHolding(r.Context(), holdingID)
	if err != nil {
		httpserver.Fail(w, r, err)
		return
	}
	httpserver.OK(w, http.StatusOK, toSettlementResponse(receipt))
}
