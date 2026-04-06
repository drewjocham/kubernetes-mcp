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
	"strings"
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
		defer func() {
			if err := dockerClient.Close(); err != nil {
				log.Printf("docker client close: %v", err)
			}
		}()
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
	metrics := []Metric{}
	events := []string{}
	timestamp := time.Now().Unix()

	// CPU metrics
	if cpuStats, err := cpu.Percent(0, true); err == nil {
		for i, percent := range cpuStats {
			metrics = append(metrics, Metric{
				Name:      "cpu_usage_percent",
				Value:     percent,
				Timestamp: timestamp,
				Labels:    map[string]string{"cpu": fmt.Sprintf("cpu%d", i)},
				Type:      "gauge",
			})
		}
	} else {
		log.Printf("Failed to get CPU stats: %v", err)
	}

	// CPU times
	if cpuTimes, err := cpu.Times(true); err == nil {
		for _, ct := range cpuTimes {
			labels := map[string]string{"cpu": ct.CPU}
			metrics = append(metrics,
				Metric{Name: "cpu_user_seconds", Value: ct.User, Timestamp: timestamp, Labels: labels, Type: "counter"},
				Metric{Name: "cpu_system_seconds", Value: ct.System, Timestamp: timestamp, Labels: labels, Type: "counter"},
				Metric{Name: "cpu_idle_seconds", Value: ct.Idle, Timestamp: timestamp, Labels: labels, Type: "counter"},
				Metric{Name: "cpu_iowait_seconds", Value: ct.Iowait, Timestamp: timestamp, Labels: labels, Type: "counter"},
			)
		}
	}

	// Memory metrics
	if memInfo, err := mem.VirtualMemory(); err == nil {
		metrics = append(metrics,
			Metric{Name: "memory_used_bytes", Value: float64(memInfo.Used), Timestamp: timestamp, Type: "gauge"},
			Metric{Name: "memory_available_bytes", Value: float64(memInfo.Available), Timestamp: timestamp, Type: "gauge"},
			Metric{Name: "memory_total_bytes", Value: float64(memInfo.Total), Timestamp: timestamp, Type: "gauge"},
			Metric{Name: "memory_used_percent", Value: memInfo.UsedPercent, Timestamp: timestamp, Type: "gauge"},
			Metric{Name: "memory_free_bytes", Value: float64(memInfo.Free), Timestamp: timestamp, Type: "gauge"},
			Metric{Name: "memory_cached_bytes", Value: float64(memInfo.Cached), Timestamp: timestamp, Type: "gauge"},
		)
	} else {
		log.Printf("Failed to get memory stats: %v", err)
	}

	// Swap memory
	if swapInfo, err := mem.SwapMemory(); err == nil {
		metrics = append(metrics,
			Metric{Name: "swap_used_bytes", Value: float64(swapInfo.Used), Timestamp: timestamp, Type: "gauge"},
			Metric{Name: "swap_free_bytes", Value: float64(swapInfo.Free), Timestamp: timestamp, Type: "gauge"},
			Metric{Name: "swap_total_bytes", Value: float64(swapInfo.Total), Timestamp: timestamp, Type: "gauge"},
			Metric{Name: "swap_used_percent", Value: swapInfo.UsedPercent, Timestamp: timestamp, Type: "gauge"},
		)
	}

	// Disk metrics
	if partitions, err := disk.Partitions(false); err == nil {
		for _, partition := range partitions {
			if usage, err := disk.Usage(partition.Mountpoint); err == nil {
				labels := map[string]string{
					"device": partition.Device,
					"fstype": partition.Fstype,
					"mount":  partition.Mountpoint,
				}
				metrics = append(metrics,
					Metric{Name: "disk_total_bytes", Value: float64(usage.Total), Timestamp: timestamp, Labels: labels, Type: "gauge"},
					Metric{Name: "disk_used_bytes", Value: float64(usage.Used), Timestamp: timestamp, Labels: labels, Type: "gauge"},
					Metric{Name: "disk_free_bytes", Value: float64(usage.Free), Timestamp: timestamp, Labels: labels, Type: "gauge"},
					Metric{Name: "disk_used_percent", Value: usage.UsedPercent, Timestamp: timestamp, Labels: labels, Type: "gauge"},
				)
			}
		}
	}

	// Disk I/O
	if ioCounters, err := disk.IOCounters(); err == nil {
		for device, io := range ioCounters {
			labels := map[string]string{"device": device}
			metrics = append(metrics,
				Metric{Name: "disk_read_bytes", Value: float64(io.ReadBytes), Timestamp: timestamp, Labels: labels, Type: "counter"},
				Metric{Name: "disk_write_bytes", Value: float64(io.WriteBytes), Timestamp: timestamp, Labels: labels, Type: "counter"},
				Metric{Name: "disk_read_count", Value: float64(io.ReadCount), Timestamp: timestamp, Labels: labels, Type: "counter"},
				Metric{Name: "disk_write_count", Value: float64(io.WriteCount), Timestamp: timestamp, Labels: labels, Type: "counter"},
			)
		}
	}

	// Network metrics
	if netIO, err := net.IOCounters(true); err == nil {
		for _, io := range netIO {
			if io.Name == "lo" { // Skip loopback
				continue
			}
			labels := map[string]string{"interface": io.Name}
			metrics = append(metrics,
				Metric{Name: "network_receive_bytes", Value: float64(io.BytesRecv), Timestamp: timestamp, Labels: labels, Type: "counter"},
				Metric{Name: "network_transmit_bytes", Value: float64(io.BytesSent), Timestamp: timestamp, Labels: labels, Type: "counter"},
				Metric{Name: "network_receive_packets", Value: float64(io.PacketsRecv), Timestamp: timestamp, Labels: labels, Type: "counter"},
				Metric{Name: "network_transmit_packets", Value: float64(io.PacketsSent), Timestamp: timestamp, Labels: labels, Type: "counter"},
				Metric{Name: "network_receive_errors", Value: float64(io.Errin), Timestamp: timestamp, Labels: labels, Type: "counter"},
				Metric{Name: "network_transmit_errors", Value: float64(io.Errout), Timestamp: timestamp, Labels: labels, Type: "counter"},
			)
		}
	}

	// Load average
	if loadAvg, err := load.Avg(); err == nil {
		metrics = append(metrics,
			Metric{Name: "load_1min", Value: loadAvg.Load1, Timestamp: timestamp, Type: "gauge"},
			Metric{Name: "load_5min", Value: loadAvg.Load5, Timestamp: timestamp, Type: "gauge"},
			Metric{Name: "load_15min", Value: loadAvg.Load15, Timestamp: timestamp, Type: "gauge"},
		)
	}

	// Host info
	if hostInfo, err := host.Info(); err == nil {
		labels := map[string]string{
			"hostname": hostInfo.Hostname,
			"os":       hostInfo.OS,
			"platform": hostInfo.Platform,
			"kernel":   hostInfo.KernelVersion,
		}
		metrics = append(metrics,
			Metric{Name: "host_uptime_seconds", Value: float64(hostInfo.Uptime), Timestamp: timestamp, Labels: labels, Type: "counter"},
			Metric{Name: "host_boot_time", Value: float64(hostInfo.BootTime), Timestamp: timestamp, Labels: labels, Type: "gauge"},
		)
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
			metrics = append(metrics, Metric{
				Name:      "docker_running_containers",
				Value:     float64(len(containers)),
				Timestamp: timestamp,
				Type:      "gauge",
			})

			// Container-specific metrics
			for _, c := range containers {
				containerName := "unknown"
				if len(c.Names) > 0 {
					containerName = strings.TrimPrefix(c.Names[0], "/")
				}
				containerLabels := map[string]string{
					"container_id":   c.ID[:12],
					"container_name": containerName,
					"image":          c.Image,
					"status":         c.Status,
				}
				metrics = append(metrics,
					Metric{Name: "docker_container_state", Value: 1.0, Timestamp: timestamp, Labels: containerLabels, Type: "gauge"},
				)
			}
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
		Timestamp: timestamp,
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
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			log.Printf("close response body: %v", cerr)
		}
	}()

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
