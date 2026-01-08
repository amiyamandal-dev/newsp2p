package p2p

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/amiyamandal-dev/newsp2p/pkg/logger"
)

// ReputationScore represents a user's reputation
type ReputationScore struct {
	DID          string    `json:"did"`
	Score        float64   `json:"score"`        // 0-100 scale
	ArticleCount int       `json:"article_count"`
	UpVotes      int       `json:"up_votes"`
	DownVotes    int       `json:"down_votes"`
	ReportCount  int       `json:"report_count"`  // Reports against this user
	LastUpdated  time.Time `json:"last_updated"`
}

// ReputationEvent represents a reputation-affecting event
type ReputationEvent struct {
	DID       string    `json:"did"`
	EventType string    `json:"event_type"` // "article_post", "up_vote", "down_vote", "report", "verified"
	Weight    float64   `json:"weight"`
	Timestamp time.Time `json:"timestamp"`
}

// ReputationSystem manages user reputation
type ReputationSystem struct {
	scores map[string]*ReputationScore
	mu     sync.RWMutex
	logger *logger.Logger
}

const (
	EventArticlePost  = "article_post"
	EventUpVote       = "up_vote"
	EventDownVote     = "down_vote"
	EventReport       = "report"
	EventVerified     = "verified"
	EventSpam         = "spam"

	InitialScore      = 50.0  // Starting reputation
	ArticlePostScore  = 2.0   // Points for posting article
	UpVoteScore       = 0.5   // Points for receiving upvote
	DownVoteScore     = -0.5  // Points for receiving downvote
	ReportPenalty     = -5.0  // Penalty for being reported
	VerifiedBonus     = 10.0  // Bonus for verified content
	SpamPenalty       = -10.0 // Heavy penalty for spam
)

// NewReputationSystem creates a new reputation system
func NewReputationSystem(log *logger.Logger) *ReputationSystem {
	return &ReputationSystem{
		scores: make(map[string]*ReputationScore),
		logger: log.WithComponent("reputation"),
	}
}

// GetScore retrieves a user's reputation score
func (rs *ReputationSystem) GetScore(did string) *ReputationScore {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	score, exists := rs.scores[did]
	if !exists {
		return &ReputationScore{
			DID:         did,
			Score:       InitialScore,
			LastUpdated: time.Now(),
		}
	}

	return score
}

// RecordEvent records a reputation event
func (rs *ReputationSystem) RecordEvent(event *ReputationEvent) error {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	score, exists := rs.scores[event.DID]
	if !exists {
		score = &ReputationScore{
			DID:         event.DID,
			Score:       InitialScore,
			LastUpdated: time.Now(),
		}
		rs.scores[event.DID] = score
	}

	// Apply event based on type
	switch event.EventType {
	case EventArticlePost:
		score.ArticleCount++
		score.Score += ArticlePostScore

	case EventUpVote:
		score.UpVotes++
		score.Score += UpVoteScore

	case EventDownVote:
		score.DownVotes++
		score.Score += DownVoteScore

	case EventReport:
		score.ReportCount++
		score.Score += ReportPenalty

	case EventVerified:
		score.Score += VerifiedBonus

	case EventSpam:
		score.Score += SpamPenalty

	default:
		return fmt.Errorf("unknown event type: %s", event.EventType)
	}

	// Clamp score to 0-100 range
	if score.Score < 0 {
		score.Score = 0
	}
	if score.Score > 100 {
		score.Score = 100
	}

	score.LastUpdated = time.Now()

	rs.logger.Debug("Reputation updated",
		"did", event.DID,
		"event", event.EventType,
		"new_score", score.Score,
	)

	return nil
}

// IsTrusted checks if a DID is trusted (reputation > 60)
func (rs *ReputationSystem) IsTrusted(did string) bool {
	score := rs.GetScore(did)
	return score.Score >= 60.0
}

// IsLowReputation checks if a DID has low reputation (< 30)
func (rs *ReputationSystem) IsLowReputation(did string) bool {
	score := rs.GetScore(did)
	return score.Score < 30.0
}

// GetTopUsers returns users with highest reputation
func (rs *ReputationSystem) GetTopUsers(limit int) []*ReputationScore {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	scores := make([]*ReputationScore, 0, len(rs.scores))
	for _, score := range rs.scores {
		scores = append(scores, score)
	}

	// Simple bubble sort for top users
	for i := 0; i < len(scores)-1 && i < limit; i++ {
		for j := i + 1; j < len(scores); j++ {
			if scores[j].Score > scores[i].Score {
				scores[i], scores[j] = scores[j], scores[i]
			}
		}
	}

	if len(scores) > limit {
		scores = scores[:limit]
	}

	return scores
}

// Export exports all reputation data
func (rs *ReputationSystem) Export() ([]byte, error) {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	return json.Marshal(rs.scores)
}

// Import imports reputation data
func (rs *ReputationSystem) Import(data []byte) error {
	var scores map[string]*ReputationScore
	if err := json.Unmarshal(data, &scores); err != nil {
		return fmt.Errorf("failed to unmarshal reputation data: %w", err)
	}

	rs.mu.Lock()
	defer rs.mu.Unlock()

	rs.scores = scores
	rs.logger.Info("Imported reputation data", "user_count", len(scores))

	return nil
}

// DecayScores applies time-based decay to reputation (call periodically)
func (rs *ReputationSystem) DecayScores(decayRate float64) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	now := time.Now()
	for did, score := range rs.scores {
		// Apply decay if score hasn't been updated recently
		daysSinceUpdate := now.Sub(score.LastUpdated).Hours() / 24
		if daysSinceUpdate > 7 {
			// Decay by decayRate per week
			decay := (daysSinceUpdate / 7) * decayRate
			score.Score -= decay
			if score.Score < InitialScore/2 {
				score.Score = InitialScore / 2 // Don't go below half of initial
			}
			score.LastUpdated = now
			rs.logger.Debug("Applied reputation decay", "did", did, "new_score", score.Score)
		}
	}
}

// CalculateContentTrust calculates trust score for content based on author reputation
func (rs *ReputationSystem) CalculateContentTrust(authorDID string, upvotes, downvotes int) float64 {
	authorScore := rs.GetScore(authorDID).Score

	// Weighted trust score combining author reputation and votes
	voteScore := float64(upvotes-downvotes) * 2.0
	trustScore := (authorScore * 0.6) + (voteScore * 0.4)

	// Normalize to 0-100
	if trustScore < 0 {
		trustScore = 0
	}
	if trustScore > 100 {
		trustScore = 100
	}

	return trustScore
}
