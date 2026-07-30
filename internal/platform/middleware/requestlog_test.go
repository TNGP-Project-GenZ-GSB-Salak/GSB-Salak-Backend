package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/middleware"
	"github.com/stretchr/testify/assert"
)

func TestRequestLog_PassesThroughStatusAndBody(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	})

	req := httptest.NewRequest(http.MethodPost, "/accounts", nil)
	rec := httptest.NewRecorder()

	middleware.RequestLog()(next).ServeHTTP(rec, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "ok", rec.Body.String())
}

func TestRequestLog_DefaultsTo200WhenHandlerNeverWritesStatus(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("implicit 200"))
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	middleware.RequestLog()(next).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}
