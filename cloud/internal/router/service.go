package router

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"kube-watcher/cloud/internal/storage"
)

var (
	ErrConnectionNotFound = errors.New("connection not found")
	ErrInvalidMessage     = errors.New("invalid message")
	ErrNotAuthorized      = errors.New("not authorized")
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// In production, implement proper origin checking
		return true
	},
}

// Message represents a WebSocket message
type Message struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
	Target  string          `json:"target,omitempty"` // For targeted messages
	Source  string          `json:"source,omitempty"` // Source connection ID
}

// Connection represents a WebSocket connection
type Connection struct {
	ID         string
	Type       string // "agent" or "user"
	UserID     string
	AgentID    string // For agent connections
	Connection *websocket.Conn
	Send       chan Message
	mu         sync.Mutex
}

// Service handles WebSocket connections and message routing
type Service struct {
	store storage.Store

	mu          sync.RWMutex
	connections map[string]*Connection
	userConns   map[string][]string // userID -> connection IDs
	agentConns  map[string]string   // agentID -> connection ID
}

// NewService creates a new router service
func NewService(store storage.Store) *Service {
	return &Service{
		store:       store,
		connections: make(map[string]*Connection),
		userConns:   make(map[string][]string),
		agentConns:  make(map[string]string),
	}
}

// HandleWebSocket handles incoming WebSocket connections
func (s *Service) HandleWebSocket(w http.ResponseWriter, r *http.Request, connType, authToken string) error {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return fmt.Errorf("failed to upgrade connection: %w", err)
	}

	// Authenticate based on connection type
	var userID, agentID string
	switch connType {
	case "user":
		// User authentication (JWT token)
		// TODO: Validate JWT token and get user ID
		userID = authToken // For now, use token as user ID
	case "agent":
		// Agent authentication (API key)
		agent, err := s.store.GetAgentByAPIKey(r.Context(), authToken)
		if err != nil {
			_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "invalid API key"))
			conn.Close()
			return ErrNotAuthorized
		}
		userID = agent.UserID
		agentID = agent.ID
	default:
		_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "invalid connection type"))
		conn.Close()
		return errors.New("invalid connection type")
	}

	// Create connection
	connectionID := fmt.Sprintf("%s-%s-%d", connType, userID, time.Now().UnixNano())
	connection := &Connection{
		ID:         connectionID,
		Type:       connType,
		UserID:     userID,
		AgentID:    agentID,
		Connection: conn,
		Send:       make(chan Message, 256),
	}

	// Register connection
	s.registerConnection(connection)

	// Start goroutines to handle connection
	go s.writePump(connection)
	go s.readPump(connection)

	log.Printf("WebSocket connection established: %s (type: %s, user: %s)", connectionID, connType, userID)
	return nil
}

// registerConnection registers a new connection
func (s *Service) registerConnection(conn *Connection) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.connections[conn.ID] = conn

	if conn.Type == "user" {
		s.userConns[conn.UserID] = append(s.userConns[conn.UserID], conn.ID)
	} else if conn.Type == "agent" && conn.AgentID != "" {
		s.agentConns[conn.AgentID] = conn.ID
	}
}

// unregisterConnection removes a connection
func (s *Service) unregisterConnection(conn *Connection) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.connections, conn.ID)

	if conn.Type == "user" {
		conns := s.userConns[conn.UserID]
		for i, id := range conns {
			if id == conn.ID {
				s.userConns[conn.UserID] = append(conns[:i], conns[i+1:]...)
				break
			}
		}
		if len(s.userConns[conn.UserID]) == 0 {
			delete(s.userConns, conn.UserID)
		}
	} else if conn.Type == "agent" && conn.AgentID != "" {
		delete(s.agentConns, conn.AgentID)
	}
}

