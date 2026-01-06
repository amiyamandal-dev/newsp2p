package auth

import (
	"crypto/ed25519"
	"fmt"

	"github.com/amiyamandal-dev/newsp2p/internal/domain"
	"github.com/amiyamandal-dev/newsp2p/pkg/crypto"
)

// ArticleSigner handles article signing and verification
type ArticleSigner struct{}

// NewArticleSigner creates a new article signer
func NewArticleSigner() *ArticleSigner {
	return &ArticleSigner{}
}

// SignArticle signs an article with a private key
func (s *ArticleSigner) SignArticle(article *domain.Article, privateKey ed25519.PrivateKey) error {
	// Get signable content
	content, err := article.GetSignableContent()
	if err != nil {
		return fmt.Errorf("failed to get signable content: %w", err)
	}

	// Sign the content
	signature, err := crypto.Sign(content, privateKey)
	if err != nil {
		return fmt.Errorf("failed to sign article: %w", err)
	}

	article.Signature = signature
	return nil
}

// VerifyArticle verifies an article's signature
func (s *ArticleSigner) VerifyArticle(article *domain.Article) error {
	// Parse public key
	publicKey, err := crypto.PublicKeyFromString(article.AuthorPubKey)
	if err != nil {
		return fmt.Errorf("failed to parse public key: %w", err)
	}

	// Get signable content
	content, err := article.GetSignableContent()
	if err != nil {
		return fmt.Errorf("failed to get signable content: %w", err)
	}

	// Verify signature
	valid, err := crypto.Verify(content, article.Signature, publicKey)
	if err != nil {
		return fmt.Errorf("failed to verify signature: %w", err)
	}

	if !valid {
		return domain.ErrInvalidSignature
	}

	return nil
}
