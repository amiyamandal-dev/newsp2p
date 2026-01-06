package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/amiyamandal-dev/newsp2p/internal/domain"
	"github.com/amiyamandal-dev/newsp2p/internal/ipfs"
	"github.com/amiyamandal-dev/newsp2p/internal/repository"
	"github.com/amiyamandal-dev/newsp2p/pkg/logger"
)

// SyncService handles background feed synchronization to IPNS
type SyncService struct {
	feedRepo    repository.FeedRepository
	articleRepo repository.ArticleRepository
	ipfsClient  *ipfs.Client
	ipnsManager *ipfs.IPNSManager
	logger      *logger.Logger
	stopChan    chan struct{}
}

// NewSyncService creates a new sync service
func NewSyncService(
	feedRepo repository.FeedRepository,
	articleRepo repository.ArticleRepository,
	ipfsClient *ipfs.Client,
	ipnsManager *ipfs.IPNSManager,
	logger *logger.Logger,
) *SyncService {
	return &SyncService{
		feedRepo:    feedRepo,
		articleRepo: articleRepo,
		ipfsClient:  ipfsClient,
		ipnsManager: ipnsManager,
		logger:      logger.WithComponent("sync-service"),
		stopChan:    make(chan struct{}),
	}
}

// Start starts the background sync service
func (s *SyncService) Start(ctx context.Context, intervalMinutes int) {
	s.logger.Info("Starting feed sync service", "interval_minutes", intervalMinutes)

	ticker := time.NewTicker(time.Duration(intervalMinutes) * time.Minute)
	defer ticker.Stop()

	// Run initial sync
	s.syncAllFeeds(ctx)

	for {
		select {
		case <-ticker.C:
			s.syncAllFeeds(ctx)
		case <-s.stopChan:
			s.logger.Info("Stopping feed sync service")
			return
		case <-ctx.Done():
			s.logger.Info("Context cancelled, stopping sync service")
			return
		}
	}
}

// Stop stops the background sync service
func (s *SyncService) Stop() {
	close(s.stopChan)
}

// syncAllFeeds syncs all feeds that are due for syncing
func (s *SyncService) syncAllFeeds(ctx context.Context) {
	feeds, err := s.feedRepo.ListDueForSync(ctx)
	if err != nil {
		s.logger.Error("Failed to list feeds due for sync", "error", err)
		return
	}

	s.logger.Debug("Found feeds to sync", "count", len(feeds))

	for _, feed := range feeds {
		if err := s.syncFeed(ctx, feed); err != nil {
			s.logger.Error("Failed to sync feed",
				"feed_name", feed.Name,
				"error", err,
			)
		}
	}
}

// syncFeed syncs a single feed to IPNS
func (s *SyncService) syncFeed(ctx context.Context, feed *domain.Feed) error {
	s.logger.Info("Syncing feed", "feed_name", feed.Name)

	// Get recent articles
	articles, err := s.articleRepo.ListRecent(ctx, 100)
	if err != nil {
		return fmt.Errorf("failed to get recent articles: %w", err)
	}

	// Extract CIDs
	cids := make([]string, 0, len(articles))
	for _, article := range articles {
		cids = append(cids, article.CID)
	}

	// Create feed manifest
	manifest := &domain.FeedManifest{
		Version:     "1.0",
		Articles:    cids,
		LastUpdated: time.Now(),
		TotalCount:  len(cids),
	}

	// TODO: Sign manifest
	// For now, leave signature empty

	// Serialize manifest to JSON
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	// Upload manifest to IPFS
	manifestCID, err := s.ipfsClient.Add(ctx, manifestJSON)
	if err != nil {
		return fmt.Errorf("failed to upload manifest to IPFS: %w", err)
	}

	s.logger.Debug("Uploaded manifest to IPFS",
		"feed_name", feed.Name,
		"manifest_cid", manifestCID,
		"article_count", len(cids),
	)

	// Publish to IPNS
	ipnsPath, err := s.ipnsManager.Publish(ctx, manifestCID, feed.IPNSKey)
	if err != nil {
		return fmt.Errorf("failed to publish to IPNS: %w", err)
	}

	s.logger.Info("Published feed to IPNS",
		"feed_name", feed.Name,
		"manifest_cid", manifestCID,
		"ipns_path", ipnsPath,
	)

	// Unpin old manifest if exists
	if feed.LastCID != "" && feed.LastCID != manifestCID {
		if err := s.ipfsClient.Unpin(ctx, feed.LastCID); err != nil {
			s.logger.Warn("Failed to unpin old manifest",
				"feed_name", feed.Name,
				"old_cid", feed.LastCID,
				"error", err,
			)
		}
	}

	// Update feed record
	feed.LastCID = manifestCID
	feed.LastSync = time.Now()
	feed.IPNSAddress = ipnsPath

	if err := s.feedRepo.Update(ctx, feed); err != nil {
		s.logger.Error("Failed to update feed record",
			"feed_name", feed.Name,
			"error", err,
		)
		return fmt.Errorf("failed to update feed: %w", err)
	}

	s.logger.Info("Feed sync completed successfully", "feed_name", feed.Name)

	return nil
}

// TriggerSync manually triggers a sync for a specific feed
func (s *SyncService) TriggerSync(ctx context.Context, feedName string) error {
	feed, err := s.feedRepo.GetByName(ctx, feedName)
	if err != nil {
		return err
	}

	return s.syncFeed(ctx, feed)
}
