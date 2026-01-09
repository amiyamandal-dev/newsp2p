package p2p

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"

	"github.com/amiyamandal-dev/newsp2p/pkg/logger"
)

const (
	// Discovery intervals
	BootstrapCheckInterval = 30 * time.Second
	PeerDiscoveryInterval  = 15 * time.Second
	ReconnectInterval      = 10 * time.Second

	// Minimum peers before we try to find more
	MinDesiredPeers = 3

	// Cache file for discovered bootstrap servers
	BootstrapCacheFile = "bootstrap_cache.json"
)

// BootstrapInfo represents bootstrap server information
type BootstrapInfo struct {
	PeerID     string   `json:"peer_id"`
	Addresses  []string `json:"addresses"`
	Rendezvous string   `json:"rendezvous"`
	Protocol   string   `json:"protocol"`
	Version    string   `json:"version"`
	LastSeen   time.Time `json:"last_seen"`
}

// AutoDiscovery handles automatic bootstrap server discovery and connection
type AutoDiscovery struct {
	ctx    context.Context
	cancel context.CancelFunc

	host   host.Host
	logger *logger.Logger

	// Bootstrap server URLs to check (can be local or remote)
	bootstrapURLs []string

	// Known bootstrap peers (from config + discovered)
	knownBootstraps map[string]*BootstrapInfo
	mu              sync.RWMutex

	// Data directory for caching
	dataDir string

	// Callbacks
	onPeerConnected    func(peer.ID)
	onPeerDisconnected func(peer.ID)
}

// NewAutoDiscovery creates a new auto-discovery service
func NewAutoDiscovery(h host.Host, dataDir string, log *logger.Logger) *AutoDiscovery {
	ctx, cancel := context.WithCancel(context.Background())

	ad := &AutoDiscovery{
		ctx:             ctx,
		cancel:          cancel,
		host:            h,
		logger:          log.WithComponent("auto-discovery"),
		bootstrapURLs:   getDefaultBootstrapURLs(),
		knownBootstraps: make(map[string]*BootstrapInfo),
		dataDir:         dataDir,
	}

	// Load cached bootstrap info
	ad.loadCache()

	return ad
}

// getDefaultBootstrapURLs returns default bootstrap server URLs to check
func getDefaultBootstrapURLs() []string {
	urls := []string{
		// Local bootstrap server (for development/same network)
		"http://localhost:8081/bootstrap",
		"http://127.0.0.1:8081/bootstrap",
	}

	// Check for custom bootstrap URL from environment
	if customURL := os.Getenv("BOOTSTRAP_URL"); customURL != "" {
		urls = append([]string{customURL}, urls...)
	}

	return urls
}

// AddBootstrapURL adds a bootstrap server URL to check
func (ad *AutoDiscovery) AddBootstrapURL(url string) {
	ad.mu.Lock()
	defer ad.mu.Unlock()

	// Add to front (priority)
	ad.bootstrapURLs = append([]string{url}, ad.bootstrapURLs...)
}

// AddBootstrapPeer adds a known bootstrap peer address
func (ad *AutoDiscovery) AddBootstrapPeer(addrStr string) error {
	addr, err := multiaddr.NewMultiaddr(addrStr)
	if err != nil {
		return fmt.Errorf("invalid multiaddr: %w", err)
	}

	peerInfo, err := peer.AddrInfoFromP2pAddr(addr)
	if err != nil {
		return fmt.Errorf("invalid peer address: %w", err)
	}

	ad.mu.Lock()
	ad.knownBootstraps[peerInfo.ID.String()] = &BootstrapInfo{
		PeerID:    peerInfo.ID.String(),
		Addresses: []string{addrStr},
		LastSeen:  time.Now(),
	}
	ad.mu.Unlock()

	return nil
}

// Start starts the auto-discovery service
func (ad *AutoDiscovery) Start() {
	ad.logger.Info("Starting auto-discovery service")

	// Initial bootstrap connection
	go ad.connectToBootstraps()

	// Start background routines
	go ad.bootstrapCheckLoop()
	go ad.peerMaintenanceLoop()

	// Setup connection notifications
	ad.host.Network().Notify(&network.NotifyBundle{
		ConnectedF: func(n network.Network, c network.Conn) {
			if ad.onPeerConnected != nil {
				ad.onPeerConnected(c.RemotePeer())
			}
		},
		DisconnectedF: func(n network.Network, c network.Conn) {
			if ad.onPeerDisconnected != nil {
				ad.onPeerDisconnected(c.RemotePeer())
			}
			// Try to reconnect if we lose too many peers
			if len(n.Peers()) < MinDesiredPeers {
				go ad.connectToBootstraps()
			}
		},
	})
}

// Stop stops the auto-discovery service
func (ad *AutoDiscovery) Stop() {
	ad.logger.Info("Stopping auto-discovery service")
	ad.saveCache()
	ad.cancel()
}

// bootstrapCheckLoop periodically checks for new bootstrap servers
func (ad *AutoDiscovery) bootstrapCheckLoop() {
	// Initial discovery
	ad.discoverBootstraps()

	ticker := time.NewTicker(BootstrapCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ad.ctx.Done():
			return
		case <-ticker.C:
			ad.discoverBootstraps()
		}
	}
}

