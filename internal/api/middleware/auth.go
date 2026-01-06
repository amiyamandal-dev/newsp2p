package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/amiyamandal-dev/newsp2p/internal/auth"
	"github.com/amiyamandal-dev/newsp2p/pkg/response"
)

// AuthMiddleware creates JWT authentication middleware
func AuthMiddleware(jwtManager *auth.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Unauthorized(c, "Missing authorization header")
			c.Abort()
			return
		}

		// Extract token from "Bearer <token>" format
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			response.Unauthorized(c, "Invalid authorization header format")
			c.Abort()
			return
		}

		token := parts[1]

		// Validate token
		claims, err := jwtManager.ValidateToken(token)
		if err != nil {
			response.Unauthorized(c, "Invalid or expired token")
			c.Abort()
			return
		}

		// Set user claims in context
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("email", claims.Email)

		c.Next()
	}
}

// GetUserID retrieves the user ID from the request context
func GetUserID(c *gin.Context) string {
	userID, _ := c.Get("user_id")
	if userID == nil {
		return ""
	}
	return userID.(string)
}

// GetUsername retrieves the username from the request context
func GetUsername(c *gin.Context) string {
	username, _ := c.Get("username")
	if username == nil {
		return ""
	}
	return username.(string)
}
