package service

import (
	"context"

	"github.com/amiyamandal-dev/newsp2p/internal/domain"
	"github.com/amiyamandal-dev/newsp2p/internal/repository"
	"github.com/amiyamandal-dev/newsp2p/internal/search"
	"github.com/amiyamandal-dev/newsp2p/pkg/logger"
)

// SearchService handles search-related operations
type SearchService struct {
	index       search.Index
	articleRepo repository.ArticleRepository
	logger      *logger.Logger
}

// NewSearchService creates a new search service
func NewSearchService(
	index search.Index,
	articleRepo repository.ArticleRepository,
	logger *logger.Logger,
) *SearchService {
	return &SearchService{
		index:       index,
		articleRepo: articleRepo,
		logger:      logger.WithComponent("search-service"),
	}
}

// Search performs a full-text search with filtering
func (s *SearchService) Search(ctx context.Context, query *search.SearchQuery) (*search.SearchResult, error) {
	// Set defaults to avoid division by zero
	if query.Page < 1 {
		query.Page = 1
	}
	if query.Limit < 1 {
		query.Limit = 20
	}
	if query.Limit > 100 {
		query.Limit = 100
	}

	// If there's a text query, use the full-text search index
	if query.Query != "" {
		result, err := s.index.Search(ctx, query)
		if err != nil {
			s.logger.Error("Full-text search failed", "error", err)
			return nil, err
		}

		// Bleve returns IDs, fetch full articles from repository
		if len(result.IDs) > 0 {
			articles, err := s.articleRepo.GetByIDs(ctx, result.IDs)
			if err != nil {
				s.logger.Error("Failed to fetch articles by IDs", "error", err)
				return nil, err
			}
			result.Articles = articles
		}

		s.logger.Debug("Full-text search completed",
			"query", query.Query,
			"results", result.Total,
			"articles_fetched", len(result.Articles),
			"page", query.Page,
		)
		return result, nil
	}

	// Fall back to filter-based search when no text query
	filter := &domain.ArticleListFilter{
		Author:   query.Author,
		Category: query.Category,
		Tags:     query.Tags,
		FromDate: query.FromDate,
		ToDate:   query.ToDate,
		Page:     query.Page,
		Limit:    query.Limit,
	}

	articles, total, err := s.articleRepo.List(ctx, filter)
	if err != nil {
		s.logger.Error("Search failed", "error", err)
		return nil, err
	}

	totalPages := 0
	if query.Limit > 0 {
		totalPages = total / query.Limit
		if total%query.Limit > 0 {
			totalPages++
		}
	}

	result := &search.SearchResult{
		Articles:   articles,
		Total:      total,
		Page:       query.Page,
		Limit:      query.Limit,
		TotalPages: totalPages,
		QueryTime:  0,
	}

	s.logger.Debug("Search completed",
		"query", query.Query,
		"results", total,
		"page", query.Page,
	)

	return result, nil
}

// IndexArticle indexes an article for search
func (s *SearchService) IndexArticle(ctx context.Context, article *domain.Article) error {
	return s.index.IndexArticle(ctx, article)
}

// UpdateArticle updates an article in the search index
func (s *SearchService) UpdateArticle(ctx context.Context, article *domain.Article) error {
	return s.index.UpdateArticle(ctx, article)
}

// DeleteArticle removes an article from the search index
func (s *SearchService) DeleteArticle(ctx context.Context, articleID string) error {
	return s.index.DeleteArticle(ctx, articleID)
}

// GetIndexStats returns statistics about the search index
func (s *SearchService) GetIndexStats(ctx context.Context) (map[string]interface{}, error) {
	count, err := s.index.Count()
	if err != nil {
		return nil, err
	}

	stats := map[string]interface{}{
		"total_documents": count,
	}

	return stats, nil
}
