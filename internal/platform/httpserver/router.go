package httpserver

import (
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/middleware"
	"github.com/gin-gonic/gin"
)

func NewRouter() *gin.Engine {
	r := gin.New()
	r.Use(middleware.Recover(), middleware.RequestLog())

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	return r
}
