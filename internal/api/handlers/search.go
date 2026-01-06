package handlers

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/amiyamandal-dev/newsp2p/internal/search"
	"github.com/amiyamandal-dev/newsp2p/internal/service"
	"github.com/amiyamandal-dev/newsp2p/pkg/logger"
	"github.com/amiyamandal-dev/newsp2p/pkg/response"
)

// SearchHandler handles search-related requests
type SearchHandler struct {
	searchService *service.SearchService
	logger        *logger.Logger
}

// NewSearchHandler creates a new search handler
func NewSearchHandler(searchService *service.SearchService, logger *logger.Logger) *SearchHandler {
	return &SearchHandler{
		searchService: searchService,
		logger:        logger.WithComponent("search-handler"),
	}
}

// Search performs a search query
func (h *SearchHandler) Search(c *gin.Context) {
	// Parse query parameters
	q := c.Query("q")
	author := c.Query("author")
	category := c.Query("category")
	tagsStr := c.Query("tags")
	fromDateStr := c.Query("from")
	toDateStr := c.Query("to")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	// Parse tags
	var tags []string
	if tagsStr != "" {
		tags = strings.Split(tagsStr, ",")
	}

	// Parse dates
	var fromDate, toDate time.Time
	if fromDateStr != "" {
		fromDate, _ = time.Parse(time.RFC3339, fromDateStr)
	}
	if toDateStr != "" {
		toDate, _ = time.Parse(time.RFC3339, toDateStr)
	}

	// Build search query
	query := &search.SearchQuery{
		Query:    q,
		Author:   author,
		Category: category,
		Tags:     tags,
		FromDate: fromDate,
		ToDate:   toDate,
		Page:     page,
		Limit:    limit,
	}

	// Perform search
	result, err := h.searchService.Search(c.Request.Context(), query)
	if err != nil {
		h.logger.Error("Search failed", "query", q, "error", err)
		response.InternalServerError(c, "Search failed")
		return
	}

	c.JSON(200, gin.H{
		"success": true,
		"data": gin.H{
			"results": result.Articles,
			"pagination": gin.H{
				"page":        result.Page,
				"limit":       result.Limit,
				"total":       result.Total,
				"total_pages": result.TotalPages,
			},
			"query_time_ms": result.QueryTime,
		},
	})
}
