package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response represents a standard API response
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Message string      `json:"message,omitempty"`
}

// PaginatedResponse represents a paginated API response
type PaginatedResponse struct {
	Success    bool        `json:"success"`
	Data       interface{} `json:"data"`
	Pagination Pagination  `json:"pagination"`
}

// Pagination represents pagination metadata
type Pagination struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// Success sends a successful response
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Success: true,
		Data:    data,
	})
}

// Created sends a 201 Created response
func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, Response{
		Success: true,
		Data:    data,
	})
}

// SuccessWithMessage sends a successful response with a message
func SuccessWithMessage(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// Paginated sends a paginated response
func Paginated(c *gin.Context, data interface{}, page, limit, total int) {
	totalPages := 0
	if limit > 0 {
		totalPages = total / limit
		if total%limit > 0 {
			totalPages++
		}
	}

	c.JSON(http.StatusOK, PaginatedResponse{
		Success: true,
		Data:    data,
		Pagination: Pagination{
			Page:       page,
			Limit:      limit,
			Total:      total,
			TotalPages: totalPages,
		},
	})
}

// Error sends an error response
func Error(c *gin.Context, statusCode int, message string) {
	c.JSON(statusCode, Response{
		Success: false,
		Error:   message,
	})
}

// BadRequest sends a 400 Bad Request response
func BadRequest(c *gin.Context, message string) {
	Error(c, http.StatusBadRequest, message)
}

// Unauthorized sends a 401 Unauthorized response
func Unauthorized(c *gin.Context, message string) {
	Error(c, http.StatusUnauthorized, message)
}

// Forbidden sends a 403 Forbidden response
func Forbidden(c *gin.Context, message string) {
	Error(c, http.StatusForbidden, message)
}

// NotFound sends a 404 Not Found response
func NotFound(c *gin.Context, message string) {
	Error(c, http.StatusNotFound, message)
}

// Conflict sends a 409 Conflict response
func Conflict(c *gin.Context, message string) {
	Error(c, http.StatusConflict, message)
}

// InternalServerError sends a 500 Internal Server Error response
func InternalServerError(c *gin.Context, message string) {
	Error(c, http.StatusInternalServerError, message)
}
