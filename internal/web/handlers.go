package web

import (
	"bytes"
	"html/template"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"

	"github.com/amiyamandal-dev/newsp2p/internal/auth"
	"github.com/amiyamandal-dev/newsp2p/internal/domain"
	"github.com/amiyamandal-dev/newsp2p/internal/ipfs"
	"github.com/amiyamandal-dev/newsp2p/internal/p2p"
	"github.com/amiyamandal-dev/newsp2p/internal/repository/badger"
	"github.com/amiyamandal-dev/newsp2p/internal/search"
	"github.com/amiyamandal-dev/newsp2p/internal/service"
	"github.com/amiyamandal-dev/newsp2p/pkg/logger"
)

// WebHandler handles web UI requests
type WebHandler struct {
	articleService *service.ArticleService
	userService    *service.UserService
	searchService  *service.SearchService
	jwtManager     *auth.JWTManager
	db             *badger.DB
	p2pNode        *p2p.P2PNode
	ipfsClient     *ipfs.Client
	logger         *logger.Logger
	templates      map[string]*template.Template
}

// NewWebHandler creates a new web handler
func NewWebHandler(
	articleService *service.ArticleService,
	userService *service.UserService,
	searchService *service.SearchService,
	jwtManager *auth.JWTManager,
	db *badger.DB,
	p2pNode *p2p.P2PNode,
	ipfsClient *ipfs.Client,
	log *logger.Logger,
) *WebHandler {
	// Create sanitizer policy for markdown HTML output
	sanitizer := bluemonday.UGCPolicy()

	// Load templates with custom functions
	funcMap := template.FuncMap{
		"truncate": func(length int, s string) string {
			if len(s) <= length {
				return s
			}
			return s[:length] + "..."
		},
		"upper": strings.ToUpper,
		"markdown": func(s string) template.HTML {
			var buf bytes.Buffer
			if err := goldmark.Convert([]byte(s), &buf); err != nil {
				return template.HTML(template.HTMLEscapeString(s))
			}
			// Sanitize the HTML output to prevent XSS
			return template.HTML(sanitizer.Sanitize(buf.String()))
		},
		"safeHTML": func(s string) template.HTML {
			// Sanitize any raw HTML content
			return template.HTML(sanitizer.Sanitize(s))
		},
		"firstChar": func(s string) string {
			if len(s) == 0 {
				return "?"
			}
			return strings.ToUpper(string([]rune(s)[0]))
		},
		"urlquery": template.URLQueryEscaper,
	}

	// Create template map - parse each page with base layout
	templates := make(map[string]*template.Template)

	baseLayout := "web/templates/layouts/base.html"
	articleListComponent := "web/templates/components/article_list.html"
	pages := map[string]string{
		"home":     "web/templates/pages/home.html",
		"explore":  "web/templates/pages/explore.html",
		"login":    "web/templates/pages/login.html",
		"register": "web/templates/pages/register.html",
		"create":   "web/templates/pages/create.html",
		"article":  "web/templates/pages/article.html",
		"network":  "web/templates/pages/network.html",
	}

	for name, pagePath := range pages {
		var tmpl *template.Template
		if name == "explore" || name == "home" {
			// Include article list component for pages that need it
			tmpl = template.Must(
				template.New(name).Funcs(funcMap).ParseFiles(baseLayout, pagePath, articleListComponent),
			)
		} else {
			tmpl = template.Must(
				template.New(name).Funcs(funcMap).ParseFiles(baseLayout, pagePath),
			)
		}
		templates[name] = tmpl
	}

	return &WebHandler{
		articleService: articleService,
		userService:    userService,
		searchService:  searchService,
		jwtManager:     jwtManager,
		db:             db,
		p2pNode:        p2pNode,
		ipfsClient:     ipfsClient,
		logger:         log.WithComponent("web-handler"),
		templates:      templates,
	}
}

