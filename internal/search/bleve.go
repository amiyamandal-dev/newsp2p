package search

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/search/query"

	"github.com/amiyamandal-dev/newsp2p/internal/domain"
	"github.com/amiyamandal-dev/newsp2p/pkg/logger"
)

// BleveIndex implements the Index interface using Bleve
type BleveIndex struct {
	index  bleve.Index
	mu     sync.RWMutex // Protects concurrent access to the index
	logger *logger.Logger
}

// NewBleveIndex creates a new Bleve search index
func NewBleveIndex(logger *logger.Logger) *BleveIndex {
	return &BleveIndex{
		logger: logger.WithComponent("bleve-index"),
	}
}

// Open opens or creates the search index
func (b *BleveIndex) Open(indexPath string) error {
	// Ensure directory exists
	indexDir := filepath.Dir(indexPath)
	if err := os.MkdirAll(indexDir, 0755); err != nil {
		return fmt.Errorf("failed to create index directory: %w", err)
	}

	var err error

	// Try to open existing index
	b.index, err = bleve.Open(indexPath)
	if err == nil {
		b.logger.Info("Opened existing search index", "path", indexPath)
		return nil
	}

	// Index doesn't exist, create new one
	indexMapping := b.buildIndexMapping()
	b.index, err = bleve.New(indexPath, indexMapping)
	if err != nil {
		return fmt.Errorf("failed to create search index: %w", err)
	}

	b.logger.Info("Created new search index", "path", indexPath)
	return nil
}

// buildIndexMapping builds the index mapping for articles
func (b *BleveIndex) buildIndexMapping() mapping.IndexMapping {
	// Create a document mapping
	articleMapping := bleve.NewDocumentMapping()

	// Title field - analyzed, boosted
	titleFieldMapping := bleve.NewTextFieldMapping()
	titleFieldMapping.Analyzer = "en"
	titleFieldMapping.Store = true
	titleFieldMapping.Index = true
	articleMapping.AddFieldMappingsAt("title", titleFieldMapping)

	// Body field - analyzed
	bodyFieldMapping := bleve.NewTextFieldMapping()
	bodyFieldMapping.Analyzer = "en"
	bodyFieldMapping.Store = false
	bodyFieldMapping.Index = true
	articleMapping.AddFieldMappingsAt("body", bodyFieldMapping)

	// Author field - keyword (not analyzed)
	authorFieldMapping := bleve.NewKeywordFieldMapping()
	authorFieldMapping.Store = true
	authorFieldMapping.Index = true
	articleMapping.AddFieldMappingsAt("author", authorFieldMapping)

	// Category field - keyword
	categoryFieldMapping := bleve.NewKeywordFieldMapping()
	categoryFieldMapping.Store = true
	categoryFieldMapping.Index = true
	articleMapping.AddFieldMappingsAt("category", categoryFieldMapping)

	// Tags field - text analyzed
	tagsFieldMapping := bleve.NewTextFieldMapping()
	tagsFieldMapping.Analyzer = "en"
	tagsFieldMapping.Store = true
	tagsFieldMapping.Index = true
	articleMapping.AddFieldMappingsAt("tags", tagsFieldMapping)

	// Timestamp field - datetime
	timestampFieldMapping := bleve.NewDateTimeFieldMapping()
	timestampFieldMapping.Store = true
	timestampFieldMapping.Index = true
	articleMapping.AddFieldMappingsAt("timestamp", timestampFieldMapping)

	// Create index mapping
	indexMapping := bleve.NewIndexMapping()
	indexMapping.AddDocumentMapping("article", articleMapping)
	indexMapping.DefaultMapping = articleMapping

	return indexMapping
}

// Close closes the search index
func (b *BleveIndex) Close() error {
	if b.index != nil {
		if err := b.index.Close(); err != nil {
			return fmt.Errorf("failed to close index: %w", err)
		}
		b.logger.Info("Closed search index")
	}
	return nil
}

// IndexArticle indexes an article
func (b *BleveIndex) IndexArticle(ctx context.Context, article *domain.Article) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	doc := ArticleToDocument(article)

	if err := b.index.Index(article.ID, doc); err != nil {
		b.logger.Error("Failed to index article", "article_id", article.ID, "error", err)
		return fmt.Errorf("failed to index article: %w", err)
	}

	b.logger.Debug("Indexed article", "article_id", article.ID)
	return nil
}

// UpdateArticle updates an indexed article
func (b *BleveIndex) UpdateArticle(ctx context.Context, article *domain.Article) error {
	// In Bleve, update is the same as index (it overwrites)
	return b.IndexArticle(ctx, article)
}

