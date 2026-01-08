package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/amiyamandal-dev/newsp2p/internal/auth"
	"github.com/amiyamandal-dev/newsp2p/internal/domain"
	"github.com/amiyamandal-dev/newsp2p/internal/ipfs"
	"github.com/amiyamandal-dev/newsp2p/internal/repository"
	"github.com/amiyamandal-dev/newsp2p/pkg/crypto"
	"github.com/amiyamandal-dev/newsp2p/pkg/logger"
)

// SearchIndexer defines the interface for search indexing
type SearchIndexer interface {
	IndexArticle(ctx context.Context, article *domain.Article) error
	UpdateArticle(ctx context.Context, article *domain.Article) error
	DeleteArticle(ctx context.Context, articleID string) error
}

// ArticleService handles article-related business logic
type ArticleService struct {
	articleRepo repository.ArticleRepository
	userRepo    repository.UserRepository
	ipfsClient  *ipfs.Client
	signer      *auth.ArticleSigner
	indexer     SearchIndexer
	logger      *logger.Logger
}

// NewArticleService creates a new article service
func NewArticleService(
	articleRepo repository.ArticleRepository,
	userRepo repository.UserRepository,
	ipfsClient *ipfs.Client,
	signer *auth.ArticleSigner,
	indexer SearchIndexer,
	logger *logger.Logger,
) *ArticleService {
	return &ArticleService{
		articleRepo: articleRepo,
		userRepo:    userRepo,
		ipfsClient:  ipfsClient,
		signer:      signer,
		indexer:     indexer,
		logger:      logger.WithComponent("article-service"),
	}
}

// Create creates a new article
func (s *ArticleService) Create(ctx context.Context, req *domain.ArticleCreateRequest, userID string) (*domain.Article, error) {
	// Get user with private key for signing
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get user", "user_id", userID, "error", err)
		return nil, err
	}

	if !user.IsActive {
		return nil, domain.ErrUserNotActive
	}

	// Decrypt private key using password hash as key derivation material
	// The private key was encrypted during registration with the user's password
	// We use the password hash as a secure key since we don't have the original password
	privateKey, err := crypto.DecryptPrivateKey(user.PrivateKey, user.PasswordHash)
	if err != nil {
		s.logger.Error("Failed to decrypt private key", "user_id", userID, "error", err)
		return nil, fmt.Errorf("failed to decrypt private key: %w", err)
	}

	// Create article
	article := &domain.Article{
		ID:           uuid.New().String(),
		Title:        req.Title,
		Body:         req.Body,
		Author:       user.Username,
		AuthorPubKey: user.PublicKey,
		Timestamp:    time.Now(),
		Tags:         req.Tags,
		Category:     req.Category,
		Version:      1,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Validate article
	if err := article.Validate(); err != nil {
		return nil, err
	}

	// Sign article
	if err := s.signer.SignArticle(article, privateKey); err != nil {
		s.logger.Error("Failed to sign article", "article_id", article.ID, "error", err)
		return nil, fmt.Errorf("failed to sign article: %w", err)
	}

	// Serialize article to JSON
	articleJSON, err := article.ToJSON()
	if err != nil {
		s.logger.Error("Failed to serialize article", "article_id", article.ID, "error", err)
		return nil, fmt.Errorf("failed to serialize article: %w", err)
	}

	// Upload to IPFS
	cid, err := s.ipfsClient.Add(ctx, articleJSON)
	if err != nil {
		s.logger.Error("Failed to upload to IPFS", "article_id", article.ID, "error", err)
		return nil, err
	}

	article.CID = cid

	// Store in database
	if err := s.articleRepo.Create(ctx, article); err != nil {
		s.logger.Error("Failed to store article", "article_id", article.ID, "error", err)
		return nil, fmt.Errorf("failed to store article: %w", err)
	}

	// Index for search
	if s.indexer != nil {
		if err := s.indexer.IndexArticle(ctx, article); err != nil {
			s.logger.Warn("Failed to index article", "article_id", article.ID, "error", err)
			// Don't fail on indexing error
		}
	}

	s.logger.Info("Article created successfully",
		"article_id", article.ID,
		"cid", cid,
		"author", user.Username,
	)

	return article, nil
}

