package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/jwtutil"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/middleware"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newNextHandler(t *testing.T, called *bool, wantUserID uuid.UUID) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*called = true
		got, ok := middleware.UserIDFromContext(r.Context())
		require.True(t, ok)
		assert.Equal(t, wantUserID, got)
		w.WriteHeader(http.StatusOK)
	})
}

func TestAuth_MissingHeader(t *testing.T) {
	signer := jwtutil.NewSigner("secret", 60)
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	middleware.Auth(signer)(next).ServeHTTP(rec, req)

	assert.False(t, called)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuth_NonBearerHeader(t *testing.T) {
	signer := jwtutil.NewSigner("secret", 60)
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	cases := []string{
		"Basic dXNlcjpwYXNz",
		"Bearertoken-with-no-space",
		"bearer lowercase-scheme",
	}

	for _, header := range cases {
		t.Run(header, func(t *testing.T) {
			called = false
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Authorization", header)
			rec := httptest.NewRecorder()

			middleware.Auth(signer)(next).ServeHTTP(rec, req)

			assert.False(t, called)
			assert.Equal(t, http.StatusUnauthorized, rec.Code)
		})
	}
}

func TestAuth_InvalidToken(t *testing.T) {
	signer := jwtutil.NewSigner("secret", 60)
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer not-a-real-token")
	rec := httptest.NewRecorder()

	middleware.Auth(signer)(next).ServeHTTP(rec, req)

	assert.False(t, called)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuth_ExpiredToken(t *testing.T) {
	signer := jwtutil.NewSigner("secret", -1)
	token, err := signer.Sign(uuid.New())
	require.NoError(t, err)

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	middleware.Auth(signer)(next).ServeHTTP(rec, req)

	assert.False(t, called)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuth_ValidToken_SetsContextAndCallsNext(t *testing.T) {
	signer := jwtutil.NewSigner("secret", 60)
	userID := uuid.New()
	token, err := signer.Sign(userID)
	require.NoError(t, err)

	called := false
	next := newNextHandler(t, &called, userID)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	middleware.Auth(signer)(next).ServeHTTP(rec, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestUserIDFromContext(t *testing.T) {
	t.Run("absent", func(t *testing.T) {
		got, ok := middleware.UserIDFromContext(context.Background())
		assert.False(t, ok)
		assert.Equal(t, uuid.Nil, got)
	})

	t.Run("present via valid token flow", func(t *testing.T) {
		signer := jwtutil.NewSigner("secret", 60)
		userID := uuid.New()
		token, err := signer.Sign(userID)
		require.NoError(t, err)

		var gotCtx context.Context
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotCtx = r.Context()
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		middleware.Auth(signer)(next).ServeHTTP(rec, req)

		got, ok := middleware.UserIDFromContext(gotCtx)
		assert.True(t, ok)
		assert.Equal(t, userID, got)
	})

	t.Run("wrong type stored under the same key is not exposed", func(t *testing.T) {
		// UserIDFromContext uses an unexported ctxKey, so external callers
		// cannot forge a colliding key; this documents that guarantee.
		ctx := context.WithValue(context.Background(), struct{ k string }{"userIDKey"}, "not-a-uuid")
		got, ok := middleware.UserIDFromContext(ctx)
		assert.False(t, ok)
		assert.Equal(t, uuid.Nil, got)
	})
}
