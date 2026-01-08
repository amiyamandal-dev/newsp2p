package auth

import (
	"fmt"
	"time"

	"github.com/amiyamandal-dev/newsp2p/internal/domain"
	"github.com/golang-jwt/jwt/v5"
)

// Claims represents JWT claims
type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	jwt.RegisteredClaims
}

// JWTManager manages JWT tokens
type JWTManager struct {
	secret             []byte
	accessTokenExpiry  time.Duration
	refreshTokenExpiry time.Duration
}

// NewJWTManager creates a new JWT manager
func NewJWTManager(secret string, accessExpiry, refreshExpiry time.Duration) *JWTManager {
	return &JWTManager{
		secret:             []byte(secret),
		accessTokenExpiry:  accessExpiry,
		refreshTokenExpiry: refreshExpiry,
	}
}

// GenerateAccessToken generates a new access token
func (m *JWTManager) GenerateAccessToken(userID, username, email string) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(m.accessTokenExpiry)

	claims := &Claims{
		UserID:   userID,
		Username: username,
		Email:    email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, expiresAt, nil
}

// GenerateRefreshToken generates a new refresh token
func (m *JWTManager) GenerateRefreshToken(userID, username, email string) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(m.refreshTokenExpiry)

	claims := &Claims{
		UserID:   userID,
		Username: username,
		Email:    email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to sign refresh token: %w", err)
	}

	return tokenString, expiresAt, nil
}

// ValidateToken validates a JWT token and returns the claims
func (m *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method is exactly HS256
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return m.secret, nil
	})

	if err != nil {
		// jwt.ParseWithClaims already handles expiration validation
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, domain.ErrInvalidToken
	}

	return claims, nil
}

// GenerateTokenPair generates both access and refresh tokens
func (m *JWTManager) GenerateTokenPair(userID, username, email string) (*domain.AuthTokens, error) {
	accessToken, expiresAt, err := m.GenerateAccessToken(userID, username, email)
	if err != nil {
		return nil, err
	}

	refreshToken, _, err := m.GenerateRefreshToken(userID, username, email)
	if err != nil {
		return nil, err
	}

	return &domain.AuthTokens{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
	}, nil
}
