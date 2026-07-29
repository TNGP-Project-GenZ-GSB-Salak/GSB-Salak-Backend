package httpserver

import (
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/middleware"
	"github.com/gin-gonic/gin"
)

func NewRouter() *gin.Engine {
	r := gin.New()
	r.Use(middleware.Recover(), middleware.RequestLog(), middleware.CORS())

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Explicit catch-all so CORS preflight requests always get a 204
	// with the headers set by middleware.CORS(), regardless of whether
	// the actual path/method is registered.
	r.OPTIONS("/*any", func(c *gin.Context) {
		c.Status(204)
	})

	return r
}
