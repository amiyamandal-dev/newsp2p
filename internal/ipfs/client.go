package ipfs

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/amiyamandal-dev/newsp2p/internal/domain"
	"github.com/amiyamandal-dev/newsp2p/pkg/logger"
	shell "github.com/ipfs/go-ipfs-api"
)

// Client wraps the IPFS HTTP API client
type Client struct {
	shell      *shell.Shell
	timeout    time.Duration
	pinContent bool
	logger     *logger.Logger
}

// NewClient creates a new IPFS client
func NewClient(apiEndpoint string, timeout time.Duration, pinContent bool, logger *logger.Logger) *Client {
	sh := shell.NewShell(apiEndpoint)
	sh.SetTimeout(timeout)

	return &Client{
		shell:      sh,
		timeout:    timeout,
		pinContent: pinContent,
		logger:     logger.WithComponent("ipfs-client"),
	}
}

// Add uploads data to IPFS and returns the CID
func (c *Client) Add(ctx context.Context, data []byte) (string, error) {
	reader := bytes.NewReader(data)
	cid, err := c.AddWithRetry(ctx, reader, 3)
	if err != nil {
		c.logger.Error("Failed to add to IPFS", "error", err)
		return "", domain.ErrIPFSUploadFailed
	}

	c.logger.Debug("Added content to IPFS", "cid", cid, "size", len(data))

	// Pin if configured
	if c.pinContent {
		if err := c.Pin(ctx, cid); err != nil {
			c.logger.Warn("Failed to pin content", "cid", cid, "error", err)
			// Don't fail on pin error, content is already uploaded
		}
	}

	return cid, nil
}

// AddWithRetry uploads data to IPFS with retry logic
func (c *Client) AddWithRetry(ctx context.Context, reader io.Reader, retries int) (string, error) {
	var lastErr error

	for i := 0; i < retries; i++ {
		cid, err := c.shell.Add(reader)
		if err == nil {
			return cid, nil
		}

		lastErr = err
		c.logger.Warn("IPFS add attempt failed", "attempt", i+1, "error", err)

		// Wait before retry with exponential backoff
		if i < retries-1 {
			time.Sleep(time.Duration(i+1) * time.Second)
		}
	}

	return "", fmt.Errorf("failed after %d retries: %w", retries, lastErr)
}

// Cat retrieves data from IPFS by CID
func (c *Client) Cat(ctx context.Context, cid string) ([]byte, error) {
	if cid == "" {
		return nil, domain.ErrInvalidCID
	}

	reader, err := c.shell.Cat(cid)
	if err != nil {
		c.logger.Error("Failed to cat from IPFS", "cid", cid, "error", err)
		return nil, fmt.Errorf("failed to retrieve from IPFS: %w", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		c.logger.Error("Failed to read IPFS content", "cid", cid, "error", err)
		return nil, fmt.Errorf("failed to read IPFS content: %w", err)
	}

	c.logger.Debug("Retrieved content from IPFS", "cid", cid, "size", len(data))

	return data, nil
}

// Pin pins content to prevent garbage collection
func (c *Client) Pin(ctx context.Context, cid string) error {
	if err := c.shell.Pin(cid); err != nil {
		c.logger.Error("Failed to pin content", "cid", cid, "error", err)
		return fmt.Errorf("failed to pin %s: %w", cid, err)
	}

	c.logger.Debug("Pinned content", "cid", cid)
	return nil
}

// Unpin unpins content to allow garbage collection
func (c *Client) Unpin(ctx context.Context, cid string) error {
	if cid == "" {
		return nil // Nothing to unpin
	}

	if err := c.shell.Unpin(cid); err != nil {
		c.logger.Warn("Failed to unpin content", "cid", cid, "error", err)
		return fmt.Errorf("failed to unpin %s: %w", cid, err)
	}

	c.logger.Debug("Unpinned content", "cid", cid)
	return nil
}

// IsHealthy checks if the IPFS daemon is reachable
func (c *Client) IsHealthy(ctx context.Context) bool {
	_, err := c.shell.ID()
	if err != nil {
		c.logger.Warn("IPFS health check failed", "error", err)
		return false
	}
	return true
}

// GetID returns the IPFS node ID
func (c *Client) GetID(ctx context.Context) (string, error) {
	id, err := c.shell.ID()
	if err != nil {
		return "", fmt.Errorf("failed to get IPFS ID: %w", err)
	}
	return id.ID, nil
}

// Stats returns information about the IPFS node
func (c *Client) Stats(ctx context.Context) (map[string]interface{}, error) {
	id, err := c.shell.ID()
	if err != nil {
		return nil, fmt.Errorf("failed to get IPFS stats: %w", err)
	}

	stats := map[string]interface{}{
		"id":               id.ID,
		"agent_version":    id.AgentVersion,
		"protocol_version": id.ProtocolVersion,
		"addresses":        id.Addresses,
	}

	return stats, nil
}
