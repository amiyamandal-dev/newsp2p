package handlers

import (
	"github.com/gin-gonic/gin"

	"github.com/amiyamandal-dev/newsp2p/internal/api/middleware"
	"github.com/amiyamandal-dev/newsp2p/internal/domain"
	"github.com/amiyamandal-dev/newsp2p/internal/service"
	"github.com/amiyamandal-dev/newsp2p/pkg/logger"
	"github.com/amiyamandal-dev/newsp2p/pkg/response"
)

// ArticleHandler handles article-related requests
type ArticleHandler struct {
	articleService *service.ArticleService
	logger         *logger.Logger
}

// NewArticleHandler creates a new article handler
func NewArticleHandler(articleService *service.ArticleService, logger *logger.Logger) *ArticleHandler {
	return &ArticleHandler{
		articleService: articleService,
		logger:         logger.WithComponent("article-handler"),
	}
}

// Create handles article creation
func (h *ArticleHandler) Create(c *gin.Context) {
	var req domain.ArticleCreateRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}

	userID := middleware.GetUserID(c)
	if userID == "" {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	article, err := h.articleService.Create(c.Request.Context(), &req, userID, c.ClientIP())
	if err != nil {
		h.logger.Error("Failed to create article", "error", err)
		response.InternalServerError(c, "Failed to create article")
		return
	}

	response.Created(c, article)
}

// GetByCID retrieves an article by CID
func (h *ArticleHandler) GetByCID(c *gin.Context) {
	cid := c.Param("cid")
	if cid == "" {
		response.BadRequest(c, "CID is required")
		return
	}

	article, err := h.articleService.GetByCID(c.Request.Context(), cid)
	if err != nil {
		if err == domain.ErrArticleNotFound {
			response.NotFound(c, "Article not found")
			return
		}
		h.logger.Error("Failed to get article", "cid", cid, "error", err)
		response.InternalServerError(c, "Failed to retrieve article")
		return
	}

	response.Success(c, article)
}

// List retrieves articles with pagination and filtering
func (h *ArticleHandler) List(c *gin.Context) {
	parser := NewQueryParamParser(c)

	pagination := parser.Pagination(20)
	dateRange := parser.DateRange("from", "to")
	author := parser.String("author", "")
	category := parser.String("category", "")

	if err := parser.Error(); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	filter := &domain.ArticleListFilter{
		Author:   author,
		Category: category,
		FromDate: dateRange.From,
		ToDate:   dateRange.To,
		Page:     pagination.Page,
		Limit:    pagination.Limit,
	}

	articles, total, err := h.articleService.List(c.Request.Context(), filter)
	if err != nil {
		h.logger.Error("Failed to list articles", "error", err)
		response.InternalServerError(c, "Failed to list articles")
		return
	}

	response.Paginated(c, articles, pagination.Page, pagination.Limit, total)
}

// Update handles article updates
func (h *ArticleHandler) Update(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.BadRequest(c, "Article ID is required")
		return
	}

	var req domain.ArticleUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}

	userID := middleware.GetUserID(c)
	if userID == "" {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	article, err := h.articleService.Update(c.Request.Context(), id, &req, userID)
	if err != nil {
		if err == domain.ErrArticleNotFound {
			response.NotFound(c, "Article not found")
			return
		}
		if err == domain.ErrForbidden {
			response.Forbidden(c, "You can only update your own articles")
			return
		}
		h.logger.Error("Failed to update article", "id", id, "error", err)
		response.InternalServerError(c, "Failed to update article")
		return
	}

	response.Success(c, article)
}

// Delete handles article deletion
func (h *ArticleHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.BadRequest(c, "Article ID is required")
		return
	}

	userID := middleware.GetUserID(c)
	if userID == "" {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	if err := h.articleService.Delete(c.Request.Context(), id, userID); err != nil {
		if err == domain.ErrArticleNotFound {
			response.NotFound(c, "Article not found")
			return
		}
		if err == domain.ErrForbidden {
			response.Forbidden(c, "You can only delete your own articles")
			return
		}
		h.logger.Error("Failed to delete article", "id", id, "error", err)
		response.InternalServerError(c, "Failed to delete article")
		return
	}

	response.Success(c, gin.H{"message": "Article deleted successfully"})
}

// VerifySignature verifies an article's signature
func (h *ArticleHandler) VerifySignature(c *gin.Context) {
	cid := c.Param("cid")
	if cid == "" {
		response.BadRequest(c, "CID is required")
		return
	}

	valid, err := h.articleService.VerifySignature(c.Request.Context(), cid)
	if err != nil {
		if err == domain.ErrArticleNotFound {
			response.NotFound(c, "Article not found")
			return
		}
		h.logger.Error("Failed to verify signature", "cid", cid, "error", err)
		response.InternalServerError(c, "Failed to verify signature")
		return
	}

	response.Success(c, gin.H{"valid": valid})
}
