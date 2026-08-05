package apperror_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/apperror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestError_Error(t *testing.T) {
	t.Run("without wrapped err", func(t *testing.T) {
		e := apperror.New(apperror.KindValidation, "bad input")
		assert.Equal(t, "bad input", e.Error())
	})

	t.Run("with wrapped err appends cause", func(t *testing.T) {
		cause := errors.New("db exploded")
		e := apperror.Wrap(apperror.KindInternal, "failed to save", cause)
		assert.Equal(t, "failed to save: db exploded", e.Error())
	})
}

func TestError_Unwrap(t *testing.T) {
	t.Run("returns wrapped error", func(t *testing.T) {
		cause := errors.New("root cause")
		e := apperror.Wrap(apperror.KindInternal, "wrapper", cause)
		assert.Same(t, cause, errors.Unwrap(e))
	})

	t.Run("nil when no wrapped error", func(t *testing.T) {
		e := apperror.New(apperror.KindValidation, "bad input")
		assert.Nil(t, errors.Unwrap(e))
	})

	t.Run("errors.Is sees through to wrapped cause", func(t *testing.T) {
		cause := errors.New("sentinel")
		e := apperror.Wrap(apperror.KindInternal, "wrapper", cause)
		assert.True(t, errors.Is(e, cause))
	})
}

func TestConstructors(t *testing.T) {
	cause := errors.New("boom")

	cases := []struct {
		name     string
		err      *apperror.Error
		wantKind apperror.Kind
		wantMsg  string
		wantErr  error
	}{
		{"Validation", apperror.Validation("v"), apperror.KindValidation, "v", nil},
		{"NotFound", apperror.NotFound("nf"), apperror.KindNotFound, "nf", nil},
		{"Unauthorized", apperror.Unauthorized("u"), apperror.KindUnauthorized, "u", nil},
		{"Forbidden", apperror.Forbidden("f"), apperror.KindForbidden, "f", nil},
		{"Conflict", apperror.Conflict("c"), apperror.KindConflict, "c", nil},
		{"Internal", apperror.Internal("i", cause), apperror.KindInternal, "i", cause},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.NotNil(t, tc.err)
			assert.Equal(t, tc.wantKind, tc.err.Kind)
			assert.Equal(t, tc.wantMsg, tc.err.Message)
			assert.Equal(t, tc.wantErr, tc.err.Err)
		})
	}
}

func TestError_WithCode(t *testing.T) {
	t.Run("sets Code and returns the same error for chaining", func(t *testing.T) {
		e := apperror.Validation("amount must be greater than zero")
		got := e.WithCode("kapook_amount_must_be_positive")
		assert.Same(t, e, got)
		assert.Equal(t, "kapook_amount_must_be_positive", e.Code)
	})

	t.Run("zero value when never called", func(t *testing.T) {
		e := apperror.Validation("bad input")
		assert.Empty(t, e.Code)
	})
}

func TestHTTPStatus(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"validation -> 400", apperror.Validation("x"), http.StatusBadRequest},
		{"not_found -> 404", apperror.NotFound("x"), http.StatusNotFound},
		{"unauthorized -> 401", apperror.Unauthorized("x"), http.StatusUnauthorized},
		{"forbidden -> 403", apperror.Forbidden("x"), http.StatusForbidden},
		{"conflict -> 409", apperror.Conflict("x"), http.StatusConflict},
		{"internal -> 500", apperror.Internal("x", errors.New("y")), http.StatusInternalServerError},
		{"plain non-apperror error -> 500", errors.New("plain"), http.StatusInternalServerError},
		{"nil error -> 500", nil, http.StatusInternalServerError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, apperror.HTTPStatus(tc.err))
		})
	}
}

func TestHTTPStatus_UnknownKindDefaultsTo500(t *testing.T) {
	e := apperror.New(apperror.Kind("something_new"), "mystery")
	assert.Equal(t, http.StatusInternalServerError, apperror.HTTPStatus(e))
}
