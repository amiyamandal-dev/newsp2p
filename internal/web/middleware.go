package web

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/amiyamandal-dev/newsp2p/internal/auth"
	"github.com/amiyamandal-dev/newsp2p/internal/domain"
	"github.com/amiyamandal-dev/newsp2p/internal/service"
)

const (
	CookieAccessToken = "access_token"
	ContextUserKey    = "web_user"
)

// AuthMiddleware handles authentication for web routes via cookies
func AuthMiddleware(jwtManager *auth.JWTManager, userService *service.UserService) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString, err := c.Cookie(CookieAccessToken)
		if err != nil {
			// No cookie, user is not logged in. Continue without user in context.
			c.Next()
			return
		}

		claims, err := jwtManager.ValidateToken(tokenString)
		if err != nil {
			// Invalid token, clear cookie
			c.SetCookie(CookieAccessToken, "", -1, "/", "", false, true)
			c.Next()
			return
		}

		// Get user details
		user, err := userService.GetUser(c.Request.Context(), claims.UserID)
		if err != nil {
			// User not found or error, clear cookie
			c.SetCookie(CookieAccessToken, "", -1, "/", "", false, true)
			c.Next()
			return
		}

		// Set user in context
		c.Set(ContextUserKey, user)
		c.Next()
	}
}

// GetUser returns the authenticated user from context, if any
func GetUser(c *gin.Context) *domain.UserResponse {
	user, exists := c.Get(ContextUserKey)
	if !exists {
		return nil
	}
	return user.(*domain.UserResponse)
}

// RequireAuth middleware ensures a user is logged in
func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		_, exists := c.Get(ContextUserKey)
		if !exists {
			c.Redirect(http.StatusSeeOther, "/login")
			c.Abort()
			return
		}
		c.Next()
	}
}
