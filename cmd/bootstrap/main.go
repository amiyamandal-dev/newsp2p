package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	connmgr "github.com/libp2p/go-libp2p/p2p/net/connmgr"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	dutil "github.com/libp2p/go-libp2p/p2p/discovery/util"
	"github.com/multiformats/go-multiaddr"
)

const (
	// Version info
	Version = "1.0.0"

	// Network identifiers
	NetworkName    = "Liberation News Network"
	Rendezvous     = "liberation-news-network"
	ProtocolPrefix = "/liberation"

	// Default ports
	DefaultP2PPort  = 4001
	DefaultHTTPPort = 8081

	// Connection limits
	MinConnections = 100
	MaxConnections = 600

	// Timeouts
	ConnectionTimeout = 30 * time.Second
	AdvertiseInterval = 30 * time.Second
	StatsInterval     = 60 * time.Second
)

// BootstrapServer is a production-ready P2P bootstrap node
type BootstrapServer struct {
	ctx       context.Context
	cancel    context.CancelFunc
	host      host.Host
	dht       *dht.IpfsDHT
	discovery *drouting.RoutingDiscovery

	// Configuration
	p2pPort  int
	httpPort int
	dataDir  string

	// Statistics (thread-safe)
	stats *ServerStats

	// HTTP server for graceful shutdown
	httpServer *http.Server
}

// ServerStats holds server statistics
type ServerStats struct {
	mu                sync.RWMutex
	StartTime         time.Time
	TotalConnections  int64
	ActiveConnections int64
	PeersDiscovered   int64
	MessagesRelayed   int64
}

func (s *ServerStats) IncrementConnections() {
	s.mu.Lock()
	s.TotalConnections++
	s.ActiveConnections++
	s.mu.Unlock()
}

func (s *ServerStats) DecrementConnections() {
	s.mu.Lock()
	if s.ActiveConnections > 0 {
		s.ActiveConnections--
	}
	s.mu.Unlock()
}

func (s *ServerStats) SetPeersDiscovered(count int64) {
	s.mu.Lock()
	s.PeersDiscovered = count
	s.mu.Unlock()
}

func (s *ServerStats) GetSnapshot() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return map[string]interface{}{
		"start_time":         s.StartTime,
		"uptime":             time.Since(s.StartTime).Round(time.Second).String(),
		"total_connections":  s.TotalConnections,
		"active_connections": s.ActiveConnections,
		"peers_discovered":   s.PeersDiscovered,
	}
}

func main() {
	printBanner()

	// Get configuration from environment (easy for non-technical users)
	config := loadConfig()

	// Create and start server
	server, err := NewBootstrapServer(config)
	if err != nil {
		logError("Failed to start server: %v", err)
		os.Exit(1)
	}

	// Print connection info for users
	server.printConnectionInfo()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logInfo("Shutting down gracefully...")
	server.Shutdown()
	logInfo("Server stopped. Goodbye!")
}

func printBanner() {
	fmt.Println()
	fmt.Println("╔═══════════════════════════════════════════════════════════╗")
	fmt.Println("║                                                           ║")
	fmt.Println("║          LIBERATION NEWS - BOOTSTRAP SERVER               ║")
	fmt.Println("║                                                           ║")
	fmt.Println("║    Decentralized News Network - Peer Discovery Node       ║")
	fmt.Println("║                                                           ║")
	fmt.Printf("║    Version: %-46s ║\n", Version)
	fmt.Println("║                                                           ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════╝")
	fmt.Println()
}

// Config holds server configuration
type Config struct {
	P2PPort  int
	HTTPPort int
	DataDir  string
}

func loadConfig() *Config {
	config := &Config{
		P2PPort:  getEnvInt("BOOTSTRAP_P2P_PORT", DefaultP2PPort),
		HTTPPort: getEnvInt("BOOTSTRAP_HTTP_PORT", DefaultHTTPPort),
		DataDir:  getEnvString("BOOTSTRAP_DATA_DIR", getDefaultDataDir()),
	}

	logInfo("Configuration loaded:")
	logInfo("  P2P Port:  %d", config.P2PPort)
	logInfo("  HTTP Port: %d", config.HTTPPort)
	logInfo("  Data Dir:  %s", config.DataDir)

	return config
}

func getDefaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "./data/bootstrap"
	}
	return filepath.Join(home, ".liberation-news", "bootstrap")
}

func getEnvString(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		var result int
		if _, err := fmt.Sscanf(val, "%d", &result); err == nil {
			return result
		}
	}
	return defaultVal
}

