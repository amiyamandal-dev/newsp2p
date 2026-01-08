package handlers

import (
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
	parser := NewQueryParamParser(c)

	q := parser.String("q", "")
	author := parser.String("author", "")
	category := parser.String("category", "")
	tags := parser.Tags("tags")
	pagination := parser.Pagination(20)
	dateRange := parser.DateRange("from", "to")

	if err := parser.Error(); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Build search query
	query := &search.SearchQuery{
		Query:    q,
		Author:   author,
		Category: category,
		Tags:     tags,
		FromDate: dateRange.From,
		ToDate:   dateRange.To,
		Page:     pagination.Page,
		Limit:    pagination.Limit,
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
