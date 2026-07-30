package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/middleware"
	"github.com/stretchr/testify/assert"
)

func TestCORS_SetsHeadersAndCallsNext(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	middleware.CORS()(next).ServeHTTP(rec, req)

	assert.True(t, called)
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, POST, PUT, PATCH, DELETE, OPTIONS", rec.Header().Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Content-Type, Authorization", rec.Header().Get("Access-Control-Allow-Headers"))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestCORS_PreflightOptionsShortCircuits(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	req := httptest.NewRequest(http.MethodOptions, "/anything", nil)
	rec := httptest.NewRecorder()

	middleware.CORS()(next).ServeHTTP(rec, req)

	assert.False(t, called, "OPTIONS preflight must not reach the wrapped handler")
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"), "preflight response still needs CORS headers")
}
