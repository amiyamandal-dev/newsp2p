package middleware

import (
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/amiyamandal-dev/newsp2p/pkg/response"
)

// rateLimitEntry tracks requests for a single client
type rateLimitEntry struct {
	count     int
	resetTime time.Time
}

// RateLimiter implements a simple in-memory rate limiter
type RateLimiter struct {
	mu               sync.RWMutex
	clients          map[string]*rateLimitEntry
	requestsPerMin   int
	cleanupInterval  time.Duration
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(requestsPerMinute int) *RateLimiter {
	rl := &RateLimiter{
		clients:         make(map[string]*rateLimitEntry),
		requestsPerMin:  requestsPerMinute,
		cleanupInterval: 5 * time.Minute,
	}
	// Start cleanup goroutine
	go rl.cleanup()
	return rl
}

// cleanup removes expired entries periodically
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.cleanupInterval)
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for key, entry := range rl.clients {
			if now.After(entry.resetTime) {
				delete(rl.clients, key)
			}
		}
		rl.mu.Unlock()
	}
}

// Allow checks if a request is allowed for the given client
func (rl *RateLimiter) Allow(clientIP string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	entry, exists := rl.clients[clientIP]

	if !exists || now.After(entry.resetTime) {
		// New client or window expired - create new entry
		rl.clients[clientIP] = &rateLimitEntry{
			count:     1,
			resetTime: now.Add(time.Minute),
		}
		return true
	}

	// Check if limit exceeded
	if entry.count >= rl.requestsPerMin {
		return false
	}

	// Increment count
	entry.count++
	return true
}

// RateLimitMiddleware creates rate limiting middleware
func RateLimitMiddleware(requestsPerMinute, burst int) gin.HandlerFunc {
	// Combine requests and burst for effective limit
	effectiveLimit := requestsPerMinute
	if burst > 0 {
		effectiveLimit = requestsPerMinute + burst
	}

	limiter := NewRateLimiter(effectiveLimit)

	return func(c *gin.Context) {
		clientIP := c.ClientIP()

		if !limiter.Allow(clientIP) {
			response.Error(c, 429, "Rate limit exceeded. Please try again later.")
			c.Abort()
			return
		}

		c.Next()
	}
}