// writePump sends messages to the WebSocket connection
func (s *Service) writePump(conn *Connection) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		conn.Connection.Close()
	}()

	for {
		select {
		case message, ok := <-conn.Send:
			if !ok {
				// Channel closed
				_ = conn.Connection.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			conn.mu.Lock()
			err := conn.Connection.WriteJSON(message)
			conn.mu.Unlock()
			if err != nil {
				log.Printf("Failed to write message to connection %s: %v", conn.ID, err)
				return
			}
		case <-ticker.C:
			// Send ping to keep connection alive
			conn.mu.Lock()
			err := conn.Connection.WriteMessage(websocket.PingMessage, nil)
			conn.mu.Unlock()
			if err != nil {
				log.Printf("Failed to send ping to connection %s: %v", conn.ID, err)
				return
			}
		}
	}
}

// readPump receives messages from the WebSocket connection
func (s *Service) readPump(conn *Connection) {
	defer func() {
		s.unregisterConnection(conn)
		conn.Connection.Close()
		close(conn.Send)
	}()

	conn.Connection.SetReadLimit(512 * 1024) // 512KB
	_ = conn.Connection.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.Connection.SetPongHandler(func(string) error {
		_ = conn.Connection.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		var message Message
		err := conn.Connection.ReadJSON(&message)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Process message
		message.Source = conn.ID
		if err := s.handleMessage(conn, message); err != nil {
			log.Printf("Failed to handle message from connection %s: %v", conn.ID, err)
			// Send error back to sender
			errorMsg := Message{
				Type:    "error",
				Payload: json.RawMessage(fmt.Sprintf(`{"error": "%s"}`, err.Error())),
			}
			conn.Send <- errorMsg
		}

		_ = conn.Connection.SetReadDeadline(time.Now().Add(60 * time.Second))
	}
}

// handleMessage processes incoming messages
func (s *Service) handleMessage(conn *Connection, message Message) error {
	switch message.Type {
	case "ping":
		// Respond with pong
		conn.Send <- Message{Type: "pong"}
	case "alert":
		// Alert from agent, route to user
		if conn.Type != "agent" {
			return errors.New("only agents can send alerts")
		}
		return s.routeAlertToUser(conn, message)
	case "command":
		// Command from user, route to agent
		if conn.Type != "user" {
			return errors.New("only users can send commands")
		}
		return s.routeCommandToAgent(conn, message)
	case "subscribe":
		// Subscription request
		return s.handleSubscribe(conn, message)
	case "unsubscribe":
		// Unsubscribe request
		return s.handleUnsubscribe(conn, message)
	default:
		return fmt.Errorf("%w: unknown message type %s", ErrInvalidMessage, message.Type)
	}
	return nil
}

// routeAlertToUser routes an alert from an agent to the user
func (s *Service) routeAlertToUser(conn *Connection, message Message) error {
	// Get user connections
	s.mu.RLock()
	userConnIDs, ok := s.userConns[conn.UserID]
	s.mu.RUnlock()

	if !ok {
		// No user connections, store alert for later delivery
		// In production, we'd store in database
		return nil
	}

	// Send to all user connections
	for _, connID := range userConnIDs {
		s.mu.RLock()
		userConn, ok := s.connections[connID]
		s.mu.RUnlock()

		if ok {
			userConn.Send <- message
		}
	}

	return nil
}

// routeCommandToAgent routes a command from a user to an agent
func (s *Service) routeCommandToAgent(conn *Connection, message Message) error {
	// Parse target agent ID from message
	var target struct {
		AgentID string `json:"agent_id"`
	}
	if err := json.Unmarshal(message.Payload, &target); err != nil {
		return fmt.Errorf("%w: failed to parse agent ID: %v", ErrInvalidMessage, err)
	}

	// Verify user owns the agent
	agent, err := s.store.GetAgent(context.Background(), target.AgentID)
	if err != nil {
		return fmt.Errorf("agent not found: %w", err)
	}

	if agent.UserID != conn.UserID {
		return ErrNotAuthorized
	}

	// Find agent connection
	s.mu.RLock()
	agentConnID, ok := s.agentConns[target.AgentID]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("agent %s is not connected", target.AgentID)
	}

	// Send command to agent
	s.mu.RLock()
	agentConn, ok := s.connections[agentConnID]
	s.mu.RUnlock()

	if ok {
		agentConn.Send <- message
	} else {
		return ErrConnectionNotFound
	}

	return nil
}