// NewBootstrapServer creates and starts a new bootstrap server
func NewBootstrapServer(config *Config) (*BootstrapServer, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Ensure data directory exists
	if err := os.MkdirAll(config.DataDir, 0755); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// Load or generate identity
	keyPath := filepath.Join(config.DataDir, "node.key")
	privKey, err := loadOrGenerateKey(keyPath)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to setup identity: %w", err)
	}

	// Create listen addresses
	listenAddrs := createListenAddrs(config.P2PPort)

	// Create connection manager
	cm, err := connmgr.NewConnManager(
		MinConnections,
		MaxConnections,
		connmgr.WithGracePeriod(time.Minute),
	)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create connection manager: %w", err)
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
		libp2p.EnableHolePunching(),
		libp2p.ConnectionManager(cm),
		// Enable AutoNAT for better connectivity
		libp2p.EnableNATService(),
	)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create libp2p host: %w", err)
	}

	// Create DHT in server mode
	kdht, err := dht.New(ctx, h,
		dht.Mode(dht.ModeServer),
		dht.ProtocolPrefix(ProtocolPrefix),
	)
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

	// Create discovery
	discovery := drouting.NewRoutingDiscovery(kdht)

	server := &BootstrapServer{
		ctx:       ctx,
		cancel:    cancel,
		host:      h,
		dht:       kdht,
		discovery: discovery,
		p2pPort:   config.P2PPort,
		httpPort:  config.HTTPPort,
		dataDir:   config.DataDir,
		stats: &ServerStats{
			StartTime: time.Now(),
		},
	}

	// Setup connection notifications
	h.Network().Notify(&network.NotifyBundle{
		ConnectedF: func(n network.Network, c network.Conn) {
			server.stats.IncrementConnections()
			logPeer("[+] CONNECTED", c.RemotePeer())
		},
		DisconnectedF: func(n network.Network, c network.Conn) {
			server.stats.DecrementConnections()
			logPeer("[-] DISCONNECTED", c.RemotePeer())
		},
	})

	// Start background services
	go server.advertiseLoop()
	go server.statsLoop()
	go server.startHTTPServer()

	// Save bootstrap info for easy sharing
	server.saveBootstrapInfo()

	return server, nil
}

func createListenAddrs(port int) []multiaddr.Multiaddr {
	addrs := []multiaddr.Multiaddr{}

	// IPv4 TCP
	if addr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", port)); err == nil {
		addrs = append(addrs, addr)
	}

	// IPv4 QUIC (faster, modern)
	if addr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic-v1", port)); err == nil {
		addrs = append(addrs, addr)
	}

	// IPv6 TCP
	if addr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip6/::/tcp/%d", port)); err == nil {
		addrs = append(addrs, addr)
	}

	// IPv6 QUIC
	if addr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip6/::/udp/%d/quic-v1", port)); err == nil {
		addrs = append(addrs, addr)
	}

	return addrs
}

func loadOrGenerateKey(path string) (crypto.PrivKey, error) {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}

	// Try to load existing key
	if data, err := os.ReadFile(path); err == nil {
		if privKey, err := crypto.UnmarshalPrivateKey(data); err == nil {
			logInfo("Loaded existing identity from %s", path)
			return privKey, nil
		}
	}

	// Generate new key
	privKey, _, err := crypto.GenerateKeyPair(crypto.Ed25519, -1)
	if err != nil {
		return nil, err
	}

	// Save key
	data, err := crypto.MarshalPrivateKey(privKey)
	if err != nil {
		return nil, err
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return nil, err
	}

	logInfo("Generated new identity, saved to %s", path)
	return privKey, nil
}

func (s *BootstrapServer) advertiseLoop() {
	// Initial advertisement
	dutil.Advertise(s.ctx, s.discovery, Rendezvous)
	logInfo("Advertising on network: %s", Rendezvous)

	ticker := time.NewTicker(AdvertiseInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			dutil.Advertise(s.ctx, s.discovery, Rendezvous)
		}
	}
}

func (s *BootstrapServer) statsLoop() {
	ticker := time.NewTicker(StatsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			peers := len(s.host.Network().Peers())
			routingSize := s.dht.RoutingTable().Size()
			s.stats.SetPeersDiscovered(int64(routingSize))

			logInfo("[STATS] Connected: %d | Routing Table: %d | Total Served: %d",
				peers, routingSize, s.stats.TotalConnections)
		}
	}
}

