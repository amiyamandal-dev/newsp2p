package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/amiyamandal-dev/newsp2p/internal/domain"
	"github.com/amiyamandal-dev/newsp2p/internal/service"
	"github.com/amiyamandal-dev/newsp2p/pkg/logger"
	"github.com/amiyamandal-dev/newsp2p/pkg/response"
)

// FeedHandler handles feed-related requests
type FeedHandler struct {
	feedService *service.FeedService
	syncService *service.SyncService
	logger      *logger.Logger
}

// NewFeedHandler creates a new feed handler
func NewFeedHandler(feedService *service.FeedService, syncService *service.SyncService, logger *logger.Logger) *FeedHandler {
	return &FeedHandler{
		feedService: feedService,
		syncService: syncService,
		logger:      logger.WithComponent("feed-handler"),
	}
}

// List retrieves all feeds
func (h *FeedHandler) List(c *gin.Context) {
	feeds, err := h.feedService.List(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to list feeds", "error", err)
		response.InternalServerError(c, "Failed to list feeds")
		return
	}

	response.Success(c, feeds)
}

// Get retrieves a feed by name
func (h *FeedHandler) Get(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		response.BadRequest(c, "Feed name is required")
		return
	}

	feed, err := h.feedService.GetByName(c.Request.Context(), name)
	if err != nil {
		if err == domain.ErrFeedNotFound {
			response.NotFound(c, "Feed not found")
			return
		}
		h.logger.Error("Failed to get feed", "name", name, "error", err)
		response.InternalServerError(c, "Failed to get feed")
		return
	}

	response.Success(c, feed)
}

// GetArticles retrieves articles for a feed
func (h *FeedHandler) GetArticles(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		response.BadRequest(c, "Feed name is required")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	articles, total, err := h.feedService.GetArticles(c.Request.Context(), name, page, limit)
	if err != nil {
		if err == domain.ErrFeedNotFound {
			response.NotFound(c, "Feed not found")
			return
		}
		h.logger.Error("Failed to get feed articles", "name", name, "error", err)
		response.InternalServerError(c, "Failed to get feed articles")
		return
	}

	response.Paginated(c, articles, page, limit, total)
}

// TriggerSync manually triggers a feed sync
func (h *FeedHandler) TriggerSync(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		response.BadRequest(c, "Feed name is required")
		return
	}

	if err := h.syncService.TriggerSync(c.Request.Context(), name); err != nil {
		if err == domain.ErrFeedNotFound {
			response.NotFound(c, "Feed not found")
			return
		}
		h.logger.Error("Failed to trigger feed sync", "name", name, "error", err)
		response.InternalServerError(c, "Failed to trigger feed sync")
		return
	}

	response.Success(c, gin.H{"message": "Feed sync triggered successfully"})
}
