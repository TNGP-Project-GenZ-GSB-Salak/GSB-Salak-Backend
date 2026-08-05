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

func newAdminNextHandler(t *testing.T, called *bool, wantAdminID uuid.UUID) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*called = true
		got, ok := middleware.AdminIDFromContext(r.Context())
		require.True(t, ok)
		assert.Equal(t, wantAdminID, got)
		w.WriteHeader(http.StatusOK)
	})
}

func TestAdminAuth_MissingHeader(t *testing.T) {
	signer := jwtutil.NewAdminSigner("admin-secret", 60)
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	middleware.AdminAuth(signer)(next).ServeHTTP(rec, req)

	assert.False(t, called)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAdminAuth_NonBearerHeader(t *testing.T) {
	signer := jwtutil.NewAdminSigner("admin-secret", 60)
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

			middleware.AdminAuth(signer)(next).ServeHTTP(rec, req)

			assert.False(t, called)
			assert.Equal(t, http.StatusUnauthorized, rec.Code)
		})
	}
}

func TestAdminAuth_InvalidToken(t *testing.T) {
	signer := jwtutil.NewAdminSigner("admin-secret", 60)
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer not-a-real-token")
	rec := httptest.NewRecorder()

	middleware.AdminAuth(signer)(next).ServeHTTP(rec, req)

	assert.False(t, called)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAdminAuth_ExpiredToken(t *testing.T) {
	signer := jwtutil.NewAdminSigner("admin-secret", -1)
	token, err := signer.Sign(uuid.New())
	require.NoError(t, err)

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	middleware.AdminAuth(signer)(next).ServeHTTP(rec, req)

	assert.False(t, called)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// TestAdminAuth_CustomerTokenRejected is AdminAuth's whole reason to exist
// as a distinct middleware rather than a role check bolted onto Auth: a
// perfectly valid customer JWT must never pass this gate, because it's
// signed with JWTSecret, not ADMIN_JWT_SECRET - the signature check fails
// before AdminClaims decoding is ever reached.
func TestAdminAuth_CustomerTokenRejected(t *testing.T) {
	customerSigner := jwtutil.NewSigner("customer-secret", 60)
	adminSigner := jwtutil.NewAdminSigner("admin-secret", 60)

	customerToken, err := customerSigner.Sign(uuid.New())
	require.NoError(t, err)

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+customerToken)
	rec := httptest.NewRecorder()

	middleware.AdminAuth(adminSigner)(next).ServeHTTP(rec, req)

	assert.False(t, called)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAdminAuth_ValidToken_SetsContextAndCallsNext(t *testing.T) {
	signer := jwtutil.NewAdminSigner("admin-secret", 60)
	adminID := uuid.New()
	token, err := signer.Sign(adminID)
	require.NoError(t, err)

	called := false
	next := newAdminNextHandler(t, &called, adminID)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	middleware.AdminAuth(signer)(next).ServeHTTP(rec, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAdminIDFromContext(t *testing.T) {
	t.Run("absent", func(t *testing.T) {
		got, ok := middleware.AdminIDFromContext(context.Background())
		assert.False(t, ok)
		assert.Equal(t, uuid.Nil, got)
	})

	t.Run("present via valid token flow", func(t *testing.T) {
		signer := jwtutil.NewAdminSigner("admin-secret", 60)
		adminID := uuid.New()
		token, err := signer.Sign(adminID)
		require.NoError(t, err)

		var gotCtx context.Context
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotCtx = r.Context()
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		middleware.AdminAuth(signer)(next).ServeHTTP(rec, req)

		got, ok := middleware.AdminIDFromContext(gotCtx)
		assert.True(t, ok)
		assert.Equal(t, adminID, got)
	})

	t.Run("wrong type stored under the same key is not exposed", func(t *testing.T) {
		// AdminIDFromContext uses an unexported adminCtxKey (its own type,
		// distinct from the customer ctxKey) - this documents that an
		// external caller cannot forge a colliding key.
		ctx := context.WithValue(context.Background(), struct{ k string }{"adminIDKey"}, "not-a-uuid")
		got, ok := middleware.AdminIDFromContext(ctx)
		assert.False(t, ok)
		assert.Equal(t, uuid.Nil, got)
	})
}
