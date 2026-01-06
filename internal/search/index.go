package search

import (
	"context"
	"time"

	"github.com/amiyamandal-dev/newsp2p/internal/domain"
)

// SearchDocument represents a document in the search index
type SearchDocument struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Author    string    `json:"author"`
	Tags      []string  `json:"tags"`
	Category  string    `json:"category"`
	Timestamp time.Time `json:"timestamp"`
	CID       string    `json:"cid"`
}

// SearchQuery represents a search query
type SearchQuery struct {
	Query    string
	Author   string
	Category string
	Tags     []string
	FromDate time.Time
	ToDate   time.Time
	Page     int
	Limit    int
}

// SearchResult represents a search result
type SearchResult struct {
	Articles   []*domain.Article
	Total      int
	Page       int
	Limit      int
	TotalPages int
	QueryTime  int64 // milliseconds
}

// Index defines the interface for search indexing
type Index interface {
	// Open opens the search index
	Open(indexPath string) error

	// Close closes the search index
	Close() error

	// IndexArticle indexes an article
	IndexArticle(ctx context.Context, article *domain.Article) error

	// UpdateArticle updates an indexed article
	UpdateArticle(ctx context.Context, article *domain.Article) error

	// DeleteArticle removes an article from the index
	DeleteArticle(ctx context.Context, articleID string) error

	// Search searches the index
	Search(ctx context.Context, query *SearchQuery) (*SearchResult, error)

	// Count returns the number of documents in the index
	Count() (uint64, error)
}

// articleToDocument converts an article to a search document
func ArticleToDocument(article *domain.Article) *SearchDocument {
	return &SearchDocument{
		ID:        article.ID,
		Title:     article.Title,
		Body:      article.Body,
		Author:    article.Author,
		Tags:      article.Tags,
		Category:  article.Category,
		Timestamp: article.Timestamp,
		CID:       article.CID,
	}
}
