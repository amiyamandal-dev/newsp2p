package api

import (
	"github.com/gin-gonic/gin"

	"github.com/amiyamandal-dev/newsp2p/internal/api/handlers"
	"github.com/amiyamandal-dev/newsp2p/internal/api/middleware"
	"github.com/amiyamandal-dev/newsp2p/internal/auth"
	"github.com/amiyamandal-dev/newsp2p/internal/config"
	"github.com/amiyamandal-dev/newsp2p/pkg/logger"
)

// Router sets up the HTTP router with all routes and middleware
type Router struct {
	engine         *gin.Engine
	authHandler    *handlers.AuthHandler
	articleHandler *handlers.ArticleHandler
	feedHandler    *handlers.FeedHandler
	searchHandler  *handlers.SearchHandler
	healthHandler  *handlers.HealthHandler
	jwtManager     *auth.JWTManager
	cfg            *config.Config
	logger         *logger.Logger
}

// NewRouter creates a new router
func NewRouter(
	authHandler *handlers.AuthHandler,
	articleHandler *handlers.ArticleHandler,
	feedHandler *handlers.FeedHandler,
	searchHandler *handlers.SearchHandler,
	healthHandler *handlers.HealthHandler,
	jwtManager *auth.JWTManager,
	cfg *config.Config,
	logger *logger.Logger,
) *Router {
	return &Router{
		authHandler:    authHandler,
		articleHandler: articleHandler,
		feedHandler:    feedHandler,
		searchHandler:  searchHandler,
		healthHandler:  healthHandler,
		jwtManager:     jwtManager,
		cfg:            cfg,
		logger:         logger,
	}
}

// Setup configures all routes and middleware
func (r *Router) Setup() *gin.Engine {
	// Set Gin mode
	gin.SetMode(r.cfg.Server.Mode)

	// Create engine
	r.engine = gin.New()

	// Recovery middleware
	r.engine.Use(gin.Recovery())

	// CORS middleware
	r.engine.Use(middleware.CORSMiddleware(r.cfg.CORS.AllowedOrigins))

	// Logger middleware
	r.engine.Use(middleware.LoggerMiddleware(r.logger))

	// Rate limit middleware
	r.engine.Use(middleware.RateLimitMiddleware(
		r.cfg.RateLimit.RequestsPerMinute,
		r.cfg.RateLimit.Burst,
	))

	// Health check endpoints (no auth required)
	r.engine.GET("/health", r.healthHandler.Health)
	r.engine.GET("/health/ready", r.healthHandler.Readiness)
	r.engine.GET("/health/live", r.healthHandler.Liveness)

	// API v1 routes
	v1 := r.engine.Group("/api/v1")
	{
		// Auth routes (no auth required)
		auth := v1.Group("/auth")
		{
			auth.POST("/register", r.authHandler.Register)
			auth.POST("/login", r.authHandler.Login)
			auth.POST("/refresh", r.authHandler.RefreshToken)

			// Protected auth routes
			authProtected := auth.Group("")
			authProtected.Use(middleware.AuthMiddleware(r.jwtManager))
			{
				authProtected.GET("/me", r.authHandler.GetMe)
			}
		}

		// Article routes
		articles := v1.Group("/articles")
		{
			// Public article routes
			articles.GET("/:cid", r.articleHandler.GetByCID)
			articles.GET("", r.articleHandler.List)
			articles.POST("/:cid/verify", r.articleHandler.VerifySignature)

			// Protected article routes
			articlesProtected := articles.Group("")
			articlesProtected.Use(middleware.AuthMiddleware(r.jwtManager))
			{
				articlesProtected.POST("", r.articleHandler.Create)
				articlesProtected.PUT("/:id", r.articleHandler.Update)
				articlesProtected.DELETE("/:id", r.articleHandler.Delete)
			}
		}

		// Feed routes
		feeds := v1.Group("/feeds")
		{
			// Public feed routes
			feeds.GET("", r.feedHandler.List)
			feeds.GET("/:name", r.feedHandler.Get)
			feeds.GET("/:name/articles", r.feedHandler.GetArticles)

			// Protected feed routes
			feedsProtected := feeds.Group("")
			feedsProtected.Use(middleware.AuthMiddleware(r.jwtManager))
			{
				feedsProtected.POST("/:name/sync", r.feedHandler.TriggerSync)
			}
		}

		// Search routes (public)
		v1.GET("/search", r.searchHandler.Search)
	}

	return r.engine
}

// GetEngine returns the Gin engine
func (r *Router) GetEngine() *gin.Engine {
	if r.engine == nil {
		return r.Setup()
	}
	return r.engine
}