// HomePage renders the home page
func (h *WebHandler) HomePage(c *gin.Context) {
	ctx := c.Request.Context()
	user := GetUser(c)

	// Get recent articles
	articles, total, err := h.articleService.List(ctx, &domain.ArticleListFilter{
		Page:  1,
		Limit: 10,
	})
	if err != nil {
		h.logger.Error("Failed to get articles", "error", err)
		articles = []*domain.Article{}
	}

	// Get stats
	var peerCount int
	var peerID string
	var p2pEnabled bool
	if h.p2pNode != nil {
		peerCount = h.p2pNode.GetPeerCount()
		peerID = h.p2pNode.GetPeerID().String()
		p2pEnabled = true
	}

	data := gin.H{
		"Title":    "Home",
		"User":     user,
		"Articles": articles,
		"Stats": gin.H{
			"TotalArticles": total,
			"ActivePeers":   peerCount,
			"IPFSOnline":    h.ipfsClient != nil && h.ipfsClient.IsHealthy(ctx),
			"P2PEnabled":    p2pEnabled,
			"PeerID":        peerID,
		},
		"PeerCount": peerCount,
	}

	// Render template
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := h.templates["home"].ExecuteTemplate(c.Writer, "base.html", data); err != nil {
		h.logger.Error("Template error", "error", err)
		c.String(http.StatusInternalServerError, "Template error")
	}
}

