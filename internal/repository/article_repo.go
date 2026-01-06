package repository

import (
	"context"

	"github.com/amiyamandal-dev/newsp2p/internal/domain"
)

// ArticleRepository defines the interface for article persistence
type ArticleRepository interface {
	// Create creates a new article
	Create(ctx context.Context, article *domain.Article) error

	// GetByID retrieves an article by ID
	GetByID(ctx context.Context, id string) (*domain.Article, error)

	// GetByCID retrieves an article by CID
	GetByCID(ctx context.Context, cid string) (*domain.Article, error)

	// Update updates an existing article
	Update(ctx context.Context, article *domain.Article) error

	// Delete deletes an article by ID
	Delete(ctx context.Context, id string) error

	// List retrieves articles with pagination and filtering
	List(ctx context.Context, filter *domain.ArticleListFilter) ([]*domain.Article, int, error)

	// ListRecent retrieves recent articles for a feed
	ListRecent(ctx context.Context, limit int) ([]*domain.Article, error)

	// ListByAuthor retrieves articles by author with pagination
	ListByAuthor(ctx context.Context, author string, page, limit int) ([]*domain.Article, int, error)
}
