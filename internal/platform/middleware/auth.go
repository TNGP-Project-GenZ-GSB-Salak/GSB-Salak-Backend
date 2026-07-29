package middleware

import (
	"net/http"
	"strings"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/jwtutil"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const UserIDKey = "userID"

func Auth(signer *jwtutil.Signer) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}

		tokenString := strings.TrimPrefix(header, "Bearer ")
		userID, err := signer.Parse(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		c.Set(UserIDKey, userID)
		c.Next()
	}
}

func UserIDFromContext(c *gin.Context) (uuid.UUID, bool) {
	v, ok := c.Get(UserIDKey)
	if !ok {
		return uuid.Nil, false
	}
	id, ok := v.(uuid.UUID)
	return id, ok
}