// ArticlePage renders a single article
func (h *WebHandler) ArticlePage(c *gin.Context) {
	cid := c.Param("cid")
	ctx := c.Request.Context()
	user := GetUser(c)

	article, err := h.articleService.GetByCID(ctx, cid)
	if err != nil {
		c.String(http.StatusNotFound, "Article not found")
		return
	}

	data := gin.H{
		"Title":     article.Title,
		"User":      user,
		"Article":   article,
		"PeerCount": h.getPeerCount(),
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := h.templates["article"].ExecuteTemplate(c.Writer, "base.html", data); err != nil {
		h.logger.Error("Template error", "error", err)
		c.String(http.StatusInternalServerError, "Template error")
	}
}

// ExplorePage renders the explore/search page
func (h *WebHandler) ExplorePage(c *gin.Context) {
	ctx := c.Request.Context()
	user := GetUser(c)

	// Get all articles for exploration
	articles, _, err := h.articleService.List(ctx, &domain.ArticleListFilter{
		Page:  1,
		Limit: 20,
	})
	if err != nil {
		h.logger.Error("Failed to get articles", "error", err)
		articles = []*domain.Article{}
	}

	data := gin.H{
		"Title":     "Explore",
		"User":      user,
		"Articles":  articles,
		"PeerCount": h.getPeerCount(),
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := h.templates["explore"].ExecuteTemplate(c.Writer, "base.html", data); err != nil {
		h.logger.Error("Template error", "error", err)
		c.String(http.StatusInternalServerError, "Template error")
	}
}

// LoginPage renders the login page
func (h *WebHandler) LoginPage(c *gin.Context) {
	if GetUser(c) != nil {
		c.Redirect(http.StatusSeeOther, "/")
		return
	}

	// Auto-login via query param (e.g. from console startup)
	token := c.Query("token")
	if token != "" {
		// Validate token
		_, err := h.jwtManager.ValidateToken(token)
		if err == nil {
			SetSecureCookie(c, CookieAccessToken, token, 3600*24)
			c.Redirect(http.StatusSeeOther, "/")
			return
		}
	}

	data := gin.H{
		"Title":     "Login",
		"PeerCount": h.getPeerCount(),
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := h.templates["login"].ExecuteTemplate(c.Writer, "base.html", data); err != nil {
		h.logger.Error("Template error", "error", err)
		c.String(http.StatusInternalServerError, "Template error")
	}
}

// RegisterPage renders the registration page
func (h *WebHandler) RegisterPage(c *gin.Context) {
	if GetUser(c) != nil {
		c.Redirect(http.StatusSeeOther, "/")
		return
	}

	data := gin.H{
		"Title":     "Register",
		"PeerCount": h.getPeerCount(),
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := h.templates["register"].ExecuteTemplate(c.Writer, "base.html", data); err != nil {
		h.logger.Error("Template error", "error", err)
		c.String(http.StatusInternalServerError, "Template error")
	}
}

// WebLogin handles login form submission
func (h *WebHandler) WebLogin(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")

	req := &domain.UserLoginRequest{
		Username: username,
		Password: password,
	}

	loginResp, err := h.userService.Login(c.Request.Context(), req)
	if err != nil {
		data := gin.H{
			"Title":     "Login",
			"Error":     "Invalid username or password",
			"PeerCount": h.getPeerCount(),
		}
		c.Header("Content-Type", "text/html; charset=utf-8")
		if err := h.templates["login"].ExecuteTemplate(c.Writer, "base.html", data); err != nil {
			h.logger.Error("Template error", "error", err)
			c.String(http.StatusInternalServerError, "Template error")
		}
		return
	}

	// Set cookie with secure attributes (24 hours)
	SetSecureCookie(c, CookieAccessToken, loginResp.Tokens.AccessToken, 3600*24)

	c.Redirect(http.StatusSeeOther, "/")
}

// WebRegister handles registration form submission
func (h *WebHandler) WebRegister(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")

	req := &domain.UserRegisterRequest{
		Username: username,
		Email:    "", // Email is no longer used/collected
		Password: password,
	}

	_, err := h.userService.Register(c.Request.Context(), req)
	if err != nil {
		errorMsg := "Registration failed"
		if err == domain.ErrUserAlreadyExists {
			errorMsg = "Username or email already exists"
		}

		data := gin.H{
			"Title":     "Register",
			"Error":     errorMsg,
			"PeerCount": h.getPeerCount(),
		}
		c.Header("Content-Type", "text/html; charset=utf-8")
		if err := h.templates["register"].ExecuteTemplate(c.Writer, "base.html", data); err != nil {
			h.logger.Error("Template error", "error", err)
			c.String(http.StatusInternalServerError, "Template error")
		}
		return
	}

	// Auto login after register
	loginReq := &domain.UserLoginRequest{
		Username: username,
		Password: password,
	}
	loginResp, err := h.userService.Login(c.Request.Context(), loginReq)
	if err == nil {
		SetSecureCookie(c, CookieAccessToken, loginResp.Tokens.AccessToken, 3600*24)
	}

	c.Redirect(http.StatusSeeOther, "/")
}

// WebLogout handles logout
func (h *WebHandler) WebLogout(c *gin.Context) {
	ClearSecureCookie(c, CookieAccessToken)
	c.Redirect(http.StatusSeeOther, "/login")
}

// CreateArticlePage renders the article creation page
func (h *WebHandler) CreateArticlePage(c *gin.Context) {
	user := GetUser(c)
	if user == nil {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}

	data := gin.H{
		"Title":     "Write Article",
		"User":      user,
		"PeerCount": h.getPeerCount(),
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := h.templates["create"].ExecuteTemplate(c.Writer, "base.html", data); err != nil {
		h.logger.Error("Template error", "error", err)
		c.String(http.StatusInternalServerError, "Template error")
	}
}

// WebCreateArticle handles article creation form submission
func (h *WebHandler) WebCreateArticle(c *gin.Context) {
	user := GetUser(c)
	if user == nil {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}

	title := c.PostForm("title")
	body := c.PostForm("body")
	category := c.PostForm("category")
	tags := c.PostForm("tags")

	tagList := strings.Split(tags, ",")
	for i := range tagList {
		tagList[i] = strings.TrimSpace(tagList[i])
	}
	// Filter empty tags
	var cleanTags []string
	for _, t := range tagList {
		if t != "" {
			cleanTags = append(cleanTags, t)
		}
	}

	req := &domain.ArticleCreateRequest{
		Title:    title,
		Body:     body,
		Category: category,
		Tags:     cleanTags,
	}

	article, err := h.articleService.Create(c.Request.Context(), req, user.ID, h.getOriginIdentifier(c))
	if err != nil {
		h.logger.Error("Failed to create article", "error", err)
		data := gin.H{
			"Title":     "Write Article",
			"User":      user,
			"PeerCount": h.getPeerCount(),
			"Error":     "Failed to create article. Please try again.",
			"Form": gin.H{
				"Title":    title,
				"Body":     body,
				"Category": category,
				"Tags":     tags,
			},
		}
		c.Header("Content-Type", "text/html; charset=utf-8")
		if err := h.templates["create"].ExecuteTemplate(c.Writer, "base.html", data); err != nil {
			h.logger.Error("Template error", "error", err)
			c.String(http.StatusInternalServerError, "Template error")
		}
		return
	}

	c.Redirect(http.StatusSeeOther, "/article/"+article.CID)
}

// WebSearch handles search requests from the web UI (HTMX)
func (h *WebHandler) WebSearch(c *gin.Context) {
	q := c.Query("q")
	author := c.Query("author")
	category := c.Query("category")
	tags := c.QueryArray("tags")
	
	query := &search.SearchQuery{
		Query:    q,
		Author:   author,
		Category: category,
		Tags:     tags,
		Page:     1,
		Limit:    20,
	}

	result, err := h.searchService.Search(c.Request.Context(), query)
	if err != nil {
		h.logger.Error("Search failed", "error", err)
		c.String(http.StatusInternalServerError, "Search failed")
		return
	}

	data := gin.H{
		"Articles": result.Articles,
	}

	// Render only the article list component
	if err := h.templates["explore"].ExecuteTemplate(c.Writer, "article_list.html", data); err != nil {
		h.logger.Error("Template error", "error", err)
		c.String(http.StatusInternalServerError, "Template error")
	}
}

// NetworkPage renders the P2P network status page
func (h *WebHandler) NetworkPage(c *gin.Context) {
	user := GetUser(c)
	var peers []gin.H
	var peerID string

	if h.p2pNode != nil {
		peerID = h.p2pNode.GetPeerID().String()
		connectedPeers := h.p2pNode.GetConnectedPeers()

		for _, p := range connectedPeers {
			peers = append(peers, gin.H{
				"ID":     p.String(),
				"Status": "connected",
			})
		}
	}

	data := gin.H{
		"Title":     "P2P Network",
		"User":      user,
		"PeerID":    peerID,
		"Peers":     peers,
		"PeerCount": len(peers),
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := h.templates["network"].ExecuteTemplate(c.Writer, "base.html", data); err != nil {
		h.logger.Error("Template error", "error", err)
		c.String(http.StatusInternalServerError, "Template error")
	}
}

// getPeerCount returns the current peer count
func (h *WebHandler) getPeerCount() int {
	if h.p2pNode != nil {
		return h.p2pNode.GetPeerCount()
	}
	return 0
}

// getOriginIdentifier returns a meaningful origin identifier for articles
// For P2P nodes, it uses the peer ID; for regular clients, it uses the IP
func (h *WebHandler) getOriginIdentifier(c *gin.Context) string {
	// If P2P node is available, use a shortened peer ID as origin
	if h.p2pNode != nil {
		peerID := h.p2pNode.GetPeerID().String()
		// Return shortened peer ID (first 12 chars after "12D3KooW" prefix)
		if len(peerID) > 20 {
			return "P2P:" + peerID[8:20]
		}
		return "P2P:" + peerID
	}

	// Try to get real IP from headers (for reverse proxy setups)
	if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}

	if xri := c.GetHeader("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to Gin's ClientIP
	return c.ClientIP()
}
