package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"kube-watcher/cloud/internal/auth"
	"kube-watcher/cloud/internal/registry"
	"kube-watcher/cloud/internal/router"
	"kube-watcher/cloud/internal/storage"
)

type contextKey int

const (
	userKey contextKey = iota
)

// Handler handles HTTP API requests
type Handler struct {
	authService     *auth.Service
	registryService *registry.Service
	routerService   *router.Service
	store           storage.Store
}

// NewHandler creates a new API handler
func NewHandler(authService *auth.Service, registryService *registry.Service, routerService *router.Service) *Handler {
	return &Handler{
		authService:     authService,
		registryService: registryService,
		routerService:   routerService,
	}
}

// RegisterRoutes registers all API routes
func (h *Handler) RegisterRoutes(r chi.Router) {
	// Auth routes (public)
	r.Post("/auth/login", h.HandleLogin)
	r.Post("/auth/register", h.HandleRegister)
	r.Post("/auth/oauth/callback", h.HandleOAuthCallback)

	// Alert ingestion (authenticated via API key)
	r.Post("/api/alerts", h.HandleIngestAlert)

	// API routes (protected)
	r.Group(func(r chi.Router) {
		r.Use(h.AuthMiddleware)

		// Agents
		r.Get("/api/agents", h.HandleListAgents)
		r.Post("/api/agents", h.HandleRegisterAgent)
		r.Get("/api/agents/{id}", h.HandleGetAgent)
		r.Put("/api/agents/{id}", h.HandleUpdateAgent)
		r.Delete("/api/agents/{id}", h.HandleDeleteAgent)
		r.Post("/api/agents/{id}/rotate-key", h.HandleRotateAgentKey)

		// Alerts
		r.Get("/api/alerts", h.HandleListAlerts)
		r.Get("/api/alerts/{id}", h.HandleGetAlert)
		r.Put("/api/alerts/{id}/acknowledge", h.HandleAcknowledgeAlert)

		// Subscription
		r.Get("/api/subscription", h.HandleGetSubscription)
		r.Post("/api/subscription/upgrade", h.HandleUpgradeSubscription)
		r.Post("/api/subscription/cancel", h.HandleCancelSubscription)

		// WebSocket
		r.Get("/ws", h.HandleWebSocket)
	})
}

// AuthMiddleware authenticates requests using JWT token
func (h *Handler) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			h.respondError(w, r, http.StatusUnauthorized, "Authorization header required", nil)
			return
		}

		// Expect "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			h.respondError(w, r, http.StatusUnauthorized, "Invalid authorization format", nil)
			return
		}

		token := parts[1]
		user, err := h.authService.ValidateToken(r.Context(), token)
		if err != nil {
			h.respondError(w, r, http.StatusUnauthorized, "Invalid token", err)
			return
		}

		// Add user to context
		ctx := context.WithValue(r.Context(), userKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// HandleLogin handles user login
func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req auth.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, r, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	resp, err := h.authService.Login(r.Context(), &req)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			h.respondError(w, r, http.StatusUnauthorized, "Invalid credentials", err)
		} else {
			h.respondError(w, r, http.StatusInternalServerError, "Login failed", err)
		}
		return
	}

	h.respondJSON(w, r, http.StatusOK, resp)
}

// HandleRegister handles user registration
func (h *Handler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	var req auth.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, r, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	resp, err := h.authService.Register(r.Context(), &req)
	if err != nil {
		if errors.Is(err, auth.ErrUserExists) {
			h.respondError(w, r, http.StatusConflict, "User already exists", err)
		} else {
			h.respondError(w, r, http.StatusInternalServerError, "Registration failed", err)
		}
		return
	}

	h.respondJSON(w, r, http.StatusCreated, resp)
}

// HandleOAuthCallback handles OAuth provider callback
func (h *Handler) HandleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	var req auth.OAuthCallbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, r, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	resp, err := h.authService.HandleOAuthCallback(r.Context(), &req)
	if err != nil {
		h.respondError(w, r, http.StatusInternalServerError, "OAuth callback failed", err)
		return
	}

	h.respondJSON(w, r, http.StatusOK, resp)
}

// HandleRegisterAgent handles agent registration
func (h *Handler) HandleRegisterAgent(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		h.respondError(w, r, http.StatusUnauthorized, "User not found in context", nil)
		return
	}

	var req registry.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, r, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	req.UserID = user.ID
	resp, err := h.registryService.RegisterAgent(r.Context(), &req)
	if err != nil {
		h.respondError(w, r, http.StatusInternalServerError, "Failed to register agent", err)
		return
	}

	h.respondJSON(w, r, http.StatusCreated, resp)
}

