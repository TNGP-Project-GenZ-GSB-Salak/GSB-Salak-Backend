package httpserver

import (
	"errors"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/apperror"
	"github.com/gin-gonic/gin"
)

func OK(c *gin.Context, code int, data any) {
	c.JSON(code, gin.H{"data": data})
}

func Fail(c *gin.Context, err error) {
	status := apperror.HTTPStatus(err)
	message := err.Error()

	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		message = appErr.Message
	}

	c.JSON(status, gin.H{"error": message})
}
