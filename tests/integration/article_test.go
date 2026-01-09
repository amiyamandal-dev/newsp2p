package integration

import (
	"context"
	"testing"

	"github.com/amiyamandal-dev/newsp2p/internal/domain"
)

func TestArticleFlow(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()

	// 1. Register User (Author)
	userReq := &domain.UserRegisterRequest{
		Username: "bob",
		Password: "password123",
	}
	user, err := env.UserService.Register(ctx, userReq)
	if err != nil {
		t.Fatalf("Failed to register user: %v", err)
	}

	// 2. Create Article
	articleReq := &domain.ArticleCreateRequest{
		Title:    "Hello World",
		Body:     "This is my first P2P article content.",
		Category: "technology",
		Tags:     []string{"p2p", "news"},
	}

	article, err := env.ArticleService.Create(ctx, articleReq, user.ID, "127.0.0.1")
	if err != nil {
		t.Fatalf("Failed to create article: %v", err)
	}

	if article.Title != articleReq.Title {
		t.Errorf("Expected title %s, got %s", articleReq.Title, article.Title)
	}
	if article.CID == "" {
		t.Error("Expected CID to be generated (mocked)")
	}
	if article.Signature == "" {
		t.Error("Expected article to be signed")
	}

	t.Logf("Created Article CID: %s", article.CID)

	// 3. Retrieve Article by CID
	fetchedArticle, err := env.ArticleService.GetByCID(ctx, article.CID)
	if err != nil {
		t.Fatalf("Failed to get article by CID: %v", err)
	}

	if fetchedArticle.ID != article.ID {
		t.Errorf("Fetched article ID mismatch")
	}

	// 4. List Articles
	articles, total, err := env.ArticleService.List(ctx, &domain.ArticleListFilter{
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("Failed to list articles: %v", err)
	}

	if total != 1 {
		t.Errorf("Expected 1 article, got %d", total)
	}
	if len(articles) != 1 {
		t.Errorf("Expected 1 article in list, got %d", len(articles))
	}
}