// HandleListAgents lists user's agents
func (h *Handler) HandleListAgents(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		h.respondError(w, r, http.StatusUnauthorized, "User not found in context", nil)
		return
	}

	agents, err := h.registryService.ListAgents(r.Context(), user.ID)
	if err != nil {
		h.respondError(w, r, http.StatusInternalServerError, "Failed to list agents", err)
		return
	}

	h.respondJSON(w, r, http.StatusOK, agents)
}

// HandleGetAgent gets a specific agent
func (h *Handler) HandleGetAgent(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		h.respondError(w, r, http.StatusUnauthorized, "User not found in context", nil)
		return
	}

	agentID := chi.URLParam(r, "id")
	agent, err := h.registryService.GetAgent(r.Context(), agentID)
	if err != nil {
		if errors.Is(err, registry.ErrAgentNotFound) {
			h.respondError(w, r, http.StatusNotFound, "Agent not found", err)
		} else {
			h.respondError(w, r, http.StatusInternalServerError, "Failed to get agent", err)
		}
		return
	}

	// Verify user owns the agent
	if agent.UserID != user.ID {
		h.respondError(w, r, http.StatusForbidden, "Access denied", nil)
		return
	}

	h.respondJSON(w, r, http.StatusOK, agent)
}

// HandleUpdateAgent updates an agent
func (h *Handler) HandleUpdateAgent(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		h.respondError(w, r, http.StatusUnauthorized, "User not found in context", nil)
		return
	}

	agentID := chi.URLParam(r, "id")
	agent, err := h.registryService.GetAgent(r.Context(), agentID)
	if err != nil {
		if errors.Is(err, registry.ErrAgentNotFound) {
			h.respondError(w, r, http.StatusNotFound, "Agent not found", err)
		} else {
			h.respondError(w, r, http.StatusInternalServerError, "Failed to get agent", err)
		}
		return
	}

	// Verify user owns the agent
	if agent.UserID != user.ID {
		h.respondError(w, r, http.StatusForbidden, "Access denied", nil)
		return
	}

	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		h.respondError(w, r, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	updatedAgent, err := h.registryService.UpdateAgent(r.Context(), agentID, updates)
	if err != nil {
		h.respondError(w, r, http.StatusInternalServerError, "Failed to update agent", err)
		return
	}

	h.respondJSON(w, r, http.StatusOK, updatedAgent)
}

// HandleDeleteAgent deletes an agent
func (h *Handler) HandleDeleteAgent(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		h.respondError(w, r, http.StatusUnauthorized, "User not found in context", nil)
		return
	}

	agentID := chi.URLParam(r, "id")
	agent, err := h.registryService.GetAgent(r.Context(), agentID)
	if err != nil {
		if errors.Is(err, registry.ErrAgentNotFound) {
			h.respondError(w, r, http.StatusNotFound, "Agent not found", err)
		} else {
			h.respondError(w, r, http.StatusInternalServerError, "Failed to get agent", err)
		}
		return
	}

	// Verify user owns the agent
	if agent.UserID != user.ID {
		h.respondError(w, r, http.StatusForbidden, "Access denied", nil)
		return
	}

	if err := h.registryService.DeleteAgent(r.Context(), agentID); err != nil {
		h.respondError(w, r, http.StatusInternalServerError, "Failed to delete agent", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleRotateAgentKey rotates an agent's API key
func (h *Handler) HandleRotateAgentKey(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		h.respondError(w, r, http.StatusUnauthorized, "User not found in context", nil)
		return
	}

	agentID := chi.URLParam(r, "id")
	agent, err := h.registryService.GetAgent(r.Context(), agentID)
	if err != nil {
		if errors.Is(err, registry.ErrAgentNotFound) {
			h.respondError(w, r, http.StatusNotFound, "Agent not found", err)
		} else {
			h.respondError(w, r, http.StatusInternalServerError, "Failed to get agent", err)
		}
		return
	}

	// Verify user owns the agent
	if agent.UserID != user.ID {
		h.respondError(w, r, http.StatusForbidden, "Access denied", nil)
		return
	}

	newKey, err := h.registryService.RotateAPIKey(r.Context(), agentID)
	if err != nil {
		h.respondError(w, r, http.StatusInternalServerError, "Failed to rotate API key", err)
		return
	}

	h.respondJSON(w, r, http.StatusOK, map[string]string{"api_key": newKey})
}

// HandleListAlerts lists user's alerts
func (h *Handler) HandleListAlerts(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		h.respondError(w, r, http.StatusUnauthorized, "User not found in context", nil)
		return
	}

	// Parse query parameters
	limit := 50
	offset := 0
	agentID := r.URL.Query().Get("agent_id")

	var alerts []*storage.Alert
	var err error

	if agentID != "" {
		// Verify user owns the agent
		agent, agentErr := h.registryService.GetAgent(r.Context(), agentID)
		if agentErr != nil || agent.UserID != user.ID {
			h.respondError(w, r, http.StatusForbidden, "Access denied", nil)
			return
		}
		alerts, err = h.store.ListAlertsByAgent(r.Context(), agentID, limit, offset)
	} else {
		alerts, err = h.store.ListAlertsByUser(r.Context(), user.ID, limit, offset)
	}

	if err != nil {
		h.respondError(w, r, http.StatusInternalServerError, "Failed to list alerts", err)
		return
	}

	h.respondJSON(w, r, http.StatusOK, alerts)
}

