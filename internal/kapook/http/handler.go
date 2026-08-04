package http

import (
	"net/http"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/kapook"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/httpserver"
	"github.com/go-chi/chi/v5"
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
