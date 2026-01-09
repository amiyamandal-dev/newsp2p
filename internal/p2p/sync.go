package p2p

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"

	"github.com/amiyamandal-dev/newsp2p/internal/domain"
	"github.com/amiyamandal-dev/newsp2p/pkg/logger"
)

const (
	// Protocol IDs for sync
	ProtocolSyncRequest  = "/newsp2p/sync/1.0.0"
	ProtocolSyncResponse = "/newsp2p/sync-response/1.0.0"

	// Sync interval
	DefaultSyncInterval = 30 * time.Second

	// Max articles to request per sync
	MaxArticlesPerSync = 50
)

// SyncRequest represents a request for articles
type SyncRequest struct {
	Since     int64    `json:"since"`      // Unix timestamp - get articles after this time
	Limit     int      `json:"limit"`      // Max articles to return
	ExcludeIDs []string `json:"exclude_ids"` // Article IDs we already have
}

// SyncResponse represents a response with articles
type SyncResponse struct {
	Articles []*domain.Article `json:"articles"`
	HasMore  bool              `json:"has_more"`
}

// ArticleProvider interface for getting articles
type ArticleProvider interface {
	GetRecent(ctx context.Context, limit int, since time.Time) ([]*domain.Article, error)
	GetByID(ctx context.Context, id string) (*domain.Article, error)
	HasArticle(ctx context.Context, id string) bool
}

// ArticleReceiver interface for receiving articles
type ArticleReceiver interface {
	HandleIncomingArticle(article *domain.Article) error
}

// SyncService handles P2P article synchronization
type SyncService struct {
	host     host.Host
	provider ArticleProvider
	receiver ArticleReceiver
	logger   *logger.Logger

	syncInterval time.Duration
	lastSync     time.Time
	mu           sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewSyncService creates a new sync service
func NewSyncService(
	h host.Host,
	provider ArticleProvider,
	receiver ArticleReceiver,
	log *logger.Logger,
) *SyncService {
	ctx, cancel := context.WithCancel(context.Background())

	s := &SyncService{
		host:         h,
		provider:     provider,
		receiver:     receiver,
		logger:       log.WithComponent("p2p-sync"),
		syncInterval: DefaultSyncInterval,
		ctx:          ctx,
		cancel:       cancel,
	}

	// Register protocol handler for incoming sync requests
	h.SetStreamHandler(protocol.ID(ProtocolSyncRequest), s.handleSyncRequest)

	return s
}

// Start starts the background sync process
func (s *SyncService) Start() {
	s.wg.Add(1)
	go s.syncLoop()
	s.logger.Info("P2P sync service started", "interval", s.syncInterval)
}

// Stop stops the sync service
func (s *SyncService) Stop() {
	s.cancel()
	s.wg.Wait()
	s.logger.Info("P2P sync service stopped")
}

// SetSyncInterval sets the sync interval
func (s *SyncService) SetSyncInterval(d time.Duration) {
	s.mu.Lock()
	s.syncInterval = d
	s.mu.Unlock()
}

// syncLoop runs the periodic sync
func (s *SyncService) syncLoop() {
	defer s.wg.Done()

	// Initial sync after a short delay
	time.Sleep(5 * time.Second)
	s.syncWithPeers()

	ticker := time.NewTicker(s.syncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.syncWithPeers()
		}
	}
}

// syncWithPeers syncs articles with all connected peers
func (s *SyncService) syncWithPeers() {
	peers := s.host.Network().Peers()
	if len(peers) == 0 {
		s.logger.Debug("No peers to sync with")
		return
	}

	s.logger.Info("Starting article sync", "peer_count", len(peers))

	var wg sync.WaitGroup
	for _, peerID := range peers {
		if peerID == s.host.ID() {
			continue
		}

		wg.Add(1)
		go func(pid peer.ID) {
			defer wg.Done()
			if err := s.syncWithPeer(pid); err != nil {
				s.logger.Debug("Failed to sync with peer", "peer", pid.String()[:16], "error", err)
			}
		}(peerID)
	}

	wg.Wait()

	s.mu.Lock()
	s.lastSync = time.Now()
	s.mu.Unlock()

	s.logger.Debug("Article sync completed")
}

// syncWithPeer syncs articles with a specific peer
func (s *SyncService) syncWithPeer(peerID peer.ID) error {
	ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
	defer cancel()

	// Open stream to peer
	stream, err := s.host.NewStream(ctx, peerID, protocol.ID(ProtocolSyncRequest))
	if err != nil {
		return fmt.Errorf("failed to open stream: %w", err)
	}
	defer stream.Close()

	// Send sync request
	s.mu.RLock()
	since := s.lastSync
	s.mu.RUnlock()

	// If never synced, get articles from last 24 hours
	if since.IsZero() {
		since = time.Now().Add(-24 * time.Hour)
	}

	req := &SyncRequest{
		Since: since.Unix(),
		Limit: MaxArticlesPerSync,
	}

	encoder := json.NewEncoder(stream)
	if err := encoder.Encode(req); err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	// Read response
	decoder := json.NewDecoder(stream)
	var resp SyncResponse
	if err := decoder.Decode(&resp); err != nil {
		if err == io.EOF {
			return nil
		}
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Process received articles
	newCount := 0
	for _, article := range resp.Articles {
		if article == nil {
			continue
		}

		// Check if we already have this article
		if s.provider.HasArticle(ctx, article.ID) {
			continue
		}

		// Handle the incoming article
		if err := s.receiver.HandleIncomingArticle(article); err != nil {
			s.logger.Warn("Failed to handle synced article", "article_id", article.ID, "error", err)
			continue
		}
		newCount++
	}

	if newCount > 0 {
		s.logger.Info("Synced articles from peer",
			"peer", peerID.String()[:16],
			"received", len(resp.Articles),
			"new", newCount,
		)
	}

	return nil
}

// handleSyncRequest handles incoming sync requests from peers
func (s *SyncService) handleSyncRequest(stream network.Stream) {
	defer stream.Close()

	peerID := stream.Conn().RemotePeer()
	s.logger.Debug("Received sync request", "from", peerID.String()[:16])

	// Read request
	decoder := json.NewDecoder(bufio.NewReader(stream))
	var req SyncRequest
	if err := decoder.Decode(&req); err != nil {
		s.logger.Warn("Failed to decode sync request", "error", err)
		return
	}

	// Get articles since the requested time
	since := time.Unix(req.Since, 0)
	limit := req.Limit
	if limit <= 0 || limit > MaxArticlesPerSync {
		limit = MaxArticlesPerSync
	}

	ctx, cancel := context.WithTimeout(s.ctx, 10*time.Second)
	defer cancel()

	articles, err := s.provider.GetRecent(ctx, limit, since)
	if err != nil {
		s.logger.Warn("Failed to get articles for sync", "error", err)
		return
	}

	// Send response
	resp := &SyncResponse{
		Articles: articles,
		HasMore:  len(articles) >= limit,
	}

	encoder := json.NewEncoder(stream)
	if err := encoder.Encode(resp); err != nil {
		s.logger.Warn("Failed to send sync response", "error", err)
		return
	}

	s.logger.Debug("Sent sync response", "to", peerID.String()[:16], "articles", len(articles))
}

// TriggerSync manually triggers a sync
func (s *SyncService) TriggerSync() {
	go s.syncWithPeers()
}

// GetLastSyncTime returns the last sync time
func (s *SyncService) GetLastSyncTime() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastSync
}
