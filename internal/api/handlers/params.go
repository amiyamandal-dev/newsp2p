package handlers

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// PaginationParams holds parsed pagination parameters
type PaginationParams struct {
	Page  int
	Limit int
}

// DateRangeParams holds parsed date range parameters
type DateRangeParams struct {
	From time.Time
	To   time.Time
}

// QueryParamParser provides helpers for parsing and validating query parameters
type QueryParamParser struct {
	c   *gin.Context
	err error
}

// NewQueryParamParser creates a new query parameter parser
func NewQueryParamParser(c *gin.Context) *QueryParamParser {
	return &QueryParamParser{c: c}
}

// Error returns any parsing error that occurred
func (p *QueryParamParser) Error() error {
	return p.err
}

// Pagination parses and validates pagination parameters
func (p *QueryParamParser) Pagination(defaultLimit int) PaginationParams {
	if p.err != nil {
		return PaginationParams{Page: 1, Limit: defaultLimit}
	}

	page := 1
	limit := defaultLimit

	if pageStr := p.c.Query("page"); pageStr != "" {
		parsed, err := strconv.Atoi(pageStr)
		if err != nil {
			p.err = fmt.Errorf("invalid 'page' parameter: must be a number")
			return PaginationParams{Page: 1, Limit: defaultLimit}
		}
		page = parsed
	}

	if limitStr := p.c.Query("limit"); limitStr != "" {
		parsed, err := strconv.Atoi(limitStr)
		if err != nil {
			p.err = fmt.Errorf("invalid 'limit' parameter: must be a number")
			return PaginationParams{Page: 1, Limit: defaultLimit}
		}
		limit = parsed
	}

	// Enforce bounds
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = defaultLimit
	}
	if limit > 100 {
		limit = 100
	}

	return PaginationParams{Page: page, Limit: limit}
}

// DateRange parses and validates date range parameters
func (p *QueryParamParser) DateRange(fromKey, toKey string) DateRangeParams {
	if p.err != nil {
		return DateRangeParams{}
	}

	var result DateRangeParams

	if fromStr := p.c.Query(fromKey); fromStr != "" {
		parsed, err := time.Parse(time.RFC3339, fromStr)
		if err != nil {
			p.err = fmt.Errorf("invalid '%s' date format: use RFC3339 format (e.g., 2024-01-15T00:00:00Z)", fromKey)
			return DateRangeParams{}
		}
		result.From = parsed
	}

	if toStr := p.c.Query(toKey); toStr != "" {
		parsed, err := time.Parse(time.RFC3339, toStr)
		if err != nil {
			p.err = fmt.Errorf("invalid '%s' date format: use RFC3339 format (e.g., 2024-01-15T23:59:59Z)", toKey)
			return DateRangeParams{}
		}
		result.To = parsed
	}

	// Validate from <= to
	if !result.From.IsZero() && !result.To.IsZero() && result.From.After(result.To) {
		p.err = fmt.Errorf("'%s' must be before or equal to '%s'", fromKey, toKey)
		return DateRangeParams{}
	}

	return result
}

// Tags parses a comma-separated tags parameter
func (p *QueryParamParser) Tags(key string) []string {
	if p.err != nil {
		return nil
	}

	tagsStr := p.c.Query(key)
	if tagsStr == "" {
		return nil
	}

	// Split and trim whitespace
	rawTags := strings.Split(tagsStr, ",")
	tags := make([]string, 0, len(rawTags))
	for _, tag := range rawTags {
		trimmed := strings.TrimSpace(tag)
		if trimmed != "" {
			tags = append(tags, trimmed)
		}
	}

	return tags
}

// String gets a string parameter with optional default
func (p *QueryParamParser) String(key, defaultValue string) string {
	if p.err != nil {
		return defaultValue
	}

	value := p.c.Query(key)
	if value == "" {
		return defaultValue
	}
	return strings.TrimSpace(value)
}
