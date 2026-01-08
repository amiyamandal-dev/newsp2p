package ipfs

import (
	"context"
	"fmt"
	"time"

	"github.com/amiyamandal-dev/newsp2p/internal/domain"
	"github.com/amiyamandal-dev/newsp2p/pkg/logger"
	shell "github.com/ipfs/go-ipfs-api"
)

// IPNSManager handles IPNS key management and publishing
type IPNSManager struct {
	shell  *shell.Shell
	logger *logger.Logger
}

// NewIPNSManager creates a new IPNS manager
func NewIPNSManager(sh *shell.Shell, logger *logger.Logger) *IPNSManager {
	return &IPNSManager{
		shell:  sh,
		logger: logger.WithComponent("ipns-manager"),
	}
}

// KeyInfo represents IPNS key information
type KeyInfo struct {
	Name string
	ID   string
}

// GenerateKey generates a new IPNS key
func (m *IPNSManager) GenerateKey(ctx context.Context, keyName string) (*KeyInfo, error) {
	// Check if key already exists
	keys, err := m.shell.KeyList(context.Background())
	if err != nil {
		m.logger.Error("Failed to list keys", "error", err)
		return nil, fmt.Errorf("failed to list keys: %w", err)
	}

	for _, key := range keys {
		if key.Name == keyName {
			m.logger.Info("Key already exists", "key_name", keyName, "key_id", key.Id)
			return &KeyInfo{Name: key.Name, ID: key.Id}, nil
		}
	}

	// Generate new key (default key type)
	key, err := m.shell.KeyGen(context.Background(), keyName)
	if err != nil {
		m.logger.Error("Failed to generate key", "key_name", keyName, "error", err)
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	m.logger.Info("Generated new IPNS key", "key_name", keyName, "key_id", key.Id)

	return &KeyInfo{Name: key.Name, ID: key.Id}, nil
}

// Publish publishes a CID to IPNS using a named key
func (m *IPNSManager) Publish(ctx context.Context, cid, keyName string) (string, error) {
	if cid == "" {
		return "", domain.ErrInvalidCID
	}

	// Publish with options
	response, err := m.shell.PublishWithDetails(
		cid,
		keyName,
		24*time.Hour,   // Lifetime
		30*time.Second, // TTL
		true,           // Resolve
	)

	if err != nil {
		m.logger.Error("Failed to publish to IPNS",
			"cid", cid,
			"key_name", keyName,
			"error", err,
		)
		return "", domain.ErrIPNSPublishFailed
	}

	ipnsPath := fmt.Sprintf("/ipns/%s", response.Name)
	m.logger.Info("Published to IPNS",
		"cid", cid,
		"key_name", keyName,
		"ipns_path", ipnsPath,
	)

	return ipnsPath, nil
}

// Resolve resolves an IPNS name to a CID
func (m *IPNSManager) Resolve(ctx context.Context, ipnsPath string) (string, error) {
	cid, err := m.shell.Resolve(ipnsPath)
	if err != nil {
		m.logger.Error("Failed to resolve IPNS", "ipns_path", ipnsPath, "error", err)
		return "", fmt.Errorf("failed to resolve IPNS: %w", err)
	}

	m.logger.Debug("Resolved IPNS", "ipns_path", ipnsPath, "cid", cid)
	return cid, nil
}

// ListKeys lists all IPNS keys
func (m *IPNSManager) ListKeys(ctx context.Context) ([]*KeyInfo, error) {
	keys, err := m.shell.KeyList(context.Background())
	if err != nil {
		m.logger.Error("Failed to list keys", "error", err)
		return nil, fmt.Errorf("failed to list keys: %w", err)
	}

	var keyInfos []*KeyInfo
	for _, key := range keys {
		keyInfos = append(keyInfos, &KeyInfo{
			Name: key.Name,
			ID:   key.Id,
		})
	}

	return keyInfos, nil
}

// GetKeyID gets the ID for a named key
func (m *IPNSManager) GetKeyID(ctx context.Context, keyName string) (string, error) {
	keys, err := m.shell.KeyList(context.Background())
	if err != nil {
		return "", fmt.Errorf("failed to list keys: %w", err)
	}

	for _, key := range keys {
		if key.Name == keyName {
			return key.Id, nil
		}
	}

	return "", fmt.Errorf("key not found: %s", keyName)
}

// EnsureKey ensures a key exists, creating it if necessary
func (m *IPNSManager) EnsureKey(ctx context.Context, keyName string) (*KeyInfo, error) {
	// Try to get existing key
	keyID, err := m.GetKeyID(ctx, keyName)
	if err == nil {
		return &KeyInfo{Name: keyName, ID: keyID}, nil
	}

	// Key doesn't exist, generate it
	return m.GenerateKey(ctx, keyName)
}
