package p2p

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	dutil "github.com/libp2p/go-libp2p/p2p/discovery/util"
	"github.com/multiformats/go-multiaddr"

	"github.com/amiyamandal-dev/newsp2p/pkg/logger"
)

// P2PNode represents a peer-to-peer node in the network
type P2PNode struct {
	ctx    context.Context
	cancel context.CancelFunc

	host   host.Host
	dht    *dht.IpfsDHT
	pubsub *pubsub.PubSub

	privKey crypto.PrivKey
	peerID  peer.ID

	discovery *drouting.RoutingDiscovery

	topics map[string]*pubsub.Topic
	subs   map[string]*pubsub.Subscription
	mu     sync.RWMutex

	logger *logger.Logger
}

// Config holds P2P node configuration
type Config struct {
	ListenAddrs   []string
	BootstrapPeers []string
	ProtocolID    protocol.ID
	Rendezvous    string
}

// DefaultConfig returns default P2P configuration
func DefaultConfig() *Config {
	return &Config{
		ListenAddrs: []string{
			"/ip4/0.0.0.0/tcp/0",
			"/ip4/0.0.0.0/udp/0/quic-v1",
		},
		BootstrapPeers: []string{
			// IPFS bootstrap nodes
			"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
			"/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
		},
		ProtocolID: "/newsp2p/1.0.0",
		Rendezvous: "newsp2p-network",
	}
}

// NewP2PNode creates a new P2P node
func NewP2PNode(ctx context.Context, cfg *Config, log *logger.Logger) (*P2PNode, error) {
	ctx, cancel := context.WithCancel(ctx)

	// Load or generate identity
	privKey, err := loadOrGenerateKey("data/node_key")
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to load or generate key: %w", err)
	}

	// Parse listen addresses
	var listenAddrs []multiaddr.Multiaddr
	for _, addrStr := range cfg.ListenAddrs {
		addr, err := multiaddr.NewMultiaddr(addrStr)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("invalid listen address %s: %w", addrStr, err)
		}
		listenAddrs = append(listenAddrs, addr)
	}

	// Create libp2p host
	h, err := libp2p.New(
		libp2p.Identity(privKey),
		libp2p.ListenAddrs(listenAddrs...),
		libp2p.DefaultSecurity,
		libp2p.DefaultTransports,
		libp2p.NATPortMap(),
		libp2p.EnableNATService(),
		libp2p.EnableRelay(),
	)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create host: %w", err)
	}

	peerID := h.ID()
	log.Info("P2P node created",
		"peer_id", peerID.String(),
		"addresses", h.Addrs(),
	)

	// Setup DHT for peer discovery
	kdht, err := dht.New(ctx, h, dht.Mode(dht.ModeServer))
	if err != nil {
		h.Close()
		cancel()
		return nil, fmt.Errorf("failed to create DHT: %w", err)
	}

	// Bootstrap DHT
	if err = kdht.Bootstrap(ctx); err != nil {
		h.Close()
		cancel()
		return nil, fmt.Errorf("failed to bootstrap DHT: %w", err)
	}

	// Setup PubSub with Gossip
	ps, err := pubsub.NewGossipSub(ctx, h,
		pubsub.WithMessageSignaturePolicy(pubsub.StrictSign),
		pubsub.WithFloodPublish(true),
	)
	if err != nil {
		h.Close()
		cancel()
		return nil, fmt.Errorf("failed to create pubsub: %w", err)
	}

	// Setup discovery
	discovery := drouting.NewRoutingDiscovery(kdht)

	node := &P2PNode{
		ctx:       ctx,
		cancel:    cancel,
		host:      h,
		dht:       kdht,
		pubsub:    ps,
		privKey:   privKey,
		peerID:    peerID,
		discovery: discovery,
		topics:    make(map[string]*pubsub.Topic),
		subs:      make(map[string]*pubsub.Subscription),
		logger:    log.WithComponent("p2p-node"),
	}

	// Connect to bootstrap peers
	go node.connectToBootstrapPeers(cfg.BootstrapPeers)

	// Advertise this node
	go node.advertise(cfg.Rendezvous)

	// Find peers
	go node.findPeers(cfg.Rendezvous)

	return node, nil
}

// loadOrGenerateKey loads a private key from file or generates a new one
func loadOrGenerateKey(path string) (crypto.PrivKey, error) {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// Try to read key from file
	if _, err := os.Stat(path); err == nil {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read key file: %w", err)
		}
		privKey, err := crypto.UnmarshalPrivateKey(data)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal private key: %w", err)
		}
		return privKey, nil
	}

	// Generate new key
	privKey, _, err := crypto.GenerateKeyPair(crypto.Ed25519, -1)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}

	// Save key to file
	data, err := crypto.MarshalPrivateKey(privKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal private key: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return nil, fmt.Errorf("failed to write key file: %w", err)
	}

	return privKey, nil
}

