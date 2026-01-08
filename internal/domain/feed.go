package domain

import (
	"time"
)

// Feed represents a collection of articles published to IPNS
type Feed struct {
	ID           string    `json:"id" db:"id"`
	Name         string    `json:"name" db:"name" binding:"required,min=1,max=50"` // e.g., "global", "tech"
	IPNSKey      string    `json:"ipns_key" db:"ipns_key"`                         // IPNS key name
	IPNSAddress  string    `json:"ipns_address" db:"ipns_address"`                 // /ipns/...
	LastCID      string    `json:"last_cid" db:"last_cid"`                         // Latest feed CID
	LastSync     time.Time `json:"last_sync" db:"last_sync"`
	SyncInterval int       `json:"sync_interval" db:"sync_interval"` // Minutes
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// Validate validates the feed fields
func (f *Feed) Validate() error {
	if f.Name == "" || len(f.Name) > 50 {
		return ErrInvalidFeed
	}
	if f.SyncInterval < 1 {
		return ErrInvalidFeed
	}
	return nil
}

// FeedManifest represents the feed content published to IPFS
type FeedManifest struct {
	Version     string    `json:"version"`  // Manifest schema version
	Articles    []string  `json:"articles"` // Array of CIDs
	LastUpdated time.Time `json:"last_updated"`
	TotalCount  int       `json:"total_count"`
	Signature   string    `json:"signature"` // Feed signature
}

// FeedCreateRequest represents a request to create a feed
type FeedCreateRequest struct {
	Name         string `json:"name" binding:"required,min=1,max=50"`
	SyncInterval int    `json:"sync_interval" binding:"required,min=1"` // Minutes
}

// FeedUpdateRequest represents a request to update a feed
type FeedUpdateRequest struct {
	SyncInterval int `json:"sync_interval" binding:"omitempty,min=1"` // Minutes
}

// FeedArticle represents the association between a feed and an article
type FeedArticle struct {
	FeedID    string    `db:"feed_id"`
	ArticleID string    `db:"article_id"`
	Position  int       `db:"position"` // Order in feed
	AddedAt   time.Time `db:"added_at"`
}
