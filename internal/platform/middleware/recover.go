package middleware

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func Recover() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		log.Printf("panic recovered: %v", recovered)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	})
}
