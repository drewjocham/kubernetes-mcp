package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"cloud.google.com/go/pubsub" //nolint:staticcheck // SA1019: v1 client; migrate to pubsub/v2 when subscription admin is refactored.
	_ "github.com/marcboeker/go-duckdb"

	"kube-watcher/pkg/agenttelemetry"
)

var (
	db *sql.DB
)

type TelemetryPayload struct {
	AgentID   string                 `json:"agent_id"`
	UserID    string                 `json:"user_id"`
	TenantID  string                 `json:"tenant_id"`
	Timestamp int64                  `json:"timestamp"`
	Metrics   map[string]interface{} `json:"metrics"`
	Events    []string               `json:"events"`
}

type Alert struct {
	UserID    string                 `json:"user_id"`
	TenantID  string                 `json:"tenant_id"`
	AgentID   string                 `json:"agent_id"`
	Timestamp time.Time              `json:"timestamp"`
	Severity  string                 `json:"severity"`
	Message   string                 `json:"message"`
	Metrics   map[string]interface{} `json:"metrics,omitempty"`
}

func initDuckDB() error {
	duckdbPath := os.Getenv("ANOMSTACK_DUCKDB_PATH")
	if duckdbPath == "" {
		duckdbPath = "/data/anomstack.db"
	}
	connStr := duckdbPath
	var err error
	db, err = sql.Open("duckdb", connStr)
	if err != nil {
		return fmt.Errorf("failed to open duckdb: %w", err)
	}
	// Create table if not exists (matching anomstack schema)
	createTableSQL := `
CREATE TABLE IF NOT EXISTS metrics (
    metric_timestamp TIMESTAMP,
    metric_batch TEXT,
    metric_name TEXT,
    metric_value DOUBLE,
    metric_type TEXT,
    metadata TEXT
);`
	_, err = db.Exec(createTableSQL)
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}
	// Create user webhooks table
	_, err = db.Exec(`
CREATE TABLE IF NOT EXISTS user_webhooks (
    user_id TEXT PRIMARY KEY,
    webhook_url TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);`)
	if err != nil {
		return fmt.Errorf("failed to create user_webhooks table: %w", err)
	}
	// Create sent alerts table to avoid duplicate webhook sends
	_, err = db.Exec(`
CREATE TABLE IF NOT EXISTS sent_alerts (
    id UUID DEFAULT uuid(),
    user_id TEXT,
    metric_name TEXT,
    metric_timestamp TIMESTAMP,
    score DOUBLE,
    sent_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    hash TEXT UNIQUE,
    PRIMARY KEY (id)
);`)
	if err != nil {
		return fmt.Errorf("failed to create sent_alerts table: %w", err)
	}
	log.Printf("DuckDB initialized at %s", duckdbPath)
	return nil
}

