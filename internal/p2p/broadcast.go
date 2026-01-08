package p2p

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	pubsub "github.com/libp2p/go-libp2p-pubsub"

	"github.com/amiyamandal-dev/newsp2p/internal/domain"
	"github.com/amiyamandal-dev/newsp2p/pkg/logger"
)

const (
	TopicArticles  = "newsp2p/articles/v1"
	TopicFeeds     = "newsp2p/feeds/v1"
	TopicVotes     = "newsp2p/votes/v1"
	TopicModerator = "newsp2p/moderation/v1"
)

// Ensure pubsub is imported
var _ = pubsub.WithMessageSigning

// ArticleMessage represents a message broadcast about an article
type ArticleMessage struct {
	Type      string          `json:"type"` // "new", "update", "delete"
	Article   *domain.Article `json:"article,omitempty"`
	ArticleID string          `json:"article_id,omitempty"`
	Timestamp int64           `json:"timestamp"`
	Signature string          `json:"signature"`
	PeerID    string          `json:"peer_id"`
}

// FeedMessage represents a feed update message
type FeedMessage struct {
	Type      string        `json:"type"` // "new", "update"
	Feed      *domain.Feed  `json:"feed"`
	Timestamp int64         `json:"timestamp"`
	Signature string        `json:"signature"`
	PeerID    string        `json:"peer_id"`
}

// VoteMessage represents a content vote/rating
type VoteMessage struct {
	ArticleID string `json:"article_id"`
	VoterDID  string `json:"voter_did"`
	Vote      int    `json:"vote"` // +1 or -1
	Reason    string `json:"reason,omitempty"`
	Timestamp int64  `json:"timestamp"`
	Signature string `json:"signature"`
}

// ModerationMessage represents a moderation action
type ModerationMessage struct {
	ArticleID   string `json:"article_id"`
	Action      string `json:"action"` // "report", "flag", "vote_remove"
	Reason      string `json:"reason"`
	ReporterDID string `json:"reporter_did"`
	Timestamp   int64  `json:"timestamp"`
	Signature   string `json:"signature"`
}

