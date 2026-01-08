package domain

import (
	"errors"
	"fmt"
)

// ValidationError provides detailed validation error information
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed: %s - %s", e.Field, e.Message)
}

// NewValidationError creates a new validation error
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{Field: field, Message: message}
}

var (
	// Article errors
	ErrArticleNotFound      = errors.New("article not found")
	ErrArticleAlreadyExists = errors.New("article already exists")
	ErrInvalidArticle       = errors.New("invalid article")
	ErrInvalidSignature     = errors.New("invalid article signature")

	// User errors
	ErrUserNotFound       = errors.New("user not found")
	ErrUserAlreadyExists  = errors.New("user already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidUser        = errors.New("invalid user")
	ErrUserNotActive      = errors.New("user account is not active")

	// Feed errors
	ErrFeedNotFound      = errors.New("feed not found")
	ErrFeedAlreadyExists = errors.New("feed already exists")
	ErrInvalidFeed       = errors.New("invalid feed")

	// Auth errors
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token has expired")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")

	// IPFS errors
	ErrIPFSUnavailable   = errors.New("IPFS service unavailable")
	ErrIPFSUploadFailed  = errors.New("IPFS upload failed")
	ErrIPNSPublishFailed = errors.New("IPNS publish failed")
	ErrInvalidCID        = errors.New("invalid CID")

	// Validation errors
	ErrValidationFailed = errors.New("validation failed")
	ErrInvalidInput     = errors.New("invalid input")

	// General errors
	ErrInternal = errors.New("internal server error")
	ErrNotFound = errors.New("resource not found")
	ErrConflict = errors.New("resource conflict")
)