func saveMetricsToDuckDB(payload TelemetryPayload) error {
	if db == nil {
		return fmt.Errorf("duckdb not initialized")
	}
	// Prepare insert statement
	stmt, err := db.Prepare(`INSERT INTO metrics (metric_timestamp, metric_batch, metric_name, metric_value, metric_type, metadata) 
VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer func() {
		if cerr := stmt.Close(); cerr != nil {
			log.Printf("duckdb stmt close: %v", cerr)
		}
	}()

	metricBatch := "system_metrics"
	metricType := "metric"
	// Convert timestamp from Unix seconds to time.Time
	ts := time.Unix(payload.Timestamp, 0)
	// Metadata as JSON
	metadata := fmt.Sprintf(`{"user_id":"%s","tenant_id":"%s","agent_id":"%s"}`,
		payload.UserID, payload.TenantID, payload.AgentID)

	// Insert each metric
	for metricName, val := range payload.Metrics {
		var metricValue float64
		switch v := val.(type) {
		case float64:
			metricValue = v
		case int:
			metricValue = float64(v)
		case int64:
			metricValue = float64(v)
		case float32:
			metricValue = float64(v)
		default:
			// Skip non-numeric metrics
			continue
		}
		// Prepend user_id to metric_name to separate per user
		fullMetricName := fmt.Sprintf("user_%s_%s", payload.UserID, metricName)
		_, err := stmt.Exec(ts, metricBatch, fullMetricName, metricValue, metricType, metadata)
		if err != nil {
			log.Printf("Failed to insert metric %s: %v", fullMetricName, err)
		}
	}
	return nil
}

func checkAnomaliesAndSendWebhooks() error {
	if db == nil {
		return fmt.Errorf("duckdb not initialized")
	}
	// Query for recent scores above threshold
	threshold := 0.8
	rows, err := db.Query(`
		SELECT metric_timestamp, metric_name, metric_value, metadata
		FROM metrics
		WHERE metric_type = 'score'
		AND metric_batch = 'system_metrics'
		AND metric_value > ?
		AND metric_timestamp > CURRENT_TIMESTAMP - INTERVAL '1 hour'
	`, threshold)
	if err != nil {
		return fmt.Errorf("failed to query anomalies: %w", err)
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			log.Printf("duckdb rows close: %v", cerr)
		}
	}()

	for rows.Next() {
		var metricTimestamp time.Time
		var metricName, metadata string
		var score float64
		if err := rows.Scan(&metricTimestamp, &metricName, &score, &metadata); err != nil {
			log.Printf("Failed to scan row: %v", err)
			continue
		}
		// Parse user_id from metric_name prefix "user_{user_id}_"
		// Expect format "user_123_cpu_usage_percent"
		parts := strings.Split(metricName, "_")
		if len(parts) < 3 || parts[0] != "user" {
			log.Printf("Invalid metric name format: %s", metricName)
			continue
		}
		userID := parts[1]
		// Look up webhook URL
		var webhookURL string
		err := db.QueryRow("SELECT webhook_url FROM user_webhooks WHERE user_id = ?", userID).Scan(&webhookURL)
		if err != nil {
			log.Printf("No webhook found for user %s: %v", userID, err)
			continue
		}
		// Generate hash to avoid duplicate alerts
		hash := fmt.Sprintf("%s|%s|%d", userID, metricName, metricTimestamp.Unix())
		var existingHash string
		err = db.QueryRow("SELECT hash FROM sent_alerts WHERE hash = ?", hash).Scan(&existingHash)
		if err == nil {
			// Already sent
			continue
		}
		// Send webhook
		payload := map[string]interface{}{
			"user_id":          userID,
			"metric_name":      metricName,
			"metric_timestamp": metricTimestamp,
			"score":            score,
			"metadata":         metadata,
		}
		jsonData, _ := json.Marshal(payload)
		resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			log.Printf("Failed to send webhook for user %s: %v", userID, err)
			continue
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			// Record sent alert
			_, err = db.Exec("INSERT INTO sent_alerts (user_id, metric_name, metric_timestamp, score, hash) VALUES (?, ?, ?, ?, ?)",
				userID, metricName, metricTimestamp, score, hash)
			if err != nil {
				log.Printf("Failed to record sent alert: %v", err)
			}
			log.Printf("Sent webhook for user %s metric %s score %.2f", userID, metricName, score)
		} else {
			log.Printf("Webhook returned non-OK status: %d", resp.StatusCode)
		}
	}
	return nil
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if projectID == "" {
		log.Fatal("GOOGLE_CLOUD_PROJECT environment variable is required")
	}

	// Initialize DuckDB for metric storage
	if os.Getenv("ANOMSTACK_DUCKDB_PATH") != "" {
		if err := initDuckDB(); err != nil {
			log.Printf("Failed to initialize DuckDB: %v (continuing without DuckDB)", err)
		} else {
			defer func() {
				if cerr := db.Close(); cerr != nil {
					log.Printf("duckdb close: %v", cerr)
				}
			}()
			// Start periodic anomaly check
			go func() {
				ticker := time.NewTicker(1 * time.Minute)
				defer ticker.Stop()
				for {
					select {
					case <-ticker.C:
						if err := checkAnomaliesAndSendWebhooks(); err != nil {
							log.Printf("Anomaly check failed: %v", err)
						}
					case <-ctx.Done():
						return
					}
				}
			}()
		}
	}

	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		log.Fatalf("Failed to create Pub/Sub client: %v", err)
	}
	defer func() {
		if cerr := client.Close(); cerr != nil {
			log.Printf("pubsub client close: %v", cerr)
		}
	}()

	subscriptionID := "agent-telemetry-sub"
	sub := client.Subscription(subscriptionID)

	// Create subscription if it doesn't exist
	exists, err := sub.Exists(ctx)
	if err != nil {
		log.Fatalf("Failed to check subscription existence: %v", err)
	}
	if !exists {
		topic := client.Topic("agent-telemetry-topic")
		sub, err = client.CreateSubscription(ctx, subscriptionID, pubsub.SubscriptionConfig{
			Topic:             topic,
			AckDeadline:       30 * time.Second,
			RetentionDuration: 7 * 24 * time.Hour,
		})
		if err != nil {
			log.Fatalf("Failed to create subscription: %v", err)
		}
		log.Printf("Created subscription %s", subscriptionID)
	}

	log.Printf("Starting processor, listening to subscription %s", subscriptionID)

	// Handle shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sig
		log.Println("Shutdown signal received")
		cancel()
	}()

	// Start receiving messages
	err = sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
		var wire struct {
			AgentID   string          `json:"agent_id"`
			UserID    string          `json:"user_id"`
			TenantID  string          `json:"tenant_id"`
			Timestamp int64           `json:"timestamp"`
			Metrics   json.RawMessage `json:"metrics"`
			Events    []string        `json:"events"`
		}
		if err := json.Unmarshal(msg.Data, &wire); err != nil {
			log.Printf("Failed to unmarshal message: %v", err)
			msg.Nack()
			return
		}

		payload := TelemetryPayload{
			AgentID:   wire.AgentID,
			UserID:    wire.UserID,
			TenantID:  wire.TenantID,
			Timestamp: wire.Timestamp,
			Metrics:   agenttelemetry.NormalizeMetricsJSON(wire.Metrics),
			Events:    wire.Events,
		}

		log.Printf("Received telemetry: agent=%s, user=%s, tenant=%s",
			payload.AgentID, payload.UserID, payload.TenantID)

		// Save metrics to DuckDB if enabled
		if db != nil {
			if err := saveMetricsToDuckDB(payload); err != nil {
				log.Printf("Failed to save metrics to DuckDB: %v", err)
			}
		}

		// Process for anomalies
		alerts := detectAnomalies(payload)

		// Send alerts to user webhooks
		for _, alert := range alerts {
			sendAlert(alert)
		}

		msg.Ack()
	})

	if err != nil && err != context.Canceled {
		log.Fatalf("Receive error: %v", err)
	}

	log.Println("Processor shutdown complete")
}

func detectAnomalies(payload TelemetryPayload) []Alert {
	var alerts []Alert

	// Simple threshold-based anomaly detection (supports map metrics and normalized probe arrays)
	cpu, ok := numericFromMap(payload.Metrics, "cpu_used_percent")
	if !ok {
		cpu, ok = maxNumericForKeyPrefix(payload.Metrics, "cpu_usage_percent")
	}
	if ok && cpu > 80.0 {
		alerts = append(alerts, Alert{
			UserID:    payload.UserID,
			TenantID:  payload.TenantID,
			AgentID:   payload.AgentID,
			Timestamp: time.Now(),
			Severity:  "warning",
			Message:   fmt.Sprintf("High CPU usage: %.1f%%", cpu),
			Metrics:   map[string]interface{}{"cpu_used_percent": cpu},
		})
	}

	mem, ok := numericFromMap(payload.Metrics, "memory_used_percent")
	if ok {
		if mem > 90.0 {
			alerts = append(alerts, Alert{
				UserID:    payload.UserID,
				TenantID:  payload.TenantID,
				AgentID:   payload.AgentID,
				Timestamp: time.Now(),
				Severity:  "warning",
				Message:   fmt.Sprintf("High memory usage: %.1f%%", mem),
				Metrics:   map[string]interface{}{"memory_used_percent": mem},
			})
		}
	}

	// Check for container events
	if len(payload.Events) > 0 {
		for _, event := range payload.Events {
			alerts = append(alerts, Alert{
				UserID:    payload.UserID,
				TenantID:  payload.TenantID,
				AgentID:   payload.AgentID,
				Timestamp: time.Now(),
				Severity:  "info",
				Message:   fmt.Sprintf("Container event: %s", event),
			})
		}
	}

	return alerts
}

func sendAlert(alert Alert) {
	// TODO: Look up user's webhook URL from configuration store
	// For now, just log the alert
	log.Printf("ALERT: user=%s, severity=%s, message=%s",
		alert.UserID, alert.Severity, alert.Message)

	// Example webhook integration: look up URL from DuckDB user_webhooks and POST (see checkAnomaliesAndSendWebhooks).
}

func numericFromMap(m map[string]interface{}, key string) (float64, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	default:
		return 0, false
	}
}

func maxNumericForKeyPrefix(m map[string]interface{}, prefix string) (float64, bool) {
	var max float64
	var found bool
	for k, v := range m {
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		switch x := v.(type) {
		case float64:
			if !found || x > max {
				max = x
				found = true
			}
		case float32:
			f := float64(x)
			if !found || f > max {
				max = f
				found = true
			}
		case int:
			f := float64(x)
			if !found || f > max {
				max = f
				found = true
			}
		case int64:
			f := float64(x)
			if !found || f > max {
				max = f
				found = true
			}
		}
	}
	return max, found
}
