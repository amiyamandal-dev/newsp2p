package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	shell "github.com/ipfs/go-ipfs-api"

	"github.com/amiyamandal-dev/newsp2p/internal/api"
	"github.com/amiyamandal-dev/newsp2p/internal/api/handlers"
	"github.com/amiyamandal-dev/newsp2p/internal/auth"
	"github.com/amiyamandal-dev/newsp2p/internal/config"
	"github.com/amiyamandal-dev/newsp2p/internal/ipfs"
	"github.com/amiyamandal-dev/newsp2p/internal/p2p"
	"github.com/amiyamandal-dev/newsp2p/internal/repository/badger"
	"github.com/amiyamandal-dev/newsp2p/internal/search"
	"github.com/amiyamandal-dev/newsp2p/internal/service"
	"github.com/amiyamandal-dev/newsp2p/internal/web"
	"github.com/amiyamandal-dev/newsp2p/pkg/logger"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to load config: %v\n", err)
		fmt.Fprintf(os.Stderr, "\n💡 Tip: Make sure to set NEWS_AUTH_JWT_SECRET environment variable\n")
		fmt.Fprintf(os.Stderr, "   Example: export NEWS_AUTH_JWT_SECRET=$(openssl rand -base64 32)\n\n")
		os.Exit(1)
	}

	// Initialize logger
	log, err := logger.New(cfg.Logging.Level, cfg.Logging.Format)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	log.Info("🚀 Starting distributed news platform server",
		"version", "1.0.0",
		"mode", cfg.Server.Mode,
	)

	// Ensure required directories exist
	if err := ensureDirectories(cfg, log); err != nil {
		log.Error("Failed to create required directories", "error", err)
		os.Exit(1)
	}

	// Initialize database (BadgerDB)
	db, err := badger.New(cfg.Database.Path)
	if err != nil {
		log.Error("Failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	log.Info("✅ Database initialized (BadgerDB)", "path", cfg.Database.Path)

	// Initialize IPFS client
	ipfsClient := ipfs.NewClient(
		cfg.IPFS.APIEndpoint,
		cfg.IPFS.Timeout,
		cfg.IPFS.PinArticles,
		log,
	)

	// Check IPFS connectivity (non-blocking)
	ctx := context.Background()
	ipfsHealthy := false
	if ipfsClient.IsHealthy(ctx) {
		nodeID, _ := ipfsClient.GetID(ctx)
		log.Info("✅ Connected to IPFS", "endpoint", cfg.IPFS.APIEndpoint, "node_id", nodeID)
		ipfsHealthy = true
	} else {
		log.Warn("⚠️  IPFS node is not reachable - some features will be limited",
			"endpoint", cfg.IPFS.APIEndpoint,
		)
		log.Warn("💡 To start IPFS: ipfs daemon")
	}

	// Initialize IPNS manager
	ipfsShell := shell.NewShell(cfg.IPFS.APIEndpoint)
	ipnsManager := ipfs.NewIPNSManager(ipfsShell, log)

	// Initialize P2P node (if enabled)
	var p2pNode *p2p.P2PNode
	var broadcaster *p2p.Broadcaster
	var reputationSys *p2p.ReputationSystem

	if cfg.P2P.Enabled {
		p2pCfg := &p2p.Config{
			ListenAddrs:    cfg.P2P.ListenAddrs,
			BootstrapPeers: cfg.P2P.BootstrapPeers,
			Rendezvous:     cfg.P2P.Rendezvous,
		}

		var err error
		p2pNode, err = p2p.NewP2PNode(ctx, p2pCfg, log)
		if err != nil {
			log.Warn("⚠️  Failed to start P2P node - continuing without P2P", "error", err)
		} else {
			log.Info("✅ P2P node started", "peer_id", p2pNode.GetPeerID().String())

			// Initialize broadcaster
			broadcaster = p2p.NewBroadcaster(p2pNode, log)
			if err := broadcaster.Start(); err != nil {
				log.Warn("Failed to start broadcaster", "error", err)
			} else {
				log.Info("✅ P2P broadcaster started")
			}

			// Initialize reputation system
			reputationSys = p2p.NewReputationSystem(log)
			log.Info("✅ Reputation system initialized")

			// Use reputation system (prevent unused variable warning)
			_ = reputationSys

			defer func() {
				if broadcaster != nil {
					broadcaster.Stop()
				}
				if p2pNode != nil {
					p2pNode.Close()
				}
			}()
		}
	} else {
		log.Info("💤 P2P mode disabled - running in centralized mode")
	}

	// Initialize search index
	searchIndex := search.NewBleveIndex(log)
	if err := searchIndex.Open(cfg.Search.IndexPath); err != nil {
		log.Error("Failed to open search index", "error", err)
		os.Exit(1)
	}
	defer searchIndex.Close()

	count, _ := searchIndex.Count()
	log.Info("✅ Search index opened", "path", cfg.Search.IndexPath, "document_count", count)

	// Initialize repositories (BadgerDB)
	articleRepo := badger.NewArticleRepo(db)
	userRepo := badger.NewUserRepo(db)
	feedRepo := badger.NewFeedRepo(db)

	// Initialize JWT manager
	jwtManager := auth.NewJWTManager(
		cfg.Auth.JWTSecret,
		cfg.Auth.JWTExpiry,
		cfg.Auth.RefreshTokenExpiry,
	)

	// Initialize article signer
	articleSigner := auth.NewArticleSigner()

	// Initialize services
	searchService := service.NewSearchService(searchIndex, articleRepo, log)
	userService := service.NewUserService(userRepo, jwtManager, cfg.Auth.BcryptCost, log)
	articleService := service.NewArticleService(
		articleRepo,
		userRepo,
		ipfsClient,
		broadcaster,
		articleSigner,
		searchService,
		log,
	)

	// Register P2P handlers
	if broadcaster != nil {
		broadcaster.OnArticle(func(msg *p2p.ArticleMessage) error {
			if msg.Article != nil {
				return articleService.HandleIncomingArticle(msg.Article)
			}
			return nil
		})
	}

	feedService := service.NewFeedService(feedRepo, articleRepo, ipnsManager, log)
	syncService := service.NewSyncService(feedRepo, articleRepo, ipfsClient, ipnsManager, log)

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(userService, log)
	articleHandler := handlers.NewArticleHandler(articleService, log)
	feedHandler := handlers.NewFeedHandler(feedService, syncService, log)
	searchHandler := handlers.NewSearchHandler(searchService, log)
	healthHandler := handlers.NewHealthHandler(db, ipfsClient, searchIndex, log)

	// Initialize web handler
	webHandler := web.NewWebHandler(articleService, userService, searchService, jwtManager, db, p2pNode, log)

	// Initialize router
	router := api.NewRouter(
		authHandler,
		articleHandler,
		feedHandler,
		searchHandler,
		healthHandler,
		webHandler,
		jwtManager,
		userService,
		cfg,
		log,
	)

	// Setup routes
	engine := router.Setup()

	// Define address
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)

	// Create HTTP server
	server := &http.Server{
		Addr:         addr,
		Handler:      engine,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Auto-Login for P2P Node Owner
	if p2pNode != nil {
		nodeUser, err := userService.EnsureNodeUser(
			context.Background(),
			p2pNode.GetPeerID().String(),
			p2pNode.GetHost().Peerstore().PubKey(p2pNode.GetPeerID()),
		)
		if err != nil {
			log.Warn("Failed to ensure node user", "error", err)
		} else {
			// Generate long-lived token for the node owner
			tokens, err := jwtManager.GenerateTokenPair(nodeUser.ID, nodeUser.Username, nodeUser.Email)
			if err == nil {
				log.Info("🔑 Node Identity Active", "username", nodeUser.Username, "peer_id", nodeUser.ID)
				log.Info("🔓 Auto-Login Link: http://" + addr + "/login?token=" + tokens.AccessToken)
			}
		}
	}

	// Start background sync service
	go syncService.Start(ctx, 15) // Sync every 15 minutes

	// Start server in goroutine
	go func() {
		log.Info("🌐 HTTP server starting", "address", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("❌ Server failed to start", "error", err)
			os.Exit(1)
		}
	}()

	log.Info("✅ Server started successfully", "address", addr)
	log.Info("📝 API documentation available at /api/v1")
	if ipfsHealthy {
		log.Info("🌍 IPFS integration: ACTIVE")
	} else {
		log.Info("💤 IPFS integration: INACTIVE (local mode)")
	}
	if p2pNode != nil {
		log.Info("🔗 P2P network: ACTIVE", "connected_peers", p2pNode.GetPeerCount())
	} else {
		log.Info("💤 P2P network: INACTIVE (centralized mode)")
	}
	log.Info("Press Ctrl+C to stop")

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")

	// Stop background services
	syncService.Stop()

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("Server forced to shutdown", "error", err)
	}

	log.Info("✅ Server stopped gracefully")
}

// ensureDirectories creates required directories if they don't exist
func ensureDirectories(cfg *config.Config, log *logger.Logger) error {
	dirs := []struct {
		path string
		name string
	}{
		{filepath.Dir(cfg.Database.Path), "database directory"},
		{filepath.Dir(cfg.Search.IndexPath), "search index directory"},
	}

	for _, dir := range dirs {
		if dir.path == "" || dir.path == "." {
			continue
		}

		if err := os.MkdirAll(dir.path, 0755); err != nil {
			return fmt.Errorf("failed to create %s (%s): %w", dir.name, dir.path, err)
		}

		log.Debug("Directory ensured", "path", dir.path, "type", dir.name)
	}

	return nil
}