// DeleteArticle removes an article from the index
func (b *BleveIndex) DeleteArticle(ctx context.Context, articleID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if err := b.index.Delete(articleID); err != nil {
		b.logger.Error("Failed to delete article from index", "article_id", articleID, "error", err)
		return fmt.Errorf("failed to delete from index: %w", err)
	}

	b.logger.Debug("Deleted article from index", "article_id", articleID)
	return nil
}

// Search searches the index
func (b *BleveIndex) Search(ctx context.Context, query *SearchQuery) (*SearchResult, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	startTime := time.Now()

	// Build search query
	searchQuery := b.buildSearchQuery(query)

	// Build search request
	searchRequest := bleve.NewSearchRequest(searchQuery)

	// Pagination
	if query.Page < 1 {
		query.Page = 1
	}
	if query.Limit < 1 {
		query.Limit = 20
	}
	if query.Limit > 100 {
		query.Limit = 100
	}

	searchRequest.From = (query.Page - 1) * query.Limit
	searchRequest.Size = query.Limit

	// Execute search
	searchResults, err := b.index.Search(searchRequest)
	if err != nil {
		b.logger.Error("Search failed", "error", err)
		return nil, fmt.Errorf("search failed: %w", err)
	}

	queryTime := time.Since(startTime).Milliseconds()

	// Extract document IDs from search results
	ids := make([]string, 0, len(searchResults.Hits))
	for _, hit := range searchResults.Hits {
		ids = append(ids, hit.ID)
	}

	totalPages := int(searchResults.Total) / query.Limit
	if int(searchResults.Total)%query.Limit > 0 {
		totalPages++
	}

	result := &SearchResult{
		Articles:   make([]*domain.Article, 0),
		IDs:        ids, // Populate IDs for the search service to fetch full articles
		Total:      int(searchResults.Total),
		Page:       query.Page,
		Limit:      query.Limit,
		TotalPages: totalPages,
		QueryTime:  queryTime,
	}

	b.logger.Debug("Search completed",
		"query", query.Query,
		"results", searchResults.Total,
		"ids_found", len(ids),
		"time_ms", queryTime,
	)

	return result, nil
}

// buildSearchQuery builds a Bleve query from search parameters
func (b *BleveIndex) buildSearchQuery(searchQuery *SearchQuery) query.Query {
	var queries []query.Query

	// Full-text query on title and body
	if searchQuery.Query != "" {
		matchQuery := bleve.NewMatchQuery(searchQuery.Query)
		queries = append(queries, matchQuery)
	}

	// Author filter
	if searchQuery.Author != "" {
		authorQuery := bleve.NewMatchQuery(searchQuery.Author)
		authorQuery.SetField("author")
		queries = append(queries, authorQuery)
	}

	// Category filter
	if searchQuery.Category != "" {
		categoryQuery := bleve.NewMatchQuery(searchQuery.Category)
		categoryQuery.SetField("category")
		queries = append(queries, categoryQuery)
	}

	// Tags filter
	for _, tag := range searchQuery.Tags {
		tagQuery := bleve.NewMatchQuery(tag)
		tagQuery.SetField("tags")
		queries = append(queries, tagQuery)
	}

	// Date range filter
	if !searchQuery.FromDate.IsZero() || !searchQuery.ToDate.IsZero() {
		dateQuery := bleve.NewDateRangeQuery(searchQuery.FromDate, searchQuery.ToDate)
		dateQuery.SetField("timestamp")
		queries = append(queries, dateQuery)
	}

	// Combine queries
	if len(queries) == 0 {
		// No filters, match all
		return bleve.NewMatchAllQuery()
	} else if len(queries) == 1 {
		return queries[0]
	} else {
		// Multiple queries, combine with AND
		conjunctionQuery := bleve.NewConjunctionQuery(queries...)
		return conjunctionQuery
	}
}

// Count returns the number of documents in the index
func (b *BleveIndex) Count() (uint64, error) {
	count, err := b.index.DocCount()
	if err != nil {
		return 0, fmt.Errorf("failed to get doc count: %w", err)
	}
	return count, nil
}

// GetDocumentIDs returns document IDs from search results
func GetDocumentIDs(searchResults *bleve.SearchResult) []string {
	ids := make([]string, 0, len(searchResults.Hits))
	for _, hit := range searchResults.Hits {
		ids = append(ids, hit.ID)
	}
	return ids
}
