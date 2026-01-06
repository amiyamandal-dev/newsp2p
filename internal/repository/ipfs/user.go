package ipfs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amiyamandal-dev/newsp2p/internal/domain"
	"github.com/amiyamandal-dev/newsp2p/internal/ipfs"
)

// UserRepo implements user repository using IPFS + IPNS
// User profiles are stored on IPFS, with IPNS pointing to latest version
type UserRepo struct {
	ipfsClient  *ipfs.Client
	ipnsManager *ipfs.IPNSManager
}

// NewUserRepo creates a new IPFS-based user repository
func NewUserRepo(client *ipfs.Client, ipnsManager *ipfs.IPNSManager) *UserRepo {
	return &UserRepo{
		ipfsClient:  client,
		ipnsManager: ipnsManager,
	}
}

// Create creates a new user and publishes to IPNS
func (r *UserRepo) Create(ctx context.Context, user *domain.User) error {
	// Serialize user
	userData, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("failed to marshal user: %w", err)
	}

	// Upload to IPFS
	cid, err := r.ipfsClient.Add(ctx, userData)
	if err != nil {
		return fmt.Errorf("failed to upload user to IPFS: %w", err)
	}

	// Create IPNS key for this user
	keyName := fmt.Sprintf("user-%s", user.Username)
	_, err = r.ipnsManager.GenerateKey(ctx, keyName)
	if err != nil {
		return fmt.Errorf("failed to generate IPNS key: %w", err)
	}

	// Publish to IPNS
	_, err = r.ipnsManager.Publish(ctx, cid, keyName)
	if err != nil {
		return fmt.Errorf("failed to publish to IPNS: %w", err)
	}

	return nil
}

// GetByUsername retrieves a user by username via IPNS
func (r *UserRepo) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	keyName := fmt.Sprintf("user-%s", username)

	// Get IPNS address
	keyID, err := r.ipnsManager.GetKeyID(ctx, keyName)
	if err != nil {
		return nil, domain.ErrUserNotFound
	}

	ipnsPath := fmt.Sprintf("/ipns/%s", keyID)

	// Resolve IPNS to CID
	cid, err := r.ipnsManager.Resolve(ctx, ipnsPath)
	if err != nil {
		return nil, domain.ErrUserNotFound
	}

	// Fetch from IPFS
	userData, err := r.ipfsClient.Cat(ctx, cid)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user from IPFS: %w", err)
	}

	// Deserialize
	var user domain.User
	if err := json.Unmarshal(userData, &user); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user: %w", err)
	}

	return &user, nil
}

// Update updates a user by publishing new version to IPNS
func (r *UserRepo) Update(ctx context.Context, user *domain.User) error {
	// Serialize user
	userData, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("failed to marshal user: %w", err)
	}

	// Upload to IPFS
	cid, err := r.ipfsClient.Add(ctx, userData)
	if err != nil {
		return fmt.Errorf("failed to upload user to IPFS: %w", err)
	}

	// Publish updated version to IPNS
	keyName := fmt.Sprintf("user-%s", user.Username)
	_, err = r.ipnsManager.Publish(ctx, cid, keyName)
	if err != nil {
		return fmt.Errorf("failed to publish to IPNS: %w", err)
	}

	return nil
}
