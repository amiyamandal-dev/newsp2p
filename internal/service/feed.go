package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/amiyamandal-dev/newsp2p/internal/domain"
	"github.com/amiyamandal-dev/newsp2p/internal/ipfs"
	"github.com/amiyamandal-dev/newsp2p/internal/repository"
	"github.com/amiyamandal-dev/newsp2p/pkg/logger"
)

// FeedService handles feed-related business logic
type FeedService struct {
	feedRepo    repository.FeedRepository
	articleRepo repository.ArticleRepository
	ipnsManager *ipfs.IPNSManager
	logger      *logger.Logger
}

// NewFeedService creates a new feed service
func NewFeedService(
	feedRepo repository.FeedRepository,
	articleRepo repository.ArticleRepository,
	ipnsManager *ipfs.IPNSManager,
	logger *logger.Logger,
) *FeedService {
	return &FeedService{
		feedRepo:    feedRepo,
		articleRepo: articleRepo,
		ipnsManager: ipnsManager,
		logger:      logger.WithComponent("feed-service"),
	}
}

// Create creates a new feed
func (s *FeedService) Create(ctx context.Context, req *domain.FeedCreateRequest) (*domain.Feed, error) {
	// Check if feed name already exists
	_, err := s.feedRepo.GetByName(ctx, req.Name)
	if err == nil {
		return nil, domain.ErrFeedAlreadyExists
	}
	if err != domain.ErrFeedNotFound {
		return nil, err
	}

	// Generate or ensure IPNS key exists
	keyInfo, err := s.ipnsManager.EnsureKey(ctx, req.Name)
	if err != nil {
		s.logger.Error("Failed to ensure IPNS key", "feed_name", req.Name, "error", err)
		return nil, fmt.Errorf("failed to create IPNS key: %w", err)
	}

	// Create feed
	feed := &domain.Feed{
		ID:           uuid.New().String(),
		Name:         req.Name,
		IPNSKey:      keyInfo.Name,
		IPNSAddress:  fmt.Sprintf("/ipns/%s", keyInfo.ID),
		SyncInterval: req.SyncInterval,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := feed.Validate(); err != nil {
		return nil, err
	}

	if err := s.feedRepo.Create(ctx, feed); err != nil {
		s.logger.Error("Failed to create feed", "feed_name", req.Name, "error", err)
		return nil, fmt.Errorf("failed to create feed: %w", err)
	}

	s.logger.Info("Feed created successfully", "feed_id", feed.ID, "feed_name", feed.Name)

	return feed, nil
}

// GetByName retrieves a feed by name
func (s *FeedService) GetByName(ctx context.Context, name string) (*domain.Feed, error) {
	return s.feedRepo.GetByName(ctx, name)
}

// List retrieves all feeds
func (s *FeedService) List(ctx context.Context) ([]*domain.Feed, error) {
	return s.feedRepo.List(ctx)
}

// Update updates a feed
func (s *FeedService) Update(ctx context.Context, name string, req *domain.FeedUpdateRequest) (*domain.Feed, error) {
	feed, err := s.feedRepo.GetByName(ctx, name)
	if err != nil {
		return nil, err
	}

	if req.SyncInterval > 0 {
		feed.SyncInterval = req.SyncInterval
	}

	if err := feed.Validate(); err != nil {
		return nil, err
	}

	if err := s.feedRepo.Update(ctx, feed); err != nil {
		s.logger.Error("Failed to update feed", "feed_name", name, "error", err)
		return nil, fmt.Errorf("failed to update feed: %w", err)
	}

	s.logger.Info("Feed updated successfully", "feed_name", name)

	return feed, nil
}

// Delete deletes a feed
func (s *FeedService) Delete(ctx context.Context, name string) error {
	feed, err := s.feedRepo.GetByName(ctx, name)
	if err != nil {
		return err
	}

	if err := s.feedRepo.Delete(ctx, feed.ID); err != nil {
		s.logger.Error("Failed to delete feed", "feed_name", name, "error", err)
		return fmt.Errorf("failed to delete feed: %w", err)
	}

	s.logger.Info("Feed deleted successfully", "feed_name", name)

	return nil
}

// GetArticles retrieves articles for a feed
func (s *FeedService) GetArticles(ctx context.Context, name string, page, limit int) ([]*domain.Article, int, error) {
	// Get feed
	_, err := s.feedRepo.GetByName(ctx, name)
	if err != nil {
		return nil, 0, err
	}

	// For now, just return recent articles
	// In a full implementation, we'd use the feed_articles junction table
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}

	filter := &domain.ArticleListFilter{
		Page:  page,
		Limit: limit,
	}

	articles, total, err := s.articleRepo.List(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	return articles, total, nil
}
