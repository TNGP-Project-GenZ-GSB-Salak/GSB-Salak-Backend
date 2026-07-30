package httpserver

import (
	"context"
	"errors"
	"log"
	"net/http"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/apperror"
	"github.com/gin-gonic/gin"
)

// statusClientClosedRequest (nginx's 499 convention) is used when the
// client disconnected/navigated away before we finished, rather than
// reporting a request the client never actually failed to receive as a 500.
const statusClientClosedRequest = 499

func OK(c *gin.Context, code int, data any) {
	c.JSON(code, gin.H{"data": data})
}

func Fail(c *gin.Context, err error) {
	if errors.Is(err, context.Canceled) {
		c.AbortWithStatus(statusClientClosedRequest)
		return
	}

	status := apperror.HTTPStatus(err)
	message := err.Error()

	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		message = appErr.Message
	}

	// The client only ever sees the safe, generic message above; the full
	// error (with any wrapped cause) is logged server-side so 5xx responses
	// are diagnosable instead of a bare status code in the access log.
	if status >= http.StatusInternalServerError {
		log.Printf("%s %s -> %d: %v", c.Request.Method, c.Request.URL.Path, status, err)
	}

	c.JSON(status, gin.H{"error": message})
}