// Broadcaster handles P2P content broadcasting
type Broadcaster struct {
	node   *P2PNode
	logger *logger.Logger

	articleHandlers     []ArticleHandler
	feedHandlers        []FeedHandler
	voteHandlers        []VoteHandler
	moderationHandlers  []ModerationHandler
	mu                  sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// ArticleHandler handles incoming article messages
type ArticleHandler func(*ArticleMessage) error

// FeedHandler handles incoming feed messages
type FeedHandler func(*FeedMessage) error

// VoteHandler handles incoming vote messages
type VoteHandler func(*VoteMessage) error

// ModerationHandler handles incoming moderation messages
type ModerationHandler func(*ModerationMessage) error

// NewBroadcaster creates a new broadcaster
func NewBroadcaster(node *P2PNode, log *logger.Logger) *Broadcaster {
	ctx, cancel := context.WithCancel(context.Background())

	return &Broadcaster{
		node:                node,
		logger:              log.WithComponent("broadcaster"),
		articleHandlers:     make([]ArticleHandler, 0),
		feedHandlers:        make([]FeedHandler, 0),
		voteHandlers:        make([]VoteHandler, 0),
		moderationHandlers:  make([]ModerationHandler, 0),
		ctx:                 ctx,
		cancel:              cancel,
	}
}

// Start starts the broadcaster
func (b *Broadcaster) Start() error {
	// Join topics
	topics := []string{TopicArticles, TopicFeeds, TopicVotes, TopicModerator}
	for _, topic := range topics {
		if _, err := b.node.JoinTopic(topic); err != nil {
			return fmt.Errorf("failed to join topic %s: %w", topic, err)
		}
	}

	// Start subscribers
	b.wg.Add(4)
	go b.subscribeArticles()
	go b.subscribeFeeds()
	go b.subscribeVotes()
	go b.subscribeModeration()

	b.logger.Info("Broadcaster started")
	return nil
}

// Stop stops the broadcaster
func (b *Broadcaster) Stop() {
	b.cancel()
	b.wg.Wait()
	b.logger.Info("Broadcaster stopped")
}

// BroadcastArticle broadcasts an article to the network
func (b *Broadcaster) BroadcastArticle(msgType string, article *domain.Article) error {
	msg := &ArticleMessage{
		Type:      msgType,
		Article:   article,
		Timestamp: article.Timestamp.Unix(),
		PeerID:    b.node.GetPeerID().String(),
	}

	if article != nil {
		msg.ArticleID = article.ID
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal article message: %w", err)
	}

	if err := b.node.Publish(TopicArticles, data); err != nil {
		return fmt.Errorf("failed to broadcast article: %w", err)
	}

	b.logger.Info("Broadcast article", "type", msgType, "article_id", article.ID)
	return nil
}

// BroadcastFeed broadcasts a feed update
func (b *Broadcaster) BroadcastFeed(msgType string, feed *domain.Feed) error {
	msg := &FeedMessage{
		Type:      msgType,
		Feed:      feed,
		Timestamp: feed.UpdatedAt.Unix(),
		PeerID:    b.node.GetPeerID().String(),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal feed message: %w", err)
	}

	if err := b.node.Publish(TopicFeeds, data); err != nil {
		return fmt.Errorf("failed to broadcast feed: %w", err)
	}

	b.logger.Info("Broadcast feed", "type", msgType, "feed_name", feed.Name)
	return nil
}

// BroadcastVote broadcasts a vote
func (b *Broadcaster) BroadcastVote(vote *VoteMessage) error {
	data, err := json.Marshal(vote)
	if err != nil {
		return fmt.Errorf("failed to marshal vote: %w", err)
	}

	if err := b.node.Publish(TopicVotes, data); err != nil {
		return fmt.Errorf("failed to broadcast vote: %w", err)
	}

	b.logger.Debug("Broadcast vote", "article_id", vote.ArticleID, "vote", vote.Vote)
	return nil
}

// BroadcastModeration broadcasts a moderation action
func (b *Broadcaster) BroadcastModeration(msg *ModerationMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal moderation message: %w", err)
	}

	if err := b.node.Publish(TopicModerator, data); err != nil {
		return fmt.Errorf("failed to broadcast moderation: %w", err)
	}

	b.logger.Info("Broadcast moderation", "action", msg.Action, "article_id", msg.ArticleID)
	return nil
}

// OnArticle registers an article handler
func (b *Broadcaster) OnArticle(handler ArticleHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.articleHandlers = append(b.articleHandlers, handler)
}

// OnFeed registers a feed handler
func (b *Broadcaster) OnFeed(handler FeedHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.feedHandlers = append(b.feedHandlers, handler)
}

// OnVote registers a vote handler
func (b *Broadcaster) OnVote(handler VoteHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.voteHandlers = append(b.voteHandlers, handler)
}

// OnModeration registers a moderation handler
func (b *Broadcaster) OnModeration(handler ModerationHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.moderationHandlers = append(b.moderationHandlers, handler)
}

// subscribeArticles subscribes to article messages
func (b *Broadcaster) subscribeArticles() {
	defer b.wg.Done()

	sub, err := b.node.Subscribe(TopicArticles)
	if err != nil {
		b.logger.Error("Failed to subscribe to articles", "error", err)
		return
	}

	b.logger.Info("Subscribed to articles topic")

	for {
		msg, err := sub.Next(b.ctx)
		if err != nil {
			if b.ctx.Err() != nil {
				return
			}
			b.logger.Warn("Error reading article message", "error", err)
			continue
		}

		// Skip messages from self
		if msg.ReceivedFrom == b.node.GetPeerID() {
			continue
		}

		var articleMsg ArticleMessage
		if err := json.Unmarshal(msg.Data, &articleMsg); err != nil {
			b.logger.Warn("Failed to unmarshal article message", "error", err)
			continue
		}

		b.handleArticleMessage(&articleMsg)
	}
}

// handleArticleMessage handles an article message
func (b *Broadcaster) handleArticleMessage(msg *ArticleMessage) {
	b.mu.RLock()
	handlers := make([]ArticleHandler, len(b.articleHandlers))
	copy(handlers, b.articleHandlers)
	b.mu.RUnlock()

	for _, handler := range handlers {
		if err := handler(msg); err != nil {
			b.logger.Warn("Article handler error", "error", err)
		}
	}
}

