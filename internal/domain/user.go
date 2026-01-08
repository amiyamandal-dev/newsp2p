package domain

import (
	"regexp"
	"time"
)

// emailRegex is a simple email validation pattern
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// User represents a user in the system
type User struct {
	ID           string    `json:"id" db:"id"`                 // This is the PeerID
	Username     string    `json:"username" db:"username" binding:"required,min=3,max=50"`
	Email        string    `json:"email,omitempty" db:"email"` // Optional
	PasswordHash string    `json:"-" db:"password_hash"`       // Never expose
	PublicKey    string    `json:"public_key" db:"public_key"` // Ed25519 public key
	PrivateKey   string    `json:"-" db:"private_key"`         // Encrypted, never expose
	IsActive     bool      `json:"is_active" db:"is_active"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// Validate validates the user fields
func (u *User) Validate() error {
	if u.Username == "" {
		return NewValidationError("username", "username is required")
	}
	if len(u.Username) < 3 {
		return NewValidationError("username", "username must be at least 3 characters")
	}
	if len(u.Username) > 50 {
		return NewValidationError("username", "username must be at most 50 characters")
	}
	// Email is optional now
	if u.Email != "" && !emailRegex.MatchString(u.Email) {
		return NewValidationError("email", "invalid email format")
	}
	if u.PasswordHash == "" {
		return NewValidationError("password", "password is required")
	}
	if u.PublicKey == "" {
		return NewValidationError("public_key", "public key is required")
	}
	return nil
}

// UserRegisterRequest represents a user registration request
type UserRegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Email    string `json:"email,omitempty"` // Optional
	Password string `json:"password" binding:"required,min=8"`
}

// UserLoginRequest represents a user login request
type UserLoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// UserResponse represents a safe user response (without sensitive data)
type UserResponse struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	PublicKey string    `json:"public_key"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

// ToResponse converts User to UserResponse
func (u *User) ToResponse() *UserResponse {
	return &UserResponse{
		ID:        u.ID,
		Username:  u.Username,
		Email:     u.Email,
		PublicKey: u.PublicKey,
		IsActive:  u.IsActive,
		CreatedAt: u.CreatedAt,
	}
}

// AuthTokens represents authentication tokens
type AuthTokens struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// LoginResponse represents a successful login response
type LoginResponse struct {
	User   *UserResponse `json:"user"`
	Tokens *AuthTokens   `json:"tokens"`
}
