package middleware

import (
	"time"

	"github.com/amiyamandal-dev/newsp2p/pkg/logger"
	"github.com/gin-gonic/gin"
)

// LoggerMiddleware creates request logging middleware
func LoggerMiddleware(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()

		// Process request
		c.Next()

		// Log request details
		duration := time.Since(startTime)
		statusCode := c.Writer.Status()

		log.Info("HTTP Request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", statusCode,
			"duration_ms", duration.Milliseconds(),
			"client_ip", c.ClientIP(),
			"user_agent", c.Request.UserAgent(),
		)

		// Log errors if any
		if len(c.Errors) > 0 {
			log.Error("Request errors", "errors", c.Errors.String())
		}
	}
}