// subscribeFeeds subscribes to feed messages
func (b *Broadcaster) subscribeFeeds() {
	defer b.wg.Done()

	sub, err := b.node.Subscribe(TopicFeeds)
	if err != nil {
		b.logger.Error("Failed to subscribe to feeds", "error", err)
		return
	}

	b.logger.Info("Subscribed to feeds topic")

	for {
		msg, err := sub.Next(b.ctx)
		if err != nil {
			if b.ctx.Err() != nil {
				return
			}
			b.logger.Warn("Error reading feed message", "error", err)
			continue
		}

		if msg.ReceivedFrom == b.node.GetPeerID() {
			continue
		}

		var feedMsg FeedMessage
		if err := json.Unmarshal(msg.Data, &feedMsg); err != nil {
			b.logger.Warn("Failed to unmarshal feed message", "error", err)
			continue
		}

		b.handleFeedMessage(&feedMsg)
	}
}

// handleFeedMessage handles a feed message
func (b *Broadcaster) handleFeedMessage(msg *FeedMessage) {
	b.mu.RLock()
	handlers := make([]FeedHandler, len(b.feedHandlers))
	copy(handlers, b.feedHandlers)
	b.mu.RUnlock()

	for _, handler := range handlers {
		if err := handler(msg); err != nil {
			b.logger.Warn("Feed handler error", "error", err)
		}
	}
}

// subscribeVotes subscribes to vote messages
func (b *Broadcaster) subscribeVotes() {
	defer b.wg.Done()

	sub, err := b.node.Subscribe(TopicVotes)
	if err != nil {
		b.logger.Error("Failed to subscribe to votes", "error", err)
		return
	}

	b.logger.Info("Subscribed to votes topic")

	for {
		msg, err := sub.Next(b.ctx)
		if err != nil {
			if b.ctx.Err() != nil {
				return
			}
			b.logger.Warn("Error reading vote message", "error", err)
			continue
		}

		if msg.ReceivedFrom == b.node.GetPeerID() {
			continue
		}

		var voteMsg VoteMessage
		if err := json.Unmarshal(msg.Data, &voteMsg); err != nil {
			b.logger.Warn("Failed to unmarshal vote message", "error", err)
			continue
		}

		b.handleVoteMessage(&voteMsg)
	}
}

// handleVoteMessage handles a vote message
func (b *Broadcaster) handleVoteMessage(msg *VoteMessage) {
	b.mu.RLock()
	handlers := make([]VoteHandler, len(b.voteHandlers))
	copy(handlers, b.voteHandlers)
	b.mu.RUnlock()

	for _, handler := range handlers {
		if err := handler(msg); err != nil {
			b.logger.Warn("Vote handler error", "error", err)
		}
	}
}

// subscribeModeration subscribes to moderation messages
func (b *Broadcaster) subscribeModeration() {
	defer b.wg.Done()

	sub, err := b.node.Subscribe(TopicModerator)
	if err != nil {
		b.logger.Error("Failed to subscribe to moderation", "error", err)
		return
	}

	b.logger.Info("Subscribed to moderation topic")

	for {
		msg, err := sub.Next(b.ctx)
		if err != nil {
			if b.ctx.Err() != nil {
				return
			}
			b.logger.Warn("Error reading moderation message", "error", err)
			continue
		}

		if msg.ReceivedFrom == b.node.GetPeerID() {
			continue
		}

		var moderationMsg ModerationMessage
		if err := json.Unmarshal(msg.Data, &moderationMsg); err != nil {
			b.logger.Warn("Failed to unmarshal moderation message", "error", err)
			continue
		}

		b.handleModerationMessage(&moderationMsg)
	}
}

// handleModerationMessage handles a moderation message
func (b *Broadcaster) handleModerationMessage(msg *ModerationMessage) {
	b.mu.RLock()
	handlers := make([]ModerationHandler, len(b.moderationHandlers))
	copy(handlers, b.moderationHandlers)
	b.mu.RUnlock()

	for _, handler := range handlers {
		if err := handler(msg); err != nil {
			b.logger.Warn("Moderation handler error", "error", err)
		}
	}
}