// GetByCID retrieves an article by CID (from DB or IPFS)
func (s *ArticleService) GetByCID(ctx context.Context, cid string) (*domain.Article, error) {
	// Try to get from database first
	article, err := s.articleRepo.GetByCID(ctx, cid)
	if err == nil {
		s.logger.Debug("Retrieved article from database", "cid", cid)
		return article, nil
	}

	if err != domain.ErrArticleNotFound {
		s.logger.Error("Database error", "cid", cid, "error", err)
		return nil, err
	}

	// Not in database, fetch from IPFS
	s.logger.Debug("Article not in database, fetching from IPFS", "cid", cid)
	data, err := s.ipfsClient.Cat(ctx, cid)
	if err != nil {
		s.logger.Error("Failed to fetch from IPFS", "cid", cid, "error", err)
		return nil, domain.ErrArticleNotFound
	}

	// Parse article
	article, err = domain.FromJSON(data)
	if err != nil {
		s.logger.Error("Failed to parse article JSON", "cid", cid, "error", err)
		return nil, fmt.Errorf("failed to parse article: %w", err)
	}

	// Verify signature
	if err := s.signer.VerifyArticle(article); err != nil {
		s.logger.Warn("Article signature verification failed", "cid", cid, "error", err)
		return nil, domain.ErrInvalidSignature
	}

	s.logger.Info("Retrieved and verified article from IPFS", "cid", cid)

	// Optionally cache in database
	// (we'll skip this for now to keep it simple)

	return article, nil
}

// GetByID retrieves an article by ID
func (s *ArticleService) GetByID(ctx context.Context, id string) (*domain.Article, error) {
	article, err := s.articleRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return article, nil
}

// List retrieves articles with pagination and filtering
func (s *ArticleService) List(ctx context.Context, filter *domain.ArticleListFilter) ([]*domain.Article, int, error) {
	// Set defaults
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.Limit < 1 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100 // Max limit
	}

	articles, total, err := s.articleRepo.List(ctx, filter)
	if err != nil {
		s.logger.Error("Failed to list articles", "error", err)
		return nil, 0, err
	}

	return articles, total, nil
}

// Update updates an existing article
func (s *ArticleService) Update(ctx context.Context, id string, req *domain.ArticleUpdateRequest, userID string) (*domain.Article, error) {
	// Get existing article
	article, err := s.articleRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Get user
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Check authorization (only author can update)
	if article.Author != user.Username {
		return nil, domain.ErrForbidden
	}

	// Update fields
	if req.Title != "" {
		article.Title = req.Title
	}
	if req.Body != "" {
		article.Body = req.Body
	}
	if req.Tags != nil {
		article.Tags = req.Tags
	}
	if req.Category != "" {
		article.Category = req.Category
	}
	article.UpdatedAt = time.Now()

	// Validate
	if err := article.Validate(); err != nil {
		return nil, err
	}

	// Update in database
	if err := s.articleRepo.Update(ctx, article); err != nil {
		s.logger.Error("Failed to update article", "article_id", id, "error", err)
		return nil, fmt.Errorf("failed to update article: %w", err)
	}

	// Update search index
	if s.indexer != nil {
		if err := s.indexer.UpdateArticle(ctx, article); err != nil {
			s.logger.Warn("Failed to update article index", "article_id", id, "error", err)
		}
	}

	s.logger.Info("Article updated successfully", "article_id", id)

	return article, nil
}

// Delete deletes an article
func (s *ArticleService) Delete(ctx context.Context, id string, userID string) error {
	// Get article
	article, err := s.articleRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Get user
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	// Check authorization
	if article.Author != user.Username {
		return domain.ErrForbidden
	}

	// Delete from database
	if err := s.articleRepo.Delete(ctx, id); err != nil {
		s.logger.Error("Failed to delete article", "article_id", id, "error", err)
		return fmt.Errorf("failed to delete article: %w", err)
	}

	// Delete from search index
	if s.indexer != nil {
		if err := s.indexer.DeleteArticle(ctx, id); err != nil {
			s.logger.Warn("Failed to delete article from index", "article_id", id, "error", err)
		}
	}

	// Optionally unpin from IPFS
	if article.CID != "" {
		if err := s.ipfsClient.Unpin(ctx, article.CID); err != nil {
			s.logger.Warn("Failed to unpin article from IPFS", "cid", article.CID, "error", err)
		}
	}

	s.logger.Info("Article deleted successfully", "article_id", id)

	return nil
}

// VerifySignature verifies an article's signature
func (s *ArticleService) VerifySignature(ctx context.Context, cid string) (bool, error) {
	article, err := s.GetByCID(ctx, cid)
	if err != nil {
		return false, err
	}

	if err := s.signer.VerifyArticle(article); err != nil {
		return false, nil
	}

	return true, nil
}