// peerMaintenanceLoop maintains minimum peer connections
func (ad *AutoDiscovery) peerMaintenanceLoop() {
	ticker := time.NewTicker(PeerDiscoveryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ad.ctx.Done():
			return
		case <-ticker.C:
			peers := ad.host.Network().Peers()
			if len(peers) < MinDesiredPeers {
				ad.logger.Debug("Low peer count, attempting to connect to more peers",
					"current", len(peers), "desired", MinDesiredPeers)
				ad.connectToBootstraps()
			}
		}
	}
}

// discoverBootstraps discovers bootstrap servers from URLs
func (ad *AutoDiscovery) discoverBootstraps() {
	ad.mu.RLock()
	urls := make([]string, len(ad.bootstrapURLs))
	copy(urls, ad.bootstrapURLs)
	ad.mu.RUnlock()

	for _, url := range urls {
		info, err := ad.fetchBootstrapInfo(url)
		if err != nil {
			ad.logger.Debug("Failed to fetch bootstrap info", "url", url, "error", err)
			continue
		}

		ad.mu.Lock()
		info.LastSeen = time.Now()
		ad.knownBootstraps[info.PeerID] = info
		ad.mu.Unlock()

		ad.logger.Info("Discovered bootstrap server", "peer_id", info.PeerID[:16]+"...")
	}
}

// fetchBootstrapInfo fetches bootstrap info from a URL
func (ad *AutoDiscovery) fetchBootstrapInfo(url string) (*BootstrapInfo, error) {
	ctx, cancel := context.WithTimeout(ad.ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var info BootstrapInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}

	return &info, nil
}

// connectToBootstraps connects to known bootstrap servers
func (ad *AutoDiscovery) connectToBootstraps() {
	ad.mu.RLock()
	bootstraps := make([]*BootstrapInfo, 0, len(ad.knownBootstraps))
	for _, info := range ad.knownBootstraps {
		bootstraps = append(bootstraps, info)
	}
	ad.mu.RUnlock()

	if len(bootstraps) == 0 {
		ad.logger.Debug("No bootstrap servers known, will retry later")
		return
	}

	connected := 0
	for _, info := range bootstraps {
		// Parse peer ID
		pid, err := peer.Decode(info.PeerID)
		if err != nil {
			ad.logger.Warn("Invalid peer ID in bootstrap info", "peer_id", info.PeerID)
			continue
		}

		// Skip if already connected
		if ad.host.Network().Connectedness(pid) == network.Connected {
			connected++
			continue
		}

		// Try each address
		for _, addrStr := range info.Addresses {
			addr, err := multiaddr.NewMultiaddr(addrStr)
			if err != nil {
				continue
			}

			// Extract peer info
			peerInfo, err := peer.AddrInfoFromP2pAddr(addr)
			if err != nil {
				continue
			}

			// Try to connect
			ctx, cancel := context.WithTimeout(ad.ctx, 10*time.Second)
			err = ad.host.Connect(ctx, *peerInfo)
			cancel()

			if err != nil {
				ad.logger.Debug("Failed to connect to bootstrap", "addr", addrStr[:50]+"...", "error", err)
				continue
			}

			ad.logger.Info("Connected to bootstrap server", "peer_id", pid.String()[:16]+"...")
			connected++
			break
		}
	}

	if connected > 0 {
		ad.logger.Info("Bootstrap connection complete", "connected", connected)
	}
}

// loadCache loads cached bootstrap info from disk
func (ad *AutoDiscovery) loadCache() {
	cachePath := filepath.Join(ad.dataDir, BootstrapCacheFile)
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return // No cache file, that's okay
	}

	var cached map[string]*BootstrapInfo
	if err := json.Unmarshal(data, &cached); err != nil {
		ad.logger.Warn("Failed to parse bootstrap cache", "error", err)
		return
	}

	ad.mu.Lock()
	for k, v := range cached {
		// Only use entries less than 24 hours old
		if time.Since(v.LastSeen) < 24*time.Hour {
			ad.knownBootstraps[k] = v
		}
	}
	ad.mu.Unlock()

	ad.logger.Debug("Loaded bootstrap cache", "entries", len(ad.knownBootstraps))
}

// saveCache saves bootstrap info to disk
func (ad *AutoDiscovery) saveCache() {
	ad.mu.RLock()
	data, err := json.MarshalIndent(ad.knownBootstraps, "", "  ")
	ad.mu.RUnlock()

	if err != nil {
		ad.logger.Warn("Failed to marshal bootstrap cache", "error", err)
		return
	}

	cachePath := filepath.Join(ad.dataDir, BootstrapCacheFile)
	if err := os.MkdirAll(filepath.Dir(cachePath), 0755); err != nil {
		return
	}

	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		ad.logger.Warn("Failed to save bootstrap cache", "error", err)
	}
}

// GetConnectedBootstraps returns the number of connected bootstrap servers
func (ad *AutoDiscovery) GetConnectedBootstraps() int {
	ad.mu.RLock()
	defer ad.mu.RUnlock()

	count := 0
	for _, info := range ad.knownBootstraps {
		pid, err := peer.Decode(info.PeerID)
		if err != nil {
			continue
		}
		if ad.host.Network().Connectedness(pid) == network.Connected {
			count++
		}
	}
	return count
}

// OnPeerConnected sets a callback for peer connection events
func (ad *AutoDiscovery) OnPeerConnected(fn func(peer.ID)) {
	ad.onPeerConnected = fn
}

// OnPeerDisconnected sets a callback for peer disconnection events
func (ad *AutoDiscovery) OnPeerDisconnected(fn func(peer.ID)) {
	ad.onPeerDisconnected = fn
}
