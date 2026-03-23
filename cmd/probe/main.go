package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

type Metric struct {
	Name      string            `json:"name"`
	Value     float64           `json:"value"`
	Timestamp int64             `json:"timestamp"`
	Labels    map[string]string `json:"labels,omitempty"`
	Type      string            `json:"type,omitempty"` // gauge, counter, histogram
}

type TelemetryPayload struct {
	AgentID   string   `json:"agent_id"`
	UserID    string   `json:"user_id"`   // Multi-tenant user identity
	TenantID  string   `json:"tenant_id"` // Optional tenant/organization
	Timestamp int64    `json:"timestamp"`
	Metrics   []Metric `json:"metrics"`
	Events    []string `json:"events"`
}

type Config struct {
	APIEndpoint  string
	AuthToken    string
	AgentID      string
	UserID       string
	TenantID     string
	ScanInterval time.Duration
}

func main() {
	cfg := Config{
		APIEndpoint:  getEnv("API_ENDPOINT", "http://localhost:8080/ingest"),
		AuthToken:    getEnv("AUTH_TOKEN", ""),
		AgentID:      getEnv("AGENT_ID", "unknown"),
		UserID:       getEnv("USER_ID", ""),
		TenantID:     getEnv("TENANT_ID", ""),
		ScanInterval: parseDuration(getEnv("SCAN_INTERVAL", "10s")),
	}

	if cfg.AuthToken == "" {
		log.Fatal("AUTH_TOKEN environment variable is required")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Printf("Failed to create Docker client (continuing without Docker monitoring): %v", err)
		dockerClient = nil
	} else {
		defer dockerClient.Close()
	}

	ticker := time.NewTicker(cfg.ScanInterval)
	defer ticker.Stop()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	log.Printf("Starting agent probe: ID=%s, endpoint=%s, interval=%v", cfg.AgentID, cfg.APIEndpoint, cfg.ScanInterval)

	for {
		select {
		case <-ticker.C:
			if err := collectAndSend(ctx, cfg, dockerClient); err != nil {
				log.Printf("Error during collection/send: %v", err)
			}
		case <-sig:
			log.Println("Shutdown signal received")
			return
		}
	}
}

func collectAndSend(ctx context.Context, cfg Config, dockerClient *client.Client) error {
	metrics := make(map[string]interface{})
	events := []string{}

	// Host metrics
	if v, err := mem.VirtualMemory(); err == nil {
		metrics["memory_used_percent"] = v.UsedPercent
		metrics["memory_total_bytes"] = v.Total
		metrics["memory_available_bytes"] = v.Available
	} else {
		log.Printf("Failed to get memory stats: %v", err)
	}

	if c, err := cpu.Percent(0, false); err == nil && len(c) > 0 {
		metrics["cpu_used_percent"] = c[0]
	} else {
		log.Printf("Failed to get CPU stats: %v", err)
	}

	// Docker container status
	if dockerClient != nil {
		containers, err := dockerClient.ContainerList(ctx, container.ListOptions{
			All:     true,
			Filters: filters.NewArgs(filters.Arg("status", "running")),
		})
		if err != nil {
			log.Printf("Failed to list Docker containers: %v", err)
		} else {
			metrics["docker_running_containers"] = len(containers)
			// Optionally list container names
			var names []string
			for _, c := range containers {
				if len(c.Names) > 0 {
					names = append(names, c.Names[0])
				}
			}
			metrics["docker_container_names"] = names
		}

		// Check for exited containers (events)
		exited, err := dockerClient.ContainerList(ctx, container.ListOptions{
			All:     true,
			Filters: filters.NewArgs(filters.Arg("status", "exited")),
		})
		if err == nil && len(exited) > 0 {
			events = append(events, fmt.Sprintf("%d containers exited", len(exited)))
		}
	}

	payload := TelemetryPayload{
		AgentID:   cfg.AgentID,
		UserID:    cfg.UserID,
		TenantID:  cfg.TenantID,
		Timestamp: time.Now().Unix(),
		Metrics:   metrics,
		Events:    events,
	}

	return sendPayload(cfg, payload)
}

func sendPayload(cfg Config, payload TelemetryPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", cfg.APIEndpoint, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Agent-Token", cfg.AuthToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	log.Printf("Telemetry sent successfully")
	return nil
}

func getEnv(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}

func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		log.Printf("Invalid duration %s, defaulting to 10s: %v", s, err)
		return 10 * time.Second
	}
	return d
}
