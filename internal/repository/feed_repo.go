package repository

import (
	"context"

	"github.com/amiyamandal-dev/newsp2p/internal/domain"
)

// FeedRepository defines the interface for feed persistence
type FeedRepository interface {
	// Create creates a new feed
	Create(ctx context.Context, feed *domain.Feed) error

	// GetByID retrieves a feed by ID
	GetByID(ctx context.Context, id string) (*domain.Feed, error)

	// GetByName retrieves a feed by name
	GetByName(ctx context.Context, name string) (*domain.Feed, error)

	// Update updates an existing feed
	Update(ctx context.Context, feed *domain.Feed) error

	// Delete deletes a feed by ID
	Delete(ctx context.Context, id string) error

	// List retrieves all feeds
	List(ctx context.Context) ([]*domain.Feed, error)

	// ListDueForSync retrieves feeds that are due for syncing
	ListDueForSync(ctx context.Context) ([]*domain.Feed, error)
}
