package handlers

import (
	"context"
	"sync"

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

	var (
		dbHealthy     bool
		ipfsHealthy   bool
		searchHealthy bool
		searchCount   uint64
		wg            sync.WaitGroup
	)

	wg.Add(3)

	// Check database (required) - in parallel
	go func() {
		defer wg.Done()
		dbHealthy = h.db.HealthCheck() == nil
	}()

	// Check IPFS (optional) - in parallel
	go func() {
		defer wg.Done()
		ipfsHealthy = h.ipfsClient.IsHealthy(ctx)
	}()

	// Check search index (required) - in parallel
	go func() {
		defer wg.Done()
		var err error
		searchCount, err = h.searchIndex.Count()
		searchHealthy = err == nil
	}()

	wg.Wait()

	checks := map[string]interface{}{
		"database": map[string]interface{}{
			"healthy":  dbHealthy,
			"required": true,
		},
		"ipfs": map[string]interface{}{
			"healthy":  ipfsHealthy,
			"required": false,
			"note":     "Optional - some features unavailable if offline",
		},
		"search": map[string]interface{}{
			"healthy":        searchHealthy,
			"required":       true,
			"document_count": searchCount,
		},
	}

	// Overall status - only required services must be healthy
	ready := dbHealthy && searchHealthy

	status := "ready"
	code := 200
	if !ready {
		status = "not ready"
		code = 503
	}

	response := gin.H{
		"status": status,
		"checks": checks,
	}

	// Add warning if IPFS is down
	if !ipfsHealthy {
		response["warnings"] = []string{"IPFS not available - article uploads and IPNS features disabled"}
	}

	c.JSON(code, response)
}

// Liveness checks if the service is alive
func (h *HealthHandler) Liveness(c *gin.Context) {
	c.JSON(200, gin.H{
		"status": "alive",
	})
}