// HandleGetAlert gets a specific alert
func (h *Handler) HandleGetAlert(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		h.respondError(w, r, http.StatusUnauthorized, "User not found in context", nil)
		return
	}

	alertID := chi.URLParam(r, "id")
	alert, err := h.store.GetAlert(r.Context(), alertID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			h.respondError(w, r, http.StatusNotFound, "Alert not found", err)
		} else {
			h.respondError(w, r, http.StatusInternalServerError, "Failed to get alert", err)
		}
		return
	}

	// Verify user owns the alert
	if alert.UserID != user.ID {
		h.respondError(w, r, http.StatusForbidden, "Access denied", nil)
		return
	}

	h.respondJSON(w, r, http.StatusOK, alert)
}

// HandleAcknowledgeAlert acknowledges an alert
func (h *Handler) HandleAcknowledgeAlert(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		h.respondError(w, r, http.StatusUnauthorized, "User not found in context", nil)
		return
	}

	alertID := chi.URLParam(r, "id")
	alert, err := h.store.GetAlert(r.Context(), alertID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			h.respondError(w, r, http.StatusNotFound, "Alert not found", err)
		} else {
			h.respondError(w, r, http.StatusInternalServerError, "Failed to get alert", err)
		}
		return
	}

	// Verify user owns the alert
	if alert.UserID != user.ID {
		h.respondError(w, r, http.StatusForbidden, "Access denied", nil)
		return
	}

	// Update alert
	alert.Acknowledged = true
	alert.AcknowledgedBy = user.Email
	now := time.Now()
	alert.AcknowledgedAt = &now

	if err := h.store.UpdateAlert(r.Context(), alert); err != nil {
		h.respondError(w, r, http.StatusInternalServerError, "Failed to acknowledge alert", err)
		return
	}

	h.respondJSON(w, r, http.StatusOK, alert)
}

// HandleGetSubscription gets user's subscription
func (h *Handler) HandleGetSubscription(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		h.respondError(w, r, http.StatusUnauthorized, "User not found in context", nil)
		return
	}

	subscription, err := h.store.GetSubscription(r.Context(), user.ID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			h.respondError(w, r, http.StatusNotFound, "Subscription not found", err)
		} else {
			h.respondError(w, r, http.StatusInternalServerError, "Failed to get subscription", err)
		}
		return
	}

	h.respondJSON(w, r, http.StatusOK, subscription)
}

