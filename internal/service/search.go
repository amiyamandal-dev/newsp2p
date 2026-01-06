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
	// Perform search using the index
	// Note: For a full implementation, we'd need to modify the Bleve search
	// to return article IDs, then fetch full articles from the repository
	// For now, we'll use the repository's List method with filters

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

	totalPages := total / query.Limit
	if total%query.Limit > 0 {
		totalPages++
	}

	result := &search.SearchResult{
		Articles:   articles,
		Total:      total,
		Page:       query.Page,
		Limit:      query.Limit,
		TotalPages: totalPages,
		QueryTime:  0, // TODO: measure query time
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
