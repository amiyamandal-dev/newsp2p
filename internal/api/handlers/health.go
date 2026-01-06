package handlers

import (
	"context"

	"github.com/gin-gonic/gin"

	"github.com/amiyamandal-dev/newsp2p/internal/ipfs"
	"github.com/amiyamandal-dev/newsp2p/internal/repository/sqlite"
	"github.com/amiyamandal-dev/newsp2p/internal/search"
	"github.com/amiyamandal-dev/newsp2p/pkg/logger"
)

// HealthHandler handles health check requests
type HealthHandler struct {
	db          *sqlite.DB
	ipfsClient  *ipfs.Client
	searchIndex search.Index
	logger      *logger.Logger
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(db *sqlite.DB, ipfsClient *ipfs.Client, searchIndex search.Index, logger *logger.Logger) *HealthHandler {
	return &HealthHandler{
		db:          db,
		ipfsClient:  ipfsClient,
		searchIndex: searchIndex,
		logger:      logger.WithComponent("health-handler"),
	}
}

// Health returns basic health status
func (h *HealthHandler) Health(c *gin.Context) {
	c.JSON(200, gin.H{
		"status": "ok",
	})
}

// Readiness checks if the service is ready to handle requests
func (h *HealthHandler) Readiness(c *gin.Context) {
	ctx := context.Background()
	checks := make(map[string]bool)

	// Check database
	checks["database"] = h.db.HealthCheck() == nil

	// Check IPFS
	checks["ipfs"] = h.ipfsClient.IsHealthy(ctx)

	// Check search index
	_, err := h.searchIndex.Count()
	checks["search"] = err == nil

	// Overall status
	ready := checks["database"] && checks["ipfs"] && checks["search"]

	status := "ready"
	code := 200
	if !ready {
		status = "not ready"
		code = 503
	}

	c.JSON(code, gin.H{
		"status": status,
		"checks": checks,
	})
}

// Liveness checks if the service is alive
func (h *HealthHandler) Liveness(c *gin.Context) {
	c.JSON(200, gin.H{
		"status": "alive",
	})
}
