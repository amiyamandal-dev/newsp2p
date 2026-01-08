package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/amiyamandal-dev/newsp2p/internal/auth"
	"github.com/amiyamandal-dev/newsp2p/internal/domain"
	"github.com/amiyamandal-dev/newsp2p/internal/repository"
	"github.com/amiyamandal-dev/newsp2p/pkg/crypto"
	"github.com/amiyamandal-dev/newsp2p/pkg/logger"
)

// UserService handles user-related business logic
type UserService struct {
	userRepo   repository.UserRepository
	jwtManager *auth.JWTManager
	bcryptCost int
	logger     *logger.Logger
}

// NewUserService creates a new user service
func NewUserService(
	userRepo repository.UserRepository,
	jwtManager *auth.JWTManager,
	bcryptCost int,
	logger *logger.Logger,
) *UserService {
	return &UserService{
		userRepo:   userRepo,
		jwtManager: jwtManager,
		bcryptCost: bcryptCost,
		logger:     logger.WithComponent("user-service"),
	}
}

// Register registers a new user
func (s *UserService) Register(ctx context.Context, req *domain.UserRegisterRequest) (*domain.UserResponse, error) {
	// Validate password length
	if len(req.Password) < 8 {
		return nil, fmt.Errorf("password must be at least 8 characters")
	}

	// Check if username exists
	exists, err := s.userRepo.ExistsByUsername(ctx, req.Username)
	if err != nil {
		s.logger.Error("Failed to check username existence", "error", err)
		return nil, fmt.Errorf("failed to check username: %w", err)
	}
	if exists {
		return nil, domain.ErrUserAlreadyExists
	}

	// Check if email exists
	exists, err = s.userRepo.ExistsByEmail(ctx, req.Email)
	if err != nil {
		s.logger.Error("Failed to check email existence", "error", err)
		return nil, fmt.Errorf("failed to check email: %w", err)
	}
	if exists {
		return nil, domain.ErrUserAlreadyExists
	}

	// Generate Ed25519 key pair for article signing
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		s.logger.Error("Failed to generate key pair", "error", err)
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}

	// Hash password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), s.bcryptCost)
	if err != nil {
		s.logger.Error("Failed to hash password", "error", err)
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Encrypt private key using password hash as the encryption key
	// This allows decryption later using the stored password hash
	encryptedPrivateKey, err := crypto.EncryptPrivateKey(keyPair.PrivateKey, string(passwordHash))
	if err != nil {
		s.logger.Error("Failed to encrypt private key", "error", err)
		return nil, fmt.Errorf("failed to encrypt private key: %w", err)
	}

	// Create user
	user := &domain.User{
		ID:           uuid.New().String(),
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: string(passwordHash),
		PublicKey:    crypto.PublicKeyToString(keyPair.PublicKey),
		PrivateKey:   encryptedPrivateKey,
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		s.logger.Error("Failed to create user", "error", err)
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	s.logger.Info("User registered successfully", "user_id", user.ID, "username", user.Username)

	return user.ToResponse(), nil
}

// Login authenticates a user and returns tokens
func (s *UserService) Login(ctx context.Context, req *domain.UserLoginRequest) (*domain.LoginResponse, error) {
	// Get user by username
	user, err := s.userRepo.GetByUsername(ctx, req.Username)
	if err != nil {
		if err == domain.ErrUserNotFound {
			return nil, domain.ErrInvalidCredentials
		}
		s.logger.Error("Failed to get user", "error", err)
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Check if user is active
	if !user.IsActive {
		return nil, domain.ErrUserNotActive
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, domain.ErrInvalidCredentials
	}

	// Generate tokens
	tokens, err := s.jwtManager.GenerateTokenPair(user.ID, user.Username, user.Email)
	if err != nil {
		s.logger.Error("Failed to generate tokens", "error", err)
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	s.logger.Info("User logged in successfully", "user_id", user.ID, "username", user.Username)

	return &domain.LoginResponse{
		User:   user.ToResponse(),
		Tokens: tokens,
	}, nil
}

// RefreshToken refreshes an access token using a refresh token
func (s *UserService) RefreshToken(ctx context.Context, refreshToken string) (*domain.AuthTokens, error) {
	// Validate refresh token
	claims, err := s.jwtManager.ValidateToken(refreshToken)
	if err != nil {
		return nil, err
	}

	// Verify user still exists and is active
	user, err := s.userRepo.GetByID(ctx, claims.UserID)
	if err != nil {
		return nil, err
	}

	if !user.IsActive {
		return nil, domain.ErrUserNotActive
	}

	// Generate new tokens
	tokens, err := s.jwtManager.GenerateTokenPair(user.ID, user.Username, user.Email)
	if err != nil {
		s.logger.Error("Failed to generate tokens", "error", err)
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	s.logger.Info("Token refreshed successfully", "user_id", user.ID)

	return tokens, nil
}

// GetUser retrieves a user by ID
func (s *UserService) GetUser(ctx context.Context, userID string) (*domain.UserResponse, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	return user.ToResponse(), nil
}

// GetUserWithPrivateKey retrieves a user with their private key (for article signing)
func (s *UserService) GetUserWithPrivateKey(ctx context.Context, userID, password string) (*domain.User, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, domain.ErrInvalidCredentials
	}

	return user, nil
}
