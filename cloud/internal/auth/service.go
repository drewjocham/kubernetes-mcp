package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"kube-watcher/cloud/internal/storage"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrTokenExpired       = errors.New("token expired")
	ErrInvalidToken       = errors.New("invalid token")
	ErrUserExists         = errors.New("user already exists")
	ErrProviderMismatch   = errors.New("provider mismatch")
)

type contextKey int

const (
	userKey contextKey = iota
)

// Service handles authentication and authorization
type Service struct {
	store storage.Store
	// In production, these should be loaded from environment/config
	jwtSecret     []byte
	jwtExpiration time.Duration
	oauthConfigs  map[string]*OAuthConfig
}

// OAuthConfig holds OAuth provider configuration
type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	AuthURL      string
	TokenURL     string
	UserInfoURL  string
	Scopes       []string
}

// LoginRequest represents a login request
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"` // For password-based auth (future)
	APIKey   string `json:"api_key"`  // For API key-based auth
}

// LoginResponse represents a successful login response
type LoginResponse struct {
	User  *storage.User `json:"user"`
	Token string        `json:"token"`
}

// RegisterRequest represents a user registration request
type RegisterRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"` // For password-based auth (future)
}

// OAuthCallbackRequest represents an OAuth callback request
type OAuthCallbackRequest struct {
	Code        string `json:"code"`
	State       string `json:"state"`
	Provider    string `json:"provider"`
	RedirectURI string `json:"redirect_uri,omitempty"`
}

// NewService creates a new authentication service
func NewService(store storage.Store) *Service {
	// Generate a random JWT secret if not provided (development only)
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		// Fallback to a fixed secret for development
		secret = []byte("dev-secret-change-in-production")
	}

	return &Service{
		store:         store,
		jwtSecret:     secret,
		jwtExpiration: 24 * time.Hour, // 24 hours
		oauthConfigs:  make(map[string]*OAuthConfig),
	}
}

// RegisterOAuthProvider registers an OAuth provider configuration
func (s *Service) RegisterOAuthProvider(provider string, config *OAuthConfig) {
	s.oauthConfigs[provider] = config
}

// Login handles user login
func (s *Service) Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
	// For now, we support API key-based login for agents
	// and email-based lookup for users
	if req.APIKey != "" {
		// API key login (for agents)
		user, err := s.store.GetUserByAPIKey(ctx, req.APIKey)
		if err != nil {
			return nil, ErrInvalidCredentials
		}

		token, err := s.generateToken(user.ID, user.Email)
		if err != nil {
			return nil, fmt.Errorf("failed to generate token: %w", err)
		}

		return &LoginResponse{
			User:  user,
			Token: token,
		}, nil
	}

	// Email/password login (future)
	if req.Email != "" {
		user, err := s.store.GetUserByEmail(ctx, req.Email)
		if err != nil {
			return nil, ErrInvalidCredentials
		}

		// TODO: Implement password verification when we add passwords
		// For now, just generate token
		token, err := s.generateToken(user.ID, user.Email)
		if err != nil {
			return nil, fmt.Errorf("failed to generate token: %w", err)
		}

		return &LoginResponse{
			User:  user,
			Token: token,
		}, nil
	}

	return nil, ErrInvalidCredentials
}

// Register handles user registration
func (s *Service) Register(ctx context.Context, req *RegisterRequest) (*LoginResponse, error) {
	// Check if user already exists
	existing, err := s.store.GetUserByEmail(ctx, req.Email)
	if err == nil && existing != nil {
		return nil, ErrUserExists
	}

	// Create new user
	user := &storage.User{
		ID:           uuid.New().String(),
		Email:        req.Email,
		Name:         req.Name,
		Provider:     "email", // Email/password provider
		APIKey:       uuid.New().String(),
		Subscription: "free",
	}

	if err := s.store.CreateUser(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	token, err := s.generateToken(user.ID, user.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return &LoginResponse{
		User:  user,
		Token: token,
	}, nil
}

// HandleOAuthCallback handles OAuth provider callback
func (s *Service) HandleOAuthCallback(ctx context.Context, req *OAuthCallbackRequest) (*LoginResponse, error) {
	_, exists := s.oauthConfigs[req.Provider]
	if !exists {
		return nil, fmt.Errorf("unsupported provider: %s", req.Provider)
	}

	// TODO: Implement OAuth flow
	// 1. Exchange code for token
	// 2. Get user info from provider
	// 3. Create or update user
	// 4. Generate JWT token

	// For now, return a mock response
	return nil, errors.New("OAuth not implemented yet")
}

// ValidateToken validates a JWT token and returns the user
func (s *Service) ValidateToken(ctx context.Context, tokenString string) (*storage.User, error) {
	// TODO: Implement JWT validation
	// For now, just parse the token and return a mock user
	// In production, use github.com/golang-jwt/jwt/v5

	// Simple mock validation for development
	if tokenString == "" {
		return nil, ErrInvalidToken
	}

	// For development, accept any non-empty token
	// In production, validate JWT signature and claims
	userID := "mock-user-id"
	email := "mock@example.com"

	user, err := s.store.GetUser(ctx, userID)
	if err != nil {
		// Return mock user for development
		return &storage.User{
			ID:           userID,
			Email:        email,
			Name:         "Mock User",
			Provider:     "mock",
			Subscription: "free",
		}, nil
	}

	return user, nil
}

// GenerateAPIKey generates a new API key for a user
func (s *Service) GenerateAPIKey(ctx context.Context, userID string) (string, error) {
	user, err := s.store.GetUser(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("user not found: %w", err)
	}

	// Generate new API key
	apiKey := uuid.New().String()
	user.APIKey = apiKey

	if err := s.store.UpdateUser(ctx, user); err != nil {
		return "", fmt.Errorf("failed to update user: %w", err)
	}

	return apiKey, nil
}

// generateToken generates a JWT token for a user
func (s *Service) generateToken(userID, email string) (string, error) {
	// TODO: Implement JWT token generation
	// For now, return a simple token
	token := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", userID, email)))
	return token, nil
}

// Middleware returns an authentication middleware for HTTP handlers
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		// Expect "Bearer <token>"
		if len(authHeader) < 7 || authHeader[:7] != "Bearer " {
			http.Error(w, "Invalid authorization format", http.StatusUnauthorized)
			return
		}

		token := authHeader[7:]
		user, err := s.ValidateToken(r.Context(), token)
		if err != nil {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// Add user to context
		ctx := context.WithValue(r.Context(), userKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetUserFromContext extracts the user from context
func GetUserFromContext(ctx context.Context) (*storage.User, bool) {
	user, ok := ctx.Value(userKey).(*storage.User)
	return user, ok
}
