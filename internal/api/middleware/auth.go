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
		var token string

		// Get token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			// Extract token from "Bearer <token>" format
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && parts[0] == "Bearer" {
				token = parts[1]
			}
		}

		// If header missing or invalid, check cookie
		if token == "" {
			cookieToken, err := c.Cookie("access_token")
			if err == nil && cookieToken != "" {
				token = cookieToken
			}
		}

		// If still no token, unauthorized
		if token == "" {
			response.Unauthorized(c, "Missing authorization")
			c.Abort()
			return
		}

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
	userID, exists := c.Get("user_id")
	if !exists {
		return ""
	}
	str, ok := userID.(string)
	if !ok {
		return ""
	}
	return str
}

// GetUsername retrieves the username from the request context
func GetUsername(c *gin.Context) string {
	username, exists := c.Get("username")
	if !exists {
		return ""
	}
	str, ok := username.(string)
	if !ok {
		return ""
	}
	return str
}
