package domain

import (
	"encoding/json"
	"time"
)

// Article represents a news article
type Article struct {
	ID           string    `json:"id" db:"id"`
	CID          string    `json:"cid" db:"cid"` // IPFS content ID
	Title        string    `json:"title" db:"title" binding:"required,min=1,max=200"`
	Body         string    `json:"body" db:"body" binding:"required,min=1"`
	Author       string    `json:"author" db:"author" binding:"required"`
	AuthorPubKey string    `json:"author_pubkey" db:"author_pubkey"` // For verification
	Signature    string    `json:"signature" db:"signature"`         // Article signature
	Timestamp    time.Time `json:"timestamp" db:"timestamp"`
	Tags         []string  `json:"tags" db:"tags"` // JSON array in SQLite
	Category     string    `json:"category" db:"category"`
	Version      int       `json:"version" db:"version"` // For updates
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// SignableContent represents the content to be signed
type SignableContent struct {
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Author    string    `json:"author"`
	Timestamp time.Time `json:"timestamp"`
	Tags      []string  `json:"tags"`
	Category  string    `json:"category"`
}

// GetSignableContent returns the canonical content for signing
func (a *Article) GetSignableContent() ([]byte, error) {
	content := SignableContent{
		Title:     a.Title,
		Body:      a.Body,
		Author:    a.Author,
		Timestamp: a.Timestamp,
		Tags:      a.Tags,
		Category:  a.Category,
	}
	return json.Marshal(content)
}

// Validate validates the article fields
func (a *Article) Validate() error {
	if a.Title == "" {
		return ErrInvalidArticle
	}
	if len(a.Title) > 200 {
		return ErrInvalidArticle
	}
	if a.Body == "" {
		return ErrInvalidArticle
	}
	if a.Author == "" {
		return ErrInvalidArticle
	}
	return nil
}

// ToJSON converts article to JSON
func (a *Article) ToJSON() ([]byte, error) {
	return json.Marshal(a)
}

// FromJSON parses JSON into article
func FromJSON(data []byte) (*Article, error) {
	var article Article
	if err := json.Unmarshal(data, &article); err != nil {
		return nil, err
	}
	return &article, nil
}

// ArticleCreateRequest represents a request to create an article
type ArticleCreateRequest struct {
	Title    string   `json:"title" binding:"required,min=1,max=200"`
	Body     string   `json:"body" binding:"required,min=1"`
	Tags     []string `json:"tags"`
	Category string   `json:"category"`
}

// ArticleUpdateRequest represents a request to update an article
type ArticleUpdateRequest struct {
	Title    string   `json:"title" binding:"omitempty,min=1,max=200"`
	Body     string   `json:"body" binding:"omitempty,min=1"`
	Tags     []string `json:"tags"`
	Category string   `json:"category"`
}

// ArticleListFilter represents filters for listing articles
type ArticleListFilter struct {
	Author   string
	Category string
	Tags     []string
	FromDate time.Time
	ToDate   time.Time
	Page     int
	Limit    int
}
