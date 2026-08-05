package httpserver_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/apperror"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/httpserver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body
}

func TestErrorWithCode(t *testing.T) {
	t.Run("omits the code key entirely when errCode is empty", func(t *testing.T) {
		rec := httptest.NewRecorder()
		httpserver.ErrorWithCode(rec, http.StatusBadRequest, "bad input", "")

		body := decodeBody(t, rec)
		assert.Equal(t, "bad input", body["error"])
		_, hasCode := body["code"]
		assert.False(t, hasCode)
	})

	t.Run("includes the code key when errCode is set", func(t *testing.T) {
		rec := httptest.NewRecorder()
		httpserver.ErrorWithCode(rec, http.StatusConflict, "an active goal already exists for this account", "kapook_goal_already_exists")

		body := decodeBody(t, rec)
		assert.Equal(t, "an active goal already exists for this account", body["error"])
		assert.Equal(t, "kapook_goal_already_exists", body["code"])
	})
}

func TestFail(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/kapook/goals", nil)

	t.Run("apperror with a code serialises message and code", func(t *testing.T) {
		rec := httptest.NewRecorder()
		err := apperror.Conflict("an active goal already exists for this account").WithCode("kapook_goal_already_exists")
		httpserver.Fail(rec, req, err)

		assert.Equal(t, http.StatusConflict, rec.Code)
		body := decodeBody(t, rec)
		assert.Equal(t, "an active goal already exists for this account", body["error"])
		assert.Equal(t, "kapook_goal_already_exists", body["code"])
	})

	t.Run("apperror with no code omits the code key, falling back to Kind client-side via the status", func(t *testing.T) {
		rec := httptest.NewRecorder()
		err := apperror.Validation("account_id must reference a kapook-type account")
		httpserver.Fail(rec, req, err)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		body := decodeBody(t, rec)
		assert.Equal(t, "account_id must reference a kapook-type account", body["error"])
		_, hasCode := body["code"]
		assert.False(t, hasCode)
	})

	t.Run("plain non-apperror error has no code and a 500", func(t *testing.T) {
		rec := httptest.NewRecorder()
		httpserver.Fail(rec, req, assert.AnError)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		body := decodeBody(t, rec)
		_, hasCode := body["code"]
		assert.False(t, hasCode)
	})
}
