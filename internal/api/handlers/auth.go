package handlers

import (
	"github.com/gin-gonic/gin"

	"github.com/amiyamandal-dev/newsp2p/internal/api/middleware"
	"github.com/amiyamandal-dev/newsp2p/internal/domain"
	"github.com/amiyamandal-dev/newsp2p/internal/service"
	"github.com/amiyamandal-dev/newsp2p/pkg/logger"
	"github.com/amiyamandal-dev/newsp2p/pkg/response"
)

// AuthHandler handles authentication-related requests
type AuthHandler struct {
	userService *service.UserService
	logger      *logger.Logger
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(userService *service.UserService, logger *logger.Logger) *AuthHandler {
	return &AuthHandler{
		userService: userService,
		logger:      logger.WithComponent("auth-handler"),
	}
}

// Register handles user registration
func (h *AuthHandler) Register(c *gin.Context) {
	var req domain.UserRegisterRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}

	user, err := h.userService.Register(c.Request.Context(), &req)
	if err != nil {
		if err == domain.ErrUserAlreadyExists {
			response.Conflict(c, "Username or email already exists")
			return
		}
		h.logger.Error("Registration failed", "error", err)
		response.InternalServerError(c, "Failed to register user")
		return
	}

	response.Created(c, user)
}

// Login handles user login
func (h *AuthHandler) Login(c *gin.Context) {
	var req domain.UserLoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}

	loginResp, err := h.userService.Login(c.Request.Context(), &req)
	if err != nil {
		if err == domain.ErrInvalidCredentials {
			response.Unauthorized(c, "Invalid username or password")
			return
		}
		if err == domain.ErrUserNotActive {
			response.Forbidden(c, "User account is not active")
			return
		}
		h.logger.Error("Login failed", "error", err)
		response.InternalServerError(c, "Failed to login")
		return
	}

	response.Success(c, loginResp)
}

// RefreshToken handles token refresh
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}

	tokens, err := h.userService.RefreshToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		response.Unauthorized(c, "Invalid or expired refresh token")
		return
	}

	response.Success(c, tokens)
}

// GetMe returns the current authenticated user
func (h *AuthHandler) GetMe(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	user, err := h.userService.GetUser(c.Request.Context(), userID)
	if err != nil {
		if err == domain.ErrUserNotFound {
			response.NotFound(c, "User not found")
			return
		}
		h.logger.Error("Failed to get user", "error", err)
		response.InternalServerError(c, "Failed to get user")
		return
	}

	response.Success(c, user)
}
