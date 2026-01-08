package api

import (
	"github.com/gin-gonic/gin"

	"github.com/amiyamandal-dev/newsp2p/internal/api/handlers"
	"github.com/amiyamandal-dev/newsp2p/internal/api/middleware"
	"github.com/amiyamandal-dev/newsp2p/internal/auth"
	"github.com/amiyamandal-dev/newsp2p/internal/config"
	"github.com/amiyamandal-dev/newsp2p/internal/service"
	"github.com/amiyamandal-dev/newsp2p/internal/web"
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
	webHandler     *web.WebHandler
	jwtManager     *auth.JWTManager
	userService    *service.UserService
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
	webHandler *web.WebHandler,
	jwtManager *auth.JWTManager,
	userService *service.UserService,
	cfg *config.Config,
	logger *logger.Logger,
) *Router {
	return &Router{
		authHandler:    authHandler,
		articleHandler: articleHandler,
		feedHandler:    feedHandler,
		searchHandler:  searchHandler,
		healthHandler:  healthHandler,
		webHandler:     webHandler,
		jwtManager:     jwtManager,
		userService:    userService,
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

	// Recovery middleware (global)
	r.engine.Use(gin.Recovery())

	// CORS middleware (global)
	r.engine.Use(middleware.CORSMiddleware(r.cfg.CORS.AllowedOrigins))

	// Logger middleware (global)
	r.engine.Use(middleware.LoggerMiddleware(r.logger))

	// Health check endpoints (no rate limiting, no auth)
	r.engine.GET("/health", r.healthHandler.Health)
	r.engine.GET("/health/ready", r.healthHandler.Readiness)
	r.engine.GET("/health/live", r.healthHandler.Liveness)

	// Web UI routes (if webHandler is available)
	if r.webHandler != nil {
		// Apply web auth middleware
		r.engine.Use(web.AuthMiddleware(r.jwtManager, r.userService))

		r.engine.GET("/", r.webHandler.HomePage)
		r.engine.GET("/explore", r.webHandler.ExplorePage)
		r.engine.GET("/login", r.webHandler.LoginPage)
		r.engine.POST("/login", r.webHandler.WebLogin)
		r.engine.GET("/logout", r.webHandler.WebLogout)
		r.engine.GET("/register", r.webHandler.RegisterPage)
		r.engine.POST("/register", r.webHandler.WebRegister)
		r.engine.GET("/create", r.webHandler.CreateArticlePage)
		r.engine.POST("/create", r.webHandler.WebCreateArticle)
		r.engine.GET("/article/:cid", r.webHandler.ArticlePage)
		r.engine.GET("/network", r.webHandler.NetworkPage)
	}

	// API v1 routes (with rate limiting)
	v1 := r.engine.Group("/api/v1")
	v1.Use(middleware.RateLimitMiddleware(
		r.cfg.RateLimit.RequestsPerMinute,
		r.cfg.RateLimit.Burst,
	))
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
