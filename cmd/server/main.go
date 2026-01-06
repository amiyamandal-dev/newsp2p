package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	shell "github.com/ipfs/go-ipfs-api"

	"github.com/amiyamandal-dev/newsp2p/internal/api"
	"github.com/amiyamandal-dev/newsp2p/internal/api/handlers"
	"github.com/amiyamandal-dev/newsp2p/internal/auth"
	"github.com/amiyamandal-dev/newsp2p/internal/config"
	"github.com/amiyamandal-dev/newsp2p/internal/ipfs"
	"github.com/amiyamandal-dev/newsp2p/internal/repository/sqlite"
	"github.com/amiyamandal-dev/newsp2p/internal/search"
	"github.com/amiyamandal-dev/newsp2p/internal/service"
	"github.com/amiyamandal-dev/newsp2p/pkg/logger"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	log, err := logger.New(cfg.Logging.Level, cfg.Logging.Format)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	log.Info("Starting distributed news platform server",
		"version", "1.0.0",
		"mode", cfg.Server.Mode,
	)

	// Initialize database
	db, err := sqlite.New(
		cfg.Database.Path,
		cfg.Database.MaxOpenConns,
		cfg.Database.MaxIdleConns,
	)
	if err != nil {
		log.Error("Failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	log.Info("Database initialized", "path", cfg.Database.Path)

	// Initialize IPFS client
	ipfsClient := ipfs.NewClient(
		cfg.IPFS.APIEndpoint,
		cfg.IPFS.Timeout,
		cfg.IPFS.PinArticles,
		log,
	)

	// Check IPFS connectivity
	ctx := context.Background()
	if !ipfsClient.IsHealthy(ctx) {
		log.Warn("IPFS node is not reachable", "endpoint", cfg.IPFS.APIEndpoint)
	} else {
		nodeID, _ := ipfsClient.GetID(ctx)
		log.Info("Connected to IPFS", "endpoint", cfg.IPFS.APIEndpoint, "node_id", nodeID)
	}

	// Initialize IPNS manager
	ipfsShell := shell.NewShell(cfg.IPFS.APIEndpoint)
	ipnsManager := ipfs.NewIPNSManager(ipfsShell, log)

	// Initialize search index
	searchIndex := search.NewBleveIndex(log)
	if err := searchIndex.Open(cfg.Search.IndexPath); err != nil {
		log.Error("Failed to open search index", "error", err)
		os.Exit(1)
	}
	defer searchIndex.Close()

	count, _ := searchIndex.Count()
	log.Info("Search index opened", "path", cfg.Search.IndexPath, "document_count", count)

	// Initialize repositories
	articleRepo := sqlite.NewArticleRepo(db)
	userRepo := sqlite.NewUserRepo(db)
	feedRepo := sqlite.NewFeedRepo(db)

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
		articleSigner,
		searchService,
		log,
	)
	feedService := service.NewFeedService(feedRepo, articleRepo, ipnsManager, log)
	syncService := service.NewSyncService(feedRepo, articleRepo, ipfsClient, ipnsManager, log)

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(userService, log)
	articleHandler := handlers.NewArticleHandler(articleService, log)
	feedHandler := handlers.NewFeedHandler(feedService, syncService, log)
	searchHandler := handlers.NewSearchHandler(searchService, log)
	healthHandler := handlers.NewHealthHandler(db, ipfsClient, searchIndex, log)

	// Initialize router
	router := api.NewRouter(
		authHandler,
		articleHandler,
		feedHandler,
		searchHandler,
		healthHandler,
		jwtManager,
		cfg,
		log,
	)

	// Setup routes
	engine := router.Setup()

	// Create HTTP server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      engine,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Start background sync service
	go syncService.Start(ctx, 15) // Sync every 15 minutes

	// Start server in goroutine
	go func() {
		log.Info("HTTP server starting", "address", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("Server failed to start", "error", err)
			os.Exit(1)
		}
	}()

	log.Info("Server started successfully", "address", addr)

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

	log.Info("Server stopped gracefully")
}
