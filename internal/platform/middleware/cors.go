package middleware

import "github.com/gin-gonic/gin"

// CORS allows the standalone static frontend (served from a different
// origin/port) to call the API. Bearer tokens are used instead of cookies,
// so a permissive wildcard origin carries no credential-leak risk.
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
