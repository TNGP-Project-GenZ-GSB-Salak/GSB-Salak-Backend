package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/jwtutil"
	"github.com/google/uuid"
)

// adminCtxKey is its own type (not ctxKey) so an admin id can never collide
// with - or be confused for - the customer userIDKey, even accidentally.
type adminCtxKey int

const adminIDKey adminCtxKey = iota

// AdminAuth mirrors Auth exactly (Bearer-parsing, 401 on missing/invalid)
// but against AdminSigner.Parse - a distinct secret from the customer
// Signer, so an ordinary user's valid token can never pass this gate.
func AdminAuth(signer *jwtutil.AdminSigner) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" || !strings.HasPrefix(header, "Bearer ") {
				writeJSONError(w, http.StatusUnauthorized, "missing bearer token")
				return
			}

			tokenString := strings.TrimPrefix(header, "Bearer ")
			adminID, err := signer.Parse(tokenString)
			if err != nil {
				writeJSONError(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}

			ctx := context.WithValue(r.Context(), adminIDKey, adminID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func AdminIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	v := ctx.Value(adminIDKey)
	if v == nil {
		return uuid.Nil, false
	}
	id, ok := v.(uuid.UUID)
	return id, ok
}
