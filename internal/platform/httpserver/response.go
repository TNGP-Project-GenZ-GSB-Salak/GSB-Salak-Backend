package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/apperror"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/middleware"
	"github.com/google/uuid"
)

// statusClientClosedRequest (nginx's 499 convention) is used when the
// client disconnected/navigated away before we finished, rather than
// reporting a request the client never actually failed to receive as a 500.
const statusClientClosedRequest = 499

func OK(w http.ResponseWriter, code int, data any) {
	writeJSON(w, code, map[string]any{"data": data})
}

func Error(w http.ResponseWriter, code int, message string) {
	writeJSON(w, code, map[string]any{"error": message})
}

// ErrorWithCode is Error's superset: it additionally serialises a stable
// machine-readable errCode the client maps to localized copy, omitted from
// the envelope entirely when empty so a client can tell "no code assigned"
// apart from an empty string and fall back to a generic per-Kind message.
func ErrorWithCode(w http.ResponseWriter, code int, message string, errCode string) {
	body := map[string]any{"error": message}
	if errCode != "" {
		body["code"] = errCode
	}
	writeJSON(w, code, body)
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(body)
}

func Fail(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, context.Canceled) {
		w.WriteHeader(statusClientClosedRequest)
		return
	}

	status := apperror.HTTPStatus(err)
	message := err.Error()
	code := ""

	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		message = appErr.Message
		code = appErr.Code
	}

	// The client only ever sees the safe, generic message (and code) above;
	// the full error (with any wrapped cause) is logged server-side so 5xx
	// responses are diagnosable instead of a bare status code in the access log.
	if status >= http.StatusInternalServerError {
		log.Printf("%s %s -> %d: %v", r.Method, r.URL.Path, status, err)
	}

	ErrorWithCode(w, status, message, code)
}

// RequireUserID pulls the authenticated user id set by middleware.Auth out of
// the request context, writing a 401 response itself when it's missing.
func RequireUserID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "unauthorized")
		return uuid.Nil, false
	}
	return userID, true
}
