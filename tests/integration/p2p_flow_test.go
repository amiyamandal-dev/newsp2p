package integration

import (
	"context"
	"testing"

	"github.com/amiyamandal-dev/newsp2p/internal/auth"
	"github.com/amiyamandal-dev/newsp2p/internal/domain"
	"github.com/amiyamandal-dev/newsp2p/internal/service"
	"github.com/amiyamandal-dev/newsp2p/pkg/logger"
	"github.com/amiyamandal-dev/newsp2p/tests/mocks"
)

// MockBroadcaster captures broadcasted articles for testing
type MockBroadcaster struct {
	LastArticle *domain.Article
}

func (m *MockBroadcaster) BroadcastArticle(msgType string, article *domain.Article) error {
	m.LastArticle = article
	return nil
}

func TestP2PBroadcastingFlow(t *testing.T) {
	// 1. Setup Node A (The Author)
	envA := SetupTestEnv(t)
	defer envA.Cleanup()

	// 2. Setup Node B (The Reader/Replica)
	envB := SetupTestEnv(t)
	defer envB.Cleanup()

	ctx := context.Background()

	// --- Step 1: Node A creates an article ---

	// Register User A
	userA, err := envA.UserService.Register(ctx, &domain.UserRegisterRequest{
		Username: "alice_node_a",
		Password: "password_a",
	})
	if err != nil {
		t.Fatalf("Node A: Failed to register user: %v", err)
	}

	// Create Article on Node A
	// We need to inject a mock broadcaster to capture the message
	mockBroadcaster := &MockBroadcaster{}
	// Re-create service with mock broadcaster (since SetupTestEnv passes nil)
	envA.ArticleService = createServiceWithBroadcaster(envA, mockBroadcaster)

	articleReq := &domain.ArticleCreateRequest{
		Title:    "P2P is Awesome",
		Body:     "This article travels through the network.",
		Category: "technology",
		Tags:     []string{"p2p", "testing"},
	}

	articleA, err := envA.ArticleService.Create(ctx, articleReq, userA.ID, "127.0.0.1")
	if err != nil {
		t.Fatalf("Node A: Failed to create article: %v", err)
	}

	// Verify Node A has it
	if _, err := envA.ArticleRepo.GetByID(ctx, articleA.ID); err != nil {
		t.Fatalf("Node A: Article not found in local DB: %v", err)
	}

	// Verify it was broadcasted
	if mockBroadcaster.LastArticle == nil {
		t.Fatal("Node A: Article was not broadcasted")
	}
	if mockBroadcaster.LastArticle.ID != articleA.ID {
		t.Errorf("Node A: Broadcasted article ID mismatch")
	}

	// --- Step 2: Simulate Network Transport ---
	// We take the article from the MockBroadcaster and "send" it to Node B
	transportedArticle := mockBroadcaster.LastArticle

	// --- Step 3: Node B receives the article ---

	// Verify Node B does NOT have it yet
	if _, err := envB.ArticleRepo.GetByID(ctx, transportedArticle.ID); err == nil {
		t.Fatal("Node B: Article should not exist yet")
	}

	// Node B processes incoming message
	// Note: We need to register User A's public key on Node B?
	// In a real DHT, Node B would resolve the user. 
	// But HandleIncomingArticle relies on the Signature inside the Article struct 
	// and the AuthorPubKey inside the Article struct.
	// So it should work self-contained if verify logic is correct.

	err = envB.ArticleService.HandleIncomingArticle(transportedArticle)
	if err != nil {
		t.Fatalf("Node B: Failed to handle incoming article: %v", err)
	}

	// --- Step 4: Verify Node B has the article ---

	fetchedArticleB, err := envB.ArticleRepo.GetByID(ctx, transportedArticle.ID)
	if err != nil {
		t.Fatalf("Node B: Article NOT persisted after broadcast: %v", err)
	}

	if fetchedArticleB.Title != "P2P is Awesome" {
		t.Errorf("Node B: Title mismatch. Got %s", fetchedArticleB.Title)
	}
	if fetchedArticleB.OriginIP != "127.0.0.1" {
		t.Errorf("Node B: OriginIP mismatch. Got %s, expected 127.0.0.1", fetchedArticleB.OriginIP)
	}

	// --- Step 5: Verify it appears in Node B's Feed (ListRecent) ---
	
	recentB, err := envB.ArticleRepo.ListRecent(ctx, 10)
	if err != nil {
		t.Fatalf("Node B: Failed to list recent: %v", err)
	}

	found := false
	for _, art := range recentB {
		if art.ID == articleA.ID {
			found = true
			break
		}
	}

	if !found {
		t.Error("Node B: Article not found in recent feed")
	}

	t.Log("Success! Article created on Node A was successfully replicated to Node B")
}

// Helper to reinject broadcaster
func createServiceWithBroadcaster(env *TestEnv, b *MockBroadcaster) *service.ArticleService {
	// This is a bit hacky, recreating the service, but clean enough for test
	// We need to access the unexported fields or just create a new one using the same repos
	// Since we can't access env.UserService's private fields easily to get keys if needed,
	// but ArticleService constructor takes public interfaces.
	
	// We need to rebuild the service using the SAME repos from EnvA
	// We can't reuse the old service instance because we can't set the broadcaster field (private/struct)
	// So we make a new one.
	
	// Note: In real app, we'd probably use a Setter for broadcaster or construct it correctly first.
	// For this test, I'll copy the constructor logic from main/setup.
	
	// We need the signer from the original setup, but it's not exposed in TestEnv explicitly
	// except implicitly.
	// Let's create a new signer, it's stateless.
	signer := auth.NewArticleSigner()
	
	// We need the logger
	logVal, _ := logger.New("error", "text") // discard output
	
	return service.NewArticleService(
		env.ArticleRepo,
		env.UserRepo,
		mocks.NewMockIPFSClient(), // New mock is fine
		b,
		signer,
		nil,
		logVal,
	)
}
