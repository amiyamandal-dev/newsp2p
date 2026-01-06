package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	limiter "github.com/ulule/limiter/v3"
	mgin "github.com/ulule/limiter/v3/drivers/middleware/gin"
	"github.com/ulule/limiter/v3/drivers/store/memory"

	"github.com/amiyamandal-dev/newsp2p/pkg/response"
)

// RateLimitMiddleware creates rate limiting middleware
func RateLimitMiddleware(requestsPerMinute, burst int) gin.HandlerFunc {
	// Create rate limiter
	rate := limiter.Rate{
		Period: 1 * time.Minute,
		Limit:  int64(requestsPerMinute),
	}

	store := memory.NewStore()
	instance := limiter.New(store, rate)

	// Create Gin middleware
	middleware := mgin.NewMiddleware(instance)

	return func(c *gin.Context) {
		// Use the middleware
		ginContext := middleware(c)
		if ginContext == nil {
			return // Rate limit exceeded
		}

		// Check if rate limit was exceeded
		rateLimitContext := limiter.GetContext(c.Request.Context())
		if rateLimitContext.Reached {
			response.Error(c, 429, "Rate limit exceeded. Please try again later.")
			c.Abort()
			return
		}

		c.Next()
	}
}
