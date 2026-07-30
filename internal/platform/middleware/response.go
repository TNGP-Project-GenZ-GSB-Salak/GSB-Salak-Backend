package middleware

import (
	"encoding/json"
	"net/http"
)

// writeJSONError is a minimal JSON error writer shared by middleware that
// must respond before a request reaches any handler (auth, recovery), kept
// local to this package so middleware stays a leaf with no httpserver import.
func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
