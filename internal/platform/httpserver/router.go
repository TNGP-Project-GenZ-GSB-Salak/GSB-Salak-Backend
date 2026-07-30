package httpserver

import (
	"encoding/json"
	"net/http"

	_ "github.com/ciaabcdefg/gsb-salak-backend/docs"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/middleware"
	"github.com/go-chi/chi/v5"
	httpSwagger "github.com/swaggo/http-swagger"
)

func NewRouter() *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Recover(), middleware.RequestLog(), middleware.CORS())

	r.Get("/healthz", healthz)
	r.Get("/swagger/*", httpSwagger.WrapHandler)

	// Explicit catch-all so CORS preflight requests always get a 204
	// with the headers set by middleware.CORS(), regardless of whether
	// the actual path/method is registered.
	r.Options("/*", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	return r
}

// healthz godoc
// @Summary  Health check
// @Tags     health
// @Produce  json
// @Success  200  {object}  map[string]string
// @Router   /healthz [get]
func healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
