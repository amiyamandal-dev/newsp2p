package web

import (
	"html/template"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/amiyamandal-dev/newsp2p/internal/auth"
	"github.com/amiyamandal-dev/newsp2p/internal/domain"
	"github.com/amiyamandal-dev/newsp2p/internal/p2p"
	"github.com/amiyamandal-dev/newsp2p/internal/repository/badger"
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
	log *logger.Logger,
) *WebHandler {
	// Load templates with custom functions
	funcMap := template.FuncMap{
		"truncate": func(length int, s string) string {
			if len(s) <= length {
				return s
			}
			return s[:length] + "..."
		},
		"upper": strings.ToUpper,
	}

	// Create template map - parse each page with base layout
	templates := make(map[string]*template.Template)

	baseLayout := "web/templates/layouts/base.html"
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
		tmpl := template.Must(
			template.New(name).Funcs(funcMap).ParseFiles(baseLayout, pagePath),
		)
		templates[name] = tmpl
	}

	return &WebHandler{
		articleService: articleService,
		userService:    userService,
		searchService:  searchService,
		jwtManager:     jwtManager,
		db:             db,
		p2pNode:        p2pNode,
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
			"IPFSOnline":    true, // TODO: Check actual IPFS status
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
			c.SetCookie(CookieAccessToken, token, 3600*24, "/", "", false, true)
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
		h.templates["login"].ExecuteTemplate(c.Writer, "base.html", data)
		return
	}

	// Set cookie
	// MaxAge in seconds (24 hours)
	c.SetCookie(CookieAccessToken, loginResp.Tokens.AccessToken, 3600*24, "/", "", false, true)

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
		h.templates["register"].ExecuteTemplate(c.Writer, "base.html", data)
		return
	}

	// Auto login after register
	loginReq := &domain.UserLoginRequest{
		Username: username,
		Password: password,
	}
	loginResp, err := h.userService.Login(c.Request.Context(), loginReq)
	if err == nil {
		c.SetCookie(CookieAccessToken, loginResp.Tokens.AccessToken, 3600*24, "/", "", false, true)
	}

	c.Redirect(http.StatusSeeOther, "/")
}

// WebLogout handles logout
func (h *WebHandler) WebLogout(c *gin.Context) {
	c.SetCookie(CookieAccessToken, "", -1, "/", "", false, true)
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

	article, err := h.articleService.Create(c.Request.Context(), req, user.ID)
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
		h.templates["create"].ExecuteTemplate(c.Writer, "base.html", data)
		return
	}

	c.Redirect(http.StatusSeeOther, "/article/"+article.CID)
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