func (s *BootstrapServer) printConnectionInfo() {
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("  YOUR BOOTSTRAP SERVER IS RUNNING!")
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Printf("  Peer ID: %s\n", s.host.ID().String())
	fmt.Println()
	fmt.Println("  Share these addresses with other nodes:")
	fmt.Println("  ─────────────────────────────────────────────────────────")

	for _, addr := range s.host.Addrs() {
		fullAddr := fmt.Sprintf("%s/p2p/%s", addr.String(), s.host.ID().String())
		fmt.Printf("    %s\n", fullAddr)
	}

	fmt.Println()
	fmt.Printf("  HTTP Status: http://localhost:%d/status\n", s.httpPort)
	fmt.Printf("  Health Check: http://localhost:%d/health\n", s.httpPort)
	fmt.Println()
	fmt.Println("  Press Ctrl+C to stop the server")
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println()
}

func (s *BootstrapServer) saveBootstrapInfo() {
	// Save bootstrap info to a file for easy sharing
	info := map[string]interface{}{
		"peer_id":    s.host.ID().String(),
		"addresses":  []string{},
		"rendezvous": Rendezvous,
		"protocol":   ProtocolPrefix,
		"version":    Version,
		"created_at": time.Now().Format(time.RFC3339),
	}

	addrs := []string{}
	for _, addr := range s.host.Addrs() {
		addrs = append(addrs, fmt.Sprintf("%s/p2p/%s", addr.String(), s.host.ID().String()))
	}
	info["addresses"] = addrs

	data, _ := json.MarshalIndent(info, "", "  ")
	infoPath := filepath.Join(s.dataDir, "bootstrap-info.json")
	os.WriteFile(infoPath, data, 0644)

	logInfo("Bootstrap info saved to: %s", infoPath)
}

func (s *BootstrapServer) startHTTPServer() {
	mux := http.NewServeMux()

	// Health check - simple endpoint for monitoring
	mux.HandleFunc("/health", s.handleHealth)

	// Status - detailed server status
	mux.HandleFunc("/status", s.handleStatus)

	// Peers - list of connected peers
	mux.HandleFunc("/peers", s.handlePeers)

	// Bootstrap info - for nodes to auto-configure
	mux.HandleFunc("/bootstrap", s.handleBootstrapInfo)

	// Connect info - user-friendly connection instructions
	mux.HandleFunc("/", s.handleHome)

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.httpPort),
		Handler:      corsMiddleware(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	logInfo("HTTP API running on port %d", s.httpPort)

	if err := s.httpServer.ListenAndServe(); err != http.ErrServerClosed {
		logError("HTTP server error: %v", err)
	}
}

func (s *BootstrapServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "healthy",
		"peers":     len(s.host.Network().Peers()),
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

func (s *BootstrapServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	addrs := []string{}
	for _, addr := range s.host.Addrs() {
		addrs = append(addrs, fmt.Sprintf("%s/p2p/%s", addr.String(), s.host.ID().String()))
	}

	status := map[string]interface{}{
		"status":     "running",
		"version":    Version,
		"network":    NetworkName,
		"peer_id":    s.host.ID().String(),
		"addresses":  addrs,
		"rendezvous": Rendezvous,
		"stats":      s.stats.GetSnapshot(),
		"system": map[string]interface{}{
			"go_version": runtime.Version(),
			"os":         runtime.GOOS,
			"arch":       runtime.GOARCH,
			"goroutines": runtime.NumGoroutine(),
		},
	}

	json.NewEncoder(w).Encode(status)
}

func (s *BootstrapServer) handlePeers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	peers := s.host.Network().Peers()
	peerList := make([]map[string]interface{}, 0, len(peers))

	for _, pid := range peers {
		conns := s.host.Network().ConnsToPeer(pid)
		addrs := []string{}
		for _, conn := range conns {
			addrs = append(addrs, conn.RemoteMultiaddr().String())
		}

		peerList = append(peerList, map[string]interface{}{
			"peer_id":   pid.String(),
			"addresses": addrs,
		})
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"count": len(peerList),
		"peers": peerList,
	})
}

func (s *BootstrapServer) handleBootstrapInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	addrs := []string{}
	for _, addr := range s.host.Addrs() {
		addrs = append(addrs, fmt.Sprintf("%s/p2p/%s", addr.String(), s.host.ID().String()))
	}

	// Return info that nodes can use to auto-connect
	info := map[string]interface{}{
		"peer_id":    s.host.ID().String(),
		"addresses":  addrs,
		"rendezvous": Rendezvous,
		"protocol":   ProtocolPrefix,
		"version":    Version,
	}

	json.NewEncoder(w).Encode(info)
}