// handleSubscribe handles subscription requests
func (s *Service) handleSubscribe(conn *Connection, message Message) error {
	// Parse subscription request
	var sub struct {
		Channel string `json:"channel"`
	}
	if err := json.Unmarshal(message.Payload, &sub); err != nil {
		return fmt.Errorf("%w: failed to parse subscription: %v", ErrInvalidMessage, err)
	}

	// For now, just acknowledge subscription
	// In production, we'd track subscriptions per connection
	conn.Send <- Message{
		Type:    "subscribed",
		Payload: json.RawMessage(fmt.Sprintf(`{"channel": "%s"}`, sub.Channel)),
	}

	return nil
}

// handleUnsubscribe handles unsubscribe requests
func (s *Service) handleUnsubscribe(conn *Connection, message Message) error {
	// Parse unsubscribe request
	var unsub struct {
		Channel string `json:"channel"`
	}
	if err := json.Unmarshal(message.Payload, &unsub); err != nil {
		return fmt.Errorf("%w: failed to parse unsubscribe: %v", ErrInvalidMessage, err)
	}

	// For now, just acknowledge unsubscribe
	conn.Send <- Message{
		Type:    "unsubscribed",
		Payload: json.RawMessage(fmt.Sprintf(`{"channel": "%s"}`, unsub.Channel)),
	}

	return nil
}

// SendToUser sends a message to all connections for a user
func (s *Service) SendToUser(userID string, message Message) error {
	s.mu.RLock()
	connIDs, ok := s.userConns[userID]
	s.mu.RUnlock()

	if !ok {
		return ErrConnectionNotFound
	}

	for _, connID := range connIDs {
		s.mu.RLock()
		conn, ok := s.connections[connID]
		s.mu.RUnlock()

		if ok {
			conn.Send <- message
		}
	}

	return nil
}

// SendToAgent sends a message to an agent
func (s *Service) SendToAgent(agentID string, message Message) error {
	s.mu.RLock()
	connID, ok := s.agentConns[agentID]
	s.mu.RUnlock()

	if !ok {
		return ErrConnectionNotFound
	}

	s.mu.RLock()
	conn, ok := s.connections[connID]
	s.mu.RUnlock()

	if !ok {
		return ErrConnectionNotFound
	}

	conn.Send <- message
	return nil
}

// Broadcast sends a message to all connections of a specific type
func (s *Service) Broadcast(connType string, message Message) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, conn := range s.connections {
		if conn.Type == connType {
			select {
			case conn.Send <- message:
			default:
				// Channel full, skip
				log.Printf("Channel full for connection %s, dropping message", conn.ID)
			}
		}
	}
}

// GetConnectionStats returns connection statistics
func (s *Service) GetConnectionStats() map[string]int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := make(map[string]int)
	stats["total"] = len(s.connections)
	stats["users"] = len(s.userConns)
	stats["agents"] = len(s.agentConns)

	// Count connections by type
	typeCounts := make(map[string]int)
	for _, conn := range s.connections {
		typeCounts[conn.Type]++
	}
	stats["user_connections"] = typeCounts["user"]
	stats["agent_connections"] = typeCounts["agent"]

	return stats
}

// CloseConnection closes a specific connection
func (s *Service) CloseConnection(connectionID string) error {
	s.mu.RLock()
	conn, ok := s.connections[connectionID]
	s.mu.RUnlock()

	if !ok {
		return ErrConnectionNotFound
	}

	conn.mu.Lock()
	err := conn.Connection.Close()
	conn.mu.Unlock()

	s.unregisterConnection(conn)
	return err
}

// CloseAllConnections closes all connections
func (s *Service) CloseAllConnections() {
	s.mu.RLock()
	connections := make([]*Connection, 0, len(s.connections))
	for _, conn := range s.connections {
		connections = append(connections, conn)
	}
	s.mu.RUnlock()

	for _, conn := range connections {
		conn.mu.Lock()
		conn.Connection.Close()
		conn.mu.Unlock()
		s.unregisterConnection(conn)
	}
}