// HandleUpgradeSubscription upgrades user's subscription
func (h *Handler) HandleUpgradeSubscription(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		h.respondError(w, r, http.StatusUnauthorized, "User not found in context", nil)
		return
	}

	var req struct {
		Plan string `json:"plan"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, r, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Validate plan
	validPlans := map[string]bool{"free": true, "pro": true, "enterprise": true}
	if !validPlans[req.Plan] {
		h.respondError(w, r, http.StatusBadRequest, "Invalid plan", nil)
		return
	}

	subscription, err := h.store.GetSubscription(r.Context(), user.ID)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		h.respondError(w, r, http.StatusInternalServerError, "Failed to get subscription", err)
		return
	}

	if subscription == nil {
		// Create new subscription
		subscription = &storage.Subscription{
			UserID:             user.ID,
			Plan:               req.Plan,
			Status:             "active",
			CurrentPeriodStart: time.Now(),
			CurrentPeriodEnd:   time.Now().AddDate(1, 0, 0), // 1 year
			CancelAtPeriodEnd:  false,
		}
	} else {
		// Update existing subscription
		subscription.Plan = req.Plan
		subscription.Status = "active"
		subscription.CurrentPeriodStart = time.Now()
		subscription.CurrentPeriodEnd = time.Now().AddDate(1, 0, 0)
		subscription.CancelAtPeriodEnd = false
	}

	if err := h.store.UpdateSubscription(r.Context(), subscription); err != nil {
		h.respondError(w, r, http.StatusInternalServerError, "Failed to update subscription", err)
		return
	}

	h.respondJSON(w, r, http.StatusOK, subscription)
}

// HandleCancelSubscription cancels user's subscription
func (h *Handler) HandleCancelSubscription(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		h.respondError(w, r, http.StatusUnauthorized, "User not found in context", nil)
		return
	}

	subscription, err := h.store.GetSubscription(r.Context(), user.ID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			h.respondError(w, r, http.StatusNotFound, "Subscription not found", err)
		} else {
			h.respondError(w, r, http.StatusInternalServerError, "Failed to get subscription", err)
		}
		return
	}

	subscription.CancelAtPeriodEnd = true
	subscription.Status = "canceled"

	if err := h.store.UpdateSubscription(r.Context(), subscription); err != nil {
		h.respondError(w, r, http.StatusInternalServerError, "Failed to cancel subscription", err)
		return
	}

	h.respondJSON(w, r, http.StatusOK, subscription)
}

// HandleWebSocket handles WebSocket connections
func (h *Handler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Determine connection type from query parameter
	connType := r.URL.Query().Get("type")
	if connType != "user" && connType != "agent" {
		h.respondError(w, r, http.StatusBadRequest, "Invalid connection type. Use 'user' or 'agent'", nil)
		return
	}

	// Get authentication token
	authToken := r.URL.Query().Get("token")
	if authToken == "" {
		// Try Authorization header for backward compatibility
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
			authToken = authHeader[7:]
		} else {
			h.respondError(w, r, http.StatusUnauthorized, "Authentication token required", nil)
			return
		}
	}

	// Handle WebSocket connection
	if err := h.routerService.HandleWebSocket(w, r, connType, authToken); err != nil {
		h.respondError(w, r, http.StatusInternalServerError, "WebSocket connection failed", err)
		return
	}
}

// HandleIngestAlert handles alert ingestion from agents
func (h *Handler) HandleIngestAlert(w http.ResponseWriter, r *http.Request) {
	// Authenticate via API key
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		h.respondError(w, r, http.StatusUnauthorized, "Authorization header required", nil)
		return
	}
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		h.respondError(w, r, http.StatusUnauthorized, "Invalid authorization format", nil)
		return
	}
	apiKey := parts[1]

	// Validate API key and get agent
	agent, err := h.registryService.ValidateAgent(r.Context(), apiKey)
	if err != nil {
		if errors.Is(err, registry.ErrInvalidAPIKey) {
			h.respondError(w, r, http.StatusUnauthorized, "Invalid API key", err)
		} else if errors.Is(err, registry.ErrAgentOffline) {
			h.respondError(w, r, http.StatusBadRequest, "Agent offline", err)
		} else {
			h.respondError(w, r, http.StatusInternalServerError, "Failed to validate agent", err)
		}
		return
	}

	// Parse alert payload (matches watcher's cloud action format)
	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		h.respondError(w, r, http.StatusBadRequest, "Invalid alert payload", err)
		return
	}

	// Build alert from payload
	alert := &storage.Alert{
		ID:           uuid.New().String(),
		AgentID:      agent.ID,
		UserID:       agent.UserID,
		RuleName:     getString(payload, "rule_name"),
		Severity:     getString(payload, "severity"),
		Message:      getString(payload, "message"),
		ResourceKind: getString(payload, "resource_kind"),
		Namespace:    getString(payload, "namespace"),
		ResourceName: getString(payload, "resource_name"),
		Details:      getMap(payload, "details"),
		Acknowledged: false,
	}

	// Parse timestamp if provided
	if ts, ok := payload["timestamp"].(string); ok {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			alert.Timestamp = t
		} else {
			alert.Timestamp = time.Now()
		}
	} else {
		alert.Timestamp = time.Now()
	}

	// Store alert
	if err := h.store.CreateAlert(r.Context(), alert); err != nil {
		h.respondError(w, r, http.StatusInternalServerError, "Failed to store alert", err)
		return
	}

	// Notify user via WebSocket if connected
	message := router.Message{
		Type: "alert",
		Payload: json.RawMessage(fmt.Sprintf(`{"id": "%s", "rule_name": "%s", "severity": "%s", "message": "%s"}`,
			alert.ID, alert.RuleName, alert.Severity, alert.Message)),
	}
	_ = h.routerService.SendToUser(agent.UserID, message)

	h.respondJSON(w, r, http.StatusCreated, map[string]string{"id": alert.ID})
}

// Helper functions for extracting values from map
func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getMap(m map[string]interface{}, key string) map[string]interface{} {
	if val, ok := m[key]; ok {
		if mp, ok := val.(map[string]interface{}); ok {
			return mp
		}
	}
	return nil
}

// respondJSON sends a JSON response
func (h *Handler) respondJSON(w http.ResponseWriter, r *http.Request, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		// Log error but can't change response now
		fmt.Printf("Failed to encode JSON response: %v\n", err)
	}
}

// respondError sends an error response
func (h *Handler) respondError(w http.ResponseWriter, r *http.Request, status int, message string, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	errorResp := map[string]string{
		"error": message,
	}
	if err != nil {
		errorResp["details"] = err.Error()
	}

	if err := json.NewEncoder(w).Encode(errorResp); err != nil {
		fmt.Printf("Failed to encode error response: %v\n", err)
	}
}