func (s *BootstrapServer) handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html")

	addrs := []string{}
	for _, addr := range s.host.Addrs() {
		addrs = append(addrs, fmt.Sprintf("%s/p2p/%s", addr.String(), s.host.ID().String()))
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Liberation News - Bootstrap Server</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
               max-width: 800px; margin: 50px auto; padding: 20px; background: #1a1a2e; color: #eee; }
        h1 { color: #00ff88; border-bottom: 2px solid #00ff88; padding-bottom: 10px; }
        h2 { color: #00ff88; margin-top: 30px; }
        .status { background: #16213e; padding: 20px; border-radius: 8px; margin: 20px 0; }
        .status.online { border-left: 4px solid #00ff88; }
        .peer-id { font-family: monospace; background: #0f0f1a; padding: 10px; border-radius: 4px;
                   word-break: break-all; margin: 10px 0; }
        .address { font-family: monospace; background: #0f0f1a; padding: 8px; margin: 5px 0;
                   border-radius: 4px; font-size: 12px; word-break: break-all; }
        .copy-btn { background: #00ff88; color: #1a1a2e; border: none; padding: 5px 10px;
                    border-radius: 4px; cursor: pointer; margin-left: 10px; }
        .copy-btn:hover { background: #00cc6a; }
        .stats { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 15px; }
        .stat-box { background: #16213e; padding: 15px; border-radius: 8px; text-align: center; }
        .stat-value { font-size: 24px; color: #00ff88; font-weight: bold; }
        .stat-label { font-size: 12px; color: #888; margin-top: 5px; }
        a { color: #00ff88; }
    </style>
</head>
<body>
    <h1>Liberation News Bootstrap Server</h1>

    <div class="status online">
        <strong>Status:</strong> Online and accepting connections
    </div>

    <h2>Server Identity</h2>
    <div class="peer-id">%s</div>

    <h2>Connection Addresses</h2>
    <p>Add one of these to your config to connect:</p>
    %s

    <h2>Quick Stats</h2>
    <div class="stats">
        <div class="stat-box">
            <div class="stat-value" id="peers">%d</div>
            <div class="stat-label">Connected Peers</div>
        </div>
        <div class="stat-box">
            <div class="stat-value" id="routing">%d</div>
            <div class="stat-label">Known Peers</div>
        </div>
    </div>

    <h2>API Endpoints</h2>
    <ul>
        <li><a href="/health">/health</a> - Health check</li>
        <li><a href="/status">/status</a> - Detailed status (JSON)</li>
        <li><a href="/peers">/peers</a> - Connected peers list</li>
        <li><a href="/bootstrap">/bootstrap</a> - Bootstrap info for auto-connect</li>
    </ul>

    <script>
        // Auto-refresh stats
        setInterval(async () => {
            try {
                const resp = await fetch('/status');
                const data = await resp.json();
                document.getElementById('peers').textContent = data.stats.active_connections;
                document.getElementById('routing').textContent = data.stats.peers_discovered;
            } catch(e) {}
        }, 5000);
    </script>
</body>
</html>`,
		s.host.ID().String(),
		formatAddressesHTML(addrs),
		len(s.host.Network().Peers()),
		s.dht.RoutingTable().Size(),
	)

	w.Write([]byte(html))
}

func formatAddressesHTML(addrs []string) string {
	html := ""
	for _, addr := range addrs {
		html += fmt.Sprintf(`<div class="address">%s</div>`, addr)
	}
	return html
}

func (s *BootstrapServer) Shutdown() {
	logInfo("Stopping services...")

	// Stop HTTP server
	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.httpServer.Shutdown(ctx)
	}

	// Cancel context (stops background goroutines)
	s.cancel()

	// Close DHT
	if s.dht != nil {
		s.dht.Close()
	}

	// Close host
	if s.host != nil {
		s.host.Close()
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Logging helpers
func logInfo(format string, args ...interface{}) {
	timestamp := time.Now().Format("15:04:05")
	fmt.Printf("[%s] INFO  %s\n", timestamp, fmt.Sprintf(format, args...))
}

func logError(format string, args ...interface{}) {
	timestamp := time.Now().Format("15:04:05")
	fmt.Printf("[%s] ERROR %s\n", timestamp, fmt.Sprintf(format, args...))
}

func logPeer(action string, pid peer.ID) {
	timestamp := time.Now().Format("15:04:05")
	shortID := pid.String()
	if len(shortID) > 16 {
		shortID = shortID[:16] + "..."
	}
	fmt.Printf("[%s] %s %s\n", timestamp, action, shortID)
}
