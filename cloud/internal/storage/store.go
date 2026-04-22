package storage

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound = errors.New("not found")
	ErrExists   = errors.New("already exists")
)

// User represents a cloud service user
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	Name         string    `json:"name"`
	AvatarURL    string    `json:"avatar_url,omitempty"`
	Provider     string    `json:"provider"` // github, google, etc.
	ProviderID   string    `json:"provider_id"`
	APIKey       string    `json:"api_key,omitempty"`
	Subscription string    `json:"subscription"` // free, pro, enterprise
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Agent represents a registered watcher agent (cluster)
type Agent struct {
	ID        string            `json:"id"`
	UserID    string            `json:"user_id"`
	Name      string            `json:"name"`
	ClusterID string            `json:"cluster_id"`
	APIKey    string            `json:"api_key"`
	LastSeen  time.Time         `json:"last_seen"`
	Version   string            `json:"version"`
	Status    string            `json:"status"` // online, offline, error
	CreatedAt time.Time         `json:"created_at"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// Alert represents an alert from a watcher agent
type Alert struct {
	ID             string                 `json:"id"`
	AgentID        string                 `json:"agent_id"`
	UserID         string                 `json:"user_id"`
	RuleName       string                 `json:"rule_name"`
	Severity       string                 `json:"severity"`
	Message        string                 `json:"message"`
	ResourceKind   string                 `json:"resource_kind"`
	Namespace      string                 `json:"namespace"`
	ResourceName   string                 `json:"resource_name"`
	Details        map[string]interface{} `json:"details,omitempty"`
	Timestamp      time.Time              `json:"timestamp"`
	Acknowledged   bool                   `json:"acknowledged"`
	AcknowledgedBy string                 `json:"acknowledged_by,omitempty"`
	AcknowledgedAt *time.Time             `json:"acknowledged_at,omitempty"`
}

// Subscription represents a user's subscription plan
type Subscription struct {
	UserID               string    `json:"user_id"`
	Plan                 string    `json:"plan"`
	Status               string    `json:"status"` // active, canceled, past_due
	CurrentPeriodStart   time.Time `json:"current_period_start"`
	CurrentPeriodEnd     time.Time `json:"current_period_end"`
	CancelAtPeriodEnd    bool      `json:"cancel_at_period_end"`
	StripeCustomerID     string    `json:"stripe_customer_id,omitempty"`
	StripeSubscriptionID string    `json:"stripe_subscription_id,omitempty"`
}

// Store defines the interface for data storage
type Store interface {
	// Users
	CreateUser(ctx context.Context, user *User) error
	GetUser(ctx context.Context, id string) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetUserByAPIKey(ctx context.Context, apiKey string) (*User, error)
	GetUserByProviderID(ctx context.Context, provider, providerID string) (*User, error)
	UpdateUser(ctx context.Context, user *User) error

	// Agents
	CreateAgent(ctx context.Context, agent *Agent) error
	GetAgent(ctx context.Context, id string) (*Agent, error)
	GetAgentByAPIKey(ctx context.Context, apiKey string) (*Agent, error)
	ListAgentsByUser(ctx context.Context, userID string) ([]*Agent, error)
	UpdateAgent(ctx context.Context, agent *Agent) error
	DeleteAgent(ctx context.Context, id string) error

	// Alerts
	CreateAlert(ctx context.Context, alert *Alert) error
	GetAlert(ctx context.Context, id string) (*Alert, error)
	ListAlertsByUser(ctx context.Context, userID string, limit, offset int) ([]*Alert, error)
	ListAlertsByAgent(ctx context.Context, agentID string, limit, offset int) ([]*Alert, error)
	UpdateAlert(ctx context.Context, alert *Alert) error
	DeleteOldAlerts(ctx context.Context, olderThan time.Time) error

	// Subscriptions
	GetSubscription(ctx context.Context, userID string) (*Subscription, error)
	UpdateSubscription(ctx context.Context, subscription *Subscription) error
}

// MemoryStore is an in-memory implementation of Store (for development)
type MemoryStore struct {
	mu sync.RWMutex

	users         map[string]*User
	agents        map[string]*Agent
	alerts        map[string]*Alert
	subscriptions map[string]*Subscription

	// Indexes
	usersByEmail      map[string]string
	usersByAPIKey     map[string]string
	usersByProviderID map[string]string // key: provider:providerID
	agentsByAPIKey    map[string]string
	agentsByUser      map[string][]string
	alertsByUser      map[string][]string
	alertsByAgent     map[string][]string
}

func NewMemoryStore() (*MemoryStore, error) {
	return &MemoryStore{
		users:         make(map[string]*User),
		agents:        make(map[string]*Agent),
		alerts:        make(map[string]*Alert),
		subscriptions: make(map[string]*Subscription),

		usersByEmail:      make(map[string]string),
		usersByAPIKey:     make(map[string]string),
		usersByProviderID: make(map[string]string),
		agentsByAPIKey:    make(map[string]string),
		agentsByUser:      make(map[string][]string),
		alertsByUser:      make(map[string][]string),
		alertsByAgent:     make(map[string][]string),
	}, nil
}

// User methods
func (s *MemoryStore) CreateUser(ctx context.Context, user *User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if user.ID == "" {
		user.ID = uuid.New().String()
	}

	if _, exists := s.users[user.ID]; exists {
		return ErrExists
	}

	if _, exists := s.usersByEmail[user.Email]; exists {
		return ErrExists
	}

	if user.ProviderID != "" {
		key := fmt.Sprintf("%s:%s", user.Provider, user.ProviderID)
		if _, exists := s.usersByProviderID[key]; exists {
			return ErrExists
		}
	}

	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now

	if user.APIKey == "" {
		user.APIKey = uuid.New().String()
	}

	s.users[user.ID] = user
	s.usersByEmail[user.Email] = user.ID
	s.usersByAPIKey[user.APIKey] = user.ID

	if user.ProviderID != "" {
		key := fmt.Sprintf("%s:%s", user.Provider, user.ProviderID)
		s.usersByProviderID[key] = user.ID
	}

	// Create default subscription
	s.subscriptions[user.ID] = &Subscription{
		UserID:             user.ID,
		Plan:               "free",
		Status:             "active",
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(100, 0, 0), // Far future for free tier
		CancelAtPeriodEnd:  false,
	}

	return nil
}

func (s *MemoryStore) GetUser(ctx context.Context, id string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, exists := s.users[id]
	if !exists {
		return nil, ErrNotFound
	}

	return user, nil
}

func (s *MemoryStore) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	id, exists := s.usersByEmail[email]
	if !exists {
		return nil, ErrNotFound
	}

	return s.GetUser(ctx, id)
}

func (s *MemoryStore) GetUserByAPIKey(ctx context.Context, apiKey string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	id, exists := s.usersByAPIKey[apiKey]
	if !exists {
		return nil, ErrNotFound
	}

	return s.GetUser(ctx, id)
}

func (s *MemoryStore) GetUserByProviderID(ctx context.Context, provider, providerID string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := fmt.Sprintf("%s:%s", provider, providerID)
	id, exists := s.usersByProviderID[key]
	if !exists {
		return nil, ErrNotFound
	}

	return s.GetUser(ctx, id)
}

func (s *MemoryStore) UpdateUser(ctx context.Context, user *User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, exists := s.users[user.ID]
	if !exists {
		return ErrNotFound
	}

	// Check if email changed
	if existing.Email != user.Email {
		if _, emailExists := s.usersByEmail[user.Email]; emailExists {
			return errors.New("email already in use")
		}
		delete(s.usersByEmail, existing.Email)
		s.usersByEmail[user.Email] = user.ID
	}

	// Check if API key changed
	if existing.APIKey != user.APIKey {
		if _, keyExists := s.usersByAPIKey[user.APIKey]; keyExists {
			return errors.New("API key already in use")
		}
		delete(s.usersByAPIKey, existing.APIKey)
		s.usersByAPIKey[user.APIKey] = user.ID
	}

	user.UpdatedAt = time.Now()
	s.users[user.ID] = user

	return nil
}

// Agent methods
func (s *MemoryStore) CreateAgent(ctx context.Context, agent *Agent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if agent.ID == "" {
		agent.ID = uuid.New().String()
	}

	if _, exists := s.agents[agent.ID]; exists {
		return ErrExists
	}

	if _, exists := s.agentsByAPIKey[agent.APIKey]; exists {
		return ErrExists
	}

	// Verify user exists
	if _, exists := s.users[agent.UserID]; !exists {
		return ErrNotFound
	}

	now := time.Now()
	agent.CreatedAt = now
	agent.LastSeen = now
	agent.Status = "online"

	if agent.APIKey == "" {
		agent.APIKey = uuid.New().String()
	}

	s.agents[agent.ID] = agent
	s.agentsByAPIKey[agent.APIKey] = agent.ID
	s.agentsByUser[agent.UserID] = append(s.agentsByUser[agent.UserID], agent.ID)

	return nil
}

func (s *MemoryStore) GetAgent(ctx context.Context, id string) (*Agent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	agent, exists := s.agents[id]
	if !exists {
		return nil, ErrNotFound
	}

	return agent, nil
}

func (s *MemoryStore) GetAgentByAPIKey(ctx context.Context, apiKey string) (*Agent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	id, exists := s.agentsByAPIKey[apiKey]
	if !exists {
		return nil, ErrNotFound
	}

	return s.GetAgent(ctx, id)
}

func (s *MemoryStore) ListAgentsByUser(ctx context.Context, userID string) ([]*Agent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	agentIDs, exists := s.agentsByUser[userID]
	if !exists {
		return []*Agent{}, nil
	}

	agents := make([]*Agent, 0, len(agentIDs))
	for _, id := range agentIDs {
		if agent, exists := s.agents[id]; exists {
			agents = append(agents, agent)
		}
	}

	return agents, nil
}

func (s *MemoryStore) UpdateAgent(ctx context.Context, agent *Agent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, exists := s.agents[agent.ID]
	if !exists {
		return ErrNotFound
	}

	// Check if API key changed
	if existing.APIKey != agent.APIKey {
		if _, keyExists := s.agentsByAPIKey[agent.APIKey]; keyExists {
			return errors.New("API key already in use")
		}
		delete(s.agentsByAPIKey, existing.APIKey)
		s.agentsByAPIKey[agent.APIKey] = agent.ID
	}

	s.agents[agent.ID] = agent

	return nil
}

func (s *MemoryStore) DeleteAgent(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	agent, exists := s.agents[id]
	if !exists {
		return ErrNotFound
	}

	// Clean up indexes
	delete(s.agentsByAPIKey, agent.APIKey)

	// Remove from user's agent list
	if agentIDs, exists := s.agentsByUser[agent.UserID]; exists {
		for i, agentID := range agentIDs {
			if agentID == id {
				s.agentsByUser[agent.UserID] = append(agentIDs[:i], agentIDs[i+1:]...)
				break
			}
		}
	}

	// Clean up alerts
	if alertIDs, exists := s.alertsByAgent[id]; exists {
		for _, alertID := range alertIDs {
			delete(s.alerts, alertID)
		}
		delete(s.alertsByAgent, id)
	}

	delete(s.agents, id)

	return nil
}

// Alert methods
func (s *MemoryStore) CreateAlert(ctx context.Context, alert *Alert) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if alert.ID == "" {
		alert.ID = uuid.New().String()
	}

	if _, exists := s.alerts[alert.ID]; exists {
		return ErrExists
	}

	// Verify agent exists
	agent, exists := s.agents[alert.AgentID]
	if !exists {
		return ErrNotFound
	}

	alert.UserID = agent.UserID
	alert.Timestamp = time.Now()

	s.alerts[alert.ID] = alert
	s.alertsByUser[alert.UserID] = append(s.alertsByUser[alert.UserID], alert.ID)
	s.alertsByAgent[alert.AgentID] = append(s.alertsByAgent[alert.AgentID], alert.ID)

	return nil
}

func (s *MemoryStore) GetAlert(ctx context.Context, id string) (*Alert, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	alert, exists := s.alerts[id]
	if !exists {
		return nil, ErrNotFound
	}

	return alert, nil
}

func (s *MemoryStore) ListAlertsByUser(ctx context.Context, userID string, limit, offset int) ([]*Alert, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	alertIDs, exists := s.alertsByUser[userID]
	if !exists {
		return []*Alert{}, nil
	}

	// Apply offset and limit
	start := offset
	if start < 0 {
		start = 0
	}
	if start > len(alertIDs) {
		start = len(alertIDs)
	}

	end := start + limit
	if end > len(alertIDs) {
		end = len(alertIDs)
	}
	if limit <= 0 {
		end = len(alertIDs)
	}

	selectedIDs := alertIDs[start:end]
	alerts := make([]*Alert, 0, len(selectedIDs))
	for _, id := range selectedIDs {
		if alert, exists := s.alerts[id]; exists {
			alerts = append(alerts, alert)
		}
	}

	return alerts, nil
}

func (s *MemoryStore) ListAlertsByAgent(ctx context.Context, agentID string, limit, offset int) ([]*Alert, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	alertIDs, exists := s.alertsByAgent[agentID]
	if !exists {
		return []*Alert{}, nil
	}

	// Apply offset and limit
	start := offset
	if start < 0 {
		start = 0
	}
	if start > len(alertIDs) {
		start = len(alertIDs)
	}

	end := start + limit
	if end > len(alertIDs) {
		end = len(alertIDs)
	}
	if limit <= 0 {
		end = len(alertIDs)
	}

	selectedIDs := alertIDs[start:end]
	alerts := make([]*Alert, 0, len(selectedIDs))
	for _, id := range selectedIDs {
		if alert, exists := s.alerts[id]; exists {
			alerts = append(alerts, alert)
		}
	}

	return alerts, nil
}

func (s *MemoryStore) UpdateAlert(ctx context.Context, alert *Alert) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, exists := s.alerts[alert.ID]
	if !exists {
		return ErrNotFound
	}

	s.alerts[alert.ID] = alert
	return nil
}

func (s *MemoryStore) DeleteOldAlerts(ctx context.Context, olderThan time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// In a real implementation, we would delete old alerts
	// For memory store, we'll just implement a basic version
	// that clears alerts older than the given time

	for id, alert := range s.alerts {
		if alert.Timestamp.Before(olderThan) {
			delete(s.alerts, id)

			// Also remove from indexes
			// Note: This is inefficient but okay for memory store
		}
	}

	// Rebuild indexes
	s.alertsByUser = make(map[string][]string)
	s.alertsByAgent = make(map[string][]string)

	for id, alert := range s.alerts {
		s.alertsByUser[alert.UserID] = append(s.alertsByUser[alert.UserID], id)
		s.alertsByAgent[alert.AgentID] = append(s.alertsByAgent[alert.AgentID], id)
	}

	return nil
}

// Subscription methods
func (s *MemoryStore) GetSubscription(ctx context.Context, userID string) (*Subscription, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	subscription, exists := s.subscriptions[userID]
	if !exists {
		return nil, ErrNotFound
	}

	return subscription, nil
}

func (s *MemoryStore) UpdateSubscription(ctx context.Context, subscription *Subscription) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.subscriptions[subscription.UserID] = subscription
	return nil
}