// connectToBootstrapPeers connects to bootstrap peers
func (n *P2PNode) connectToBootstrapPeers(bootstrapPeers []string) {
	connected := 0
	for _, peerAddr := range bootstrapPeers {
		addr, err := multiaddr.NewMultiaddr(peerAddr)
		if err != nil {
			n.logger.Warn("Invalid bootstrap peer address", "addr", peerAddr, "error", err)
			continue
		}

		peerInfo, err := peer.AddrInfoFromP2pAddr(addr)
		if err != nil {
			n.logger.Warn("Failed to parse peer info", "addr", peerAddr, "error", err)
			continue
		}

		if err := n.host.Connect(n.ctx, *peerInfo); err != nil {
			n.logger.Warn("Failed to connect to bootstrap peer", "peer", peerInfo.ID, "error", err)
		} else {
			n.logger.Info("Connected to bootstrap peer", "peer", peerInfo.ID)
			connected++
		}
	}

	n.logger.Info("Bootstrap complete", "connected_peers", connected)
}

// advertise advertises this node on the network
func (n *P2PNode) advertise(rendezvous string) {
	dutil.Advertise(n.ctx, n.discovery, rendezvous)
	n.logger.Info("Started advertising on network", "rendezvous", rendezvous)

	// Re-advertise periodically
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-n.ctx.Done():
			return
		case <-ticker.C:
			dutil.Advertise(n.ctx, n.discovery, rendezvous)
		}
	}
}

// findPeers finds and connects to peers
func (n *P2PNode) findPeers(rendezvous string) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-n.ctx.Done():
			return
		case <-ticker.C:
			peerChan, err := n.discovery.FindPeers(n.ctx, rendezvous)
			if err != nil {
				n.logger.Warn("Failed to find peers", "error", err)
				continue
			}

			for peer := range peerChan {
				if peer.ID == n.peerID {
					continue
				}

				if n.host.Network().Connectedness(peer.ID) != network.Connected {
					if err := n.host.Connect(n.ctx, peer); err != nil {
						n.logger.Debug("Failed to connect to peer", "peer", peer.ID, "error", err)
					} else {
						n.logger.Info("Connected to new peer", "peer", peer.ID)
					}
				}
			}
		}
	}
}

// JoinTopic joins a PubSub topic
func (n *P2PNode) JoinTopic(topicName string) (*pubsub.Topic, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if topic, exists := n.topics[topicName]; exists {
		return topic, nil
	}

	topic, err := n.pubsub.Join(topicName)
	if err != nil {
		return nil, fmt.Errorf("failed to join topic %s: %w", topicName, err)
	}

	n.topics[topicName] = topic
	n.logger.Info("Joined topic", "topic", topicName)

	return topic, nil
}

// Subscribe subscribes to a topic
func (n *P2PNode) Subscribe(topicName string) (*pubsub.Subscription, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if sub, exists := n.subs[topicName]; exists {
		return sub, nil
	}

	topic, exists := n.topics[topicName]
	if !exists {
		return nil, fmt.Errorf("not joined to topic: %s", topicName)
	}

	sub, err := topic.Subscribe()
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to topic %s: %w", topicName, err)
	}

	n.subs[topicName] = sub
	n.logger.Info("Subscribed to topic", "topic", topicName)

	return sub, nil
}

// Publish publishes data to a topic
func (n *P2PNode) Publish(topicName string, data []byte) error {
	n.mu.RLock()
	topic, exists := n.topics[topicName]
	n.mu.RUnlock()

	if !exists {
		return fmt.Errorf("not joined to topic: %s", topicName)
	}

	if err := topic.Publish(n.ctx, data); err != nil {
		return fmt.Errorf("failed to publish to topic %s: %w", topicName, err)
	}

	n.logger.Debug("Published to topic", "topic", topicName, "size", len(data))
	return nil
}

// GetPeerID returns the node's peer ID
func (n *P2PNode) GetPeerID() peer.ID {
	return n.peerID
}

// GetHost returns the libp2p host
func (n *P2PNode) GetHost() host.Host {
	return n.host
}

// GetDHT returns the DHT
func (n *P2PNode) GetDHT() *dht.IpfsDHT {
	return n.dht
}

// GetConnectedPeers returns list of connected peers
func (n *P2PNode) GetConnectedPeers() []peer.ID {
	return n.host.Network().Peers()
}

// GetPeerCount returns the number of connected peers
func (n *P2PNode) GetPeerCount() int {
	return len(n.GetConnectedPeers())
}

// Close closes the P2P node
func (n *P2PNode) Close() error {
	n.logger.Info("Shutting down P2P node")

	n.cancel()

	n.mu.Lock()
	for name, sub := range n.subs {
		sub.Cancel()
		delete(n.subs, name)
	}
	for name, topic := range n.topics {
		topic.Close()
		delete(n.topics, name)
	}
	n.mu.Unlock()

	if err := n.dht.Close(); err != nil {
		n.logger.Warn("Failed to close DHT", "error", err)
	}

	if err := n.host.Close(); err != nil {
		return fmt.Errorf("failed to close host: %w", err)
	}

	n.logger.Info("P2P node closed successfully")
	return nil
}
