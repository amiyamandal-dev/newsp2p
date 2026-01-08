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
// Health check endpoints are exempt from rate limiting
func RateLimitMiddleware(requestsPerMinute, burst int) gin.HandlerFunc {
	// Create rate limiter
	rate := limiter.Rate{
		Period: 1 * time.Minute,
		Limit:  int64(requestsPerMinute),
	}

	store := memory.NewStore()
	instance := limiter.New(store, rate)

	// Create Gin middleware with custom error handler
	middleware := mgin.NewMiddleware(instance, mgin.WithErrorHandler(func(c *gin.Context, err error) {
		response.Error(c, 429, "Rate limit exceeded. Please try again later.")
		c.Abort()
	}), mgin.WithKeyGetter(func(c *gin.Context) string {
		// Use client IP as the key for rate limiting
		return c.ClientIP()
	}))

	return middleware
}
