package registry

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"kube-watcher/cloud/internal/storage"
)

var (
	ErrAgentNotFound = errors.New("agent not found")
	ErrInvalidAPIKey = errors.New("invalid API key")
	ErrAgentOffline  = errors.New("agent offline")
)

// Service handles agent registration and management
type Service struct {
	store storage.Store
}

// RegisterRequest represents an agent registration request
type RegisterRequest struct {
	UserID    string            `json:"user_id"`
	Name      string            `json:"name"`
	ClusterID string            `json:"cluster_id"`
	Version   string            `json:"version"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// RegisterResponse represents the response to agent registration
type RegisterResponse struct {
	Agent  *storage.Agent `json:"agent"`
	APIKey string         `json:"api_key"`
}

// HeartbeatRequest represents an agent heartbeat
type HeartbeatRequest struct {
	AgentID string                 `json:"agent_id"`
	APIKey  string                 `json:"api_key"`
	Status  string                 `json:"status,omitempty"`
	Metrics map[string]interface{} `json:"metrics,omitempty"`
}

// HeartbeatResponse represents the heartbeat response
type HeartbeatResponse struct {
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message,omitempty"`
}

// NewService creates a new registry service
func NewService(store storage.Store) *Service {
	return &Service{
		store: store,
	}
}

// RegisterAgent registers a new agent for a user
func (s *Service) RegisterAgent(ctx context.Context, req *RegisterRequest) (*RegisterResponse, error) {
	// Verify user exists
	_, err := s.store.GetUser(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	// Generate API key for the agent
	apiKey := uuid.New().String()

	// Create agent
	agent := &storage.Agent{
		ID:        uuid.New().String(),
		UserID:    req.UserID,
		Name:      req.Name,
		ClusterID: req.ClusterID,
		APIKey:    apiKey,
		Version:   req.Version,
		Status:    "online",
		CreatedAt: time.Now(),
		LastSeen:  time.Now(),
		Metadata:  req.Metadata,
	}

	if err := s.store.CreateAgent(ctx, agent); err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	// Update user subscription check (future: enforce agent limits based on plan)
	// For now, just register the agent

	return &RegisterResponse{
		Agent:  agent,
		APIKey: apiKey,
	}, nil
}

// Heartbeat updates agent last seen time and status
func (s *Service) Heartbeat(ctx context.Context, req *HeartbeatRequest) (*HeartbeatResponse, error) {
	// Authenticate agent using API key
	agent, err := s.store.GetAgentByAPIKey(ctx, req.APIKey)
	if err != nil {
		return nil, ErrInvalidAPIKey
	}

	// Verify agent ID matches (extra security)
	if agent.ID != req.AgentID {
		return nil, ErrInvalidAPIKey
	}

	// Update agent status
	agent.LastSeen = time.Now()
	if req.Status != "" {
		agent.Status = req.Status
	} else {
		// Auto-detect status based on heartbeat timing
		agent.Status = "online"
	}

	// Update metadata if provided
	if req.Metrics != nil {
		if agent.Metadata == nil {
			agent.Metadata = make(map[string]string)
		}
		// Store some metrics as metadata
		for k, v := range req.Metrics {
			agent.Metadata[k] = fmt.Sprintf("%v", v)
		}
	}

	if err := s.store.UpdateAgent(ctx, agent); err != nil {
		return nil, fmt.Errorf("failed to update agent: %w", err)
	}

	return &HeartbeatResponse{
		Timestamp: time.Now(),
		Message:   "heartbeat received",
	}, nil
}

// GetAgent retrieves an agent by ID
func (s *Service) GetAgent(ctx context.Context, agentID string) (*storage.Agent, error) {
	agent, err := s.store.GetAgent(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}
	return agent, nil
}

// ListAgents lists all agents for a user
func (s *Service) ListAgents(ctx context.Context, userID string) ([]*storage.Agent, error) {
	agents, err := s.store.ListAgentsByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}
	return agents, nil
}

// UpdateAgent updates agent information
func (s *Service) UpdateAgent(ctx context.Context, agentID string, updates map[string]interface{}) (*storage.Agent, error) {
	agent, err := s.store.GetAgent(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("agent not found: %w", err)
	}

	// Apply updates
	if name, ok := updates["name"].(string); ok {
		agent.Name = name
	}
	if clusterID, ok := updates["cluster_id"].(string); ok {
		agent.ClusterID = clusterID
	}
	if version, ok := updates["version"].(string); ok {
		agent.Version = version
	}
	if status, ok := updates["status"].(string); ok {
		agent.Status = status
	}
	if metadata, ok := updates["metadata"].(map[string]string); ok {
		agent.Metadata = metadata
	}

	if err := s.store.UpdateAgent(ctx, agent); err != nil {
		return nil, fmt.Errorf("failed to update agent: %w", err)
	}

	return agent, nil
}

// DeleteAgent removes an agent
func (s *Service) DeleteAgent(ctx context.Context, agentID string) error {
	if err := s.store.DeleteAgent(ctx, agentID); err != nil {
		return fmt.Errorf("failed to delete agent: %w", err)
	}
	return nil
}

// RotateAPIKey generates a new API key for an agent
func (s *Service) RotateAPIKey(ctx context.Context, agentID string) (string, error) {
	agent, err := s.store.GetAgent(ctx, agentID)
	if err != nil {
		return "", fmt.Errorf("agent not found: %w", err)
	}

	newAPIKey := uuid.New().String()
	agent.APIKey = newAPIKey

	if err := s.store.UpdateAgent(ctx, agent); err != nil {
		return "", fmt.Errorf("failed to update agent: %w", err)
	}

	return newAPIKey, nil
}

// CleanupOfflineAgents marks agents as offline if they haven't sent a heartbeat recently
func (s *Service) CleanupOfflineAgents(ctx context.Context, maxAge time.Duration) (int, error) {
	// For memory store, we need to iterate through all agents
	// In a real database, we'd use a query
	agents, err := s.store.ListAgentsByUser(ctx, "") // Empty userID to get all agents (not supported yet)
	if err != nil {
		// Memory store doesn't support listing all agents, so we can't implement this fully
		return 0, nil
	}

	updated := 0
	cutoff := time.Now().Add(-maxAge)

	for _, agent := range agents {
		if agent.LastSeen.Before(cutoff) && agent.Status != "offline" {
			agent.Status = "offline"
			if err := s.store.UpdateAgent(ctx, agent); err != nil {
				// Log error but continue
				continue
			}
			updated++
		}
	}

	return updated, nil
}

// ValidateAgent validates an agent's API key and returns the agent
func (s *Service) ValidateAgent(ctx context.Context, apiKey string) (*storage.Agent, error) {
	agent, err := s.store.GetAgentByAPIKey(ctx, apiKey)
	if err != nil {
		return nil, ErrInvalidAPIKey
	}

	// Check if agent is offline (optional)
	if agent.Status == "offline" {
		return nil, ErrAgentOffline
	}

	return agent, nil
}
