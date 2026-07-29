package apperror

import "net/http"

type Kind string

const (
	KindValidation   Kind = "validation"
	KindNotFound     Kind = "not_found"
	KindUnauthorized Kind = "unauthorized"
	KindForbidden    Kind = "forbidden"
	KindConflict     Kind = "conflict"
	KindInternal     Kind = "internal"
)

type Error struct {
	Kind    Kind
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *Error) Unwrap() error {
	return e.Err
}

func New(kind Kind, message string) *Error {
	return &Error{Kind: kind, Message: message}
}

func Wrap(kind Kind, message string, err error) *Error {
	return &Error{Kind: kind, Message: message, Err: err}
}

func Validation(message string) *Error   { return New(KindValidation, message) }
func NotFound(message string) *Error     { return New(KindNotFound, message) }
func Unauthorized(message string) *Error { return New(KindUnauthorized, message) }
func Forbidden(message string) *Error    { return New(KindForbidden, message) }
func Conflict(message string) *Error     { return New(KindConflict, message) }
func Internal(message string, err error) *Error {
	return Wrap(KindInternal, message, err)
}

func HTTPStatus(err error) int {
	appErr, ok := err.(*Error)
	if !ok {
		return http.StatusInternalServerError
	}
	switch appErr.Kind {
	case KindValidation:
		return http.StatusBadRequest
	case KindNotFound:
		return http.StatusNotFound
	case KindUnauthorized:
		return http.StatusUnauthorized
	case KindForbidden:
		return http.StatusForbidden
	case KindConflict:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
