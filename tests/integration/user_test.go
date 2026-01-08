package integration

import (
	"context"
	"testing"

	"github.com/amiyamandal-dev/newsp2p/internal/domain"
)

func TestUserFlow(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()

	// 1. Register User (P2P Identity)
	req := &domain.UserRegisterRequest{
		Username: "alice",
		Password: "strong-password-123",
	}

	user, err := env.UserService.Register(ctx, req)
	if err != nil {
		t.Fatalf("Failed to register user: %v", err)
	}

	if user.Username != req.Username {
		t.Errorf("Expected username %s, got %s", req.Username, user.Username)
	}
	if user.ID == "" {
		t.Error("Expected User ID (PeerID) to be generated")
	}
	t.Logf("Registered User ID (PeerID): %s", user.ID)

	// 2. Login
	loginReq := &domain.UserLoginRequest{
		Username: "alice",
		Password: "strong-password-123",
	}

	loginResp, err := env.UserService.Login(ctx, loginReq)
	if err != nil {
		t.Fatalf("Failed to login: %v", err)
	}

	if loginResp.Tokens.AccessToken == "" {
		t.Error("Expected access token")
	}

	// 3. Verify duplicate registration fails
	_, err = env.UserService.Register(ctx, req)
	if err != domain.ErrUserAlreadyExists {
		t.Errorf("Expected ErrUserAlreadyExists, got %v", err)
	}
}
