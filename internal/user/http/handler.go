package http

import (
	"net/http"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/httpserver"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/user"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service user.Service
}

func NewHandler(service user.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/auth/register", h.Register)
	r.Post("/auth/login", h.Login)
}

// Register godoc
// @Summary      Register a new user
// @Description  Creates a new user account.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request  body      registerRequest  true  "Registration details"
// @Success      201      {object}  httpserver.DataEnvelope{data=userResponse}
// @Failure      400      {object}  httpserver.ErrorEnvelope
// @Router       /api/v1/auth/register [post]
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := httpserver.DecodeAndValidate(r, &req); err != nil {
		httpserver.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	u, err := h.service.Register(r.Context(), req.Username, req.Password, req.FullName)
	if err != nil {
		httpserver.Fail(w, r, err)
		return
	}
	httpserver.OK(w, http.StatusCreated, toUserResponse(u))
}

// Login godoc
// @Summary      Log in
// @Description  Authenticates a user and returns a JWT bearer token.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request  body      loginRequest  true  "Login credentials"
// @Success      200      {object}  httpserver.DataEnvelope{data=loginResponse}
// @Failure      400      {object}  httpserver.ErrorEnvelope
// @Failure      401      {object}  httpserver.ErrorEnvelope
// @Router       /api/v1/auth/login [post]
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httpserver.DecodeAndValidate(r, &req); err != nil {
		httpserver.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	u, token, err := h.service.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		httpserver.Fail(w, r, err)
		return
	}
	httpserver.OK(w, http.StatusOK, loginResponse{User: toUserResponse(u), Token: token})
}
