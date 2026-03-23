package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func newAnomstackCmd() *cobra.Command {
	anomstackCmd := &cobra.Command{
		Use:   "anomstack",
		Short: "Launch Anomstack for anomaly detection",
		Long:  "Start the Anomstack anomaly detection system with dashboard and Dagster UI.",
	}

	anomstackCmd.AddCommand(newAnomstackStartCmd())
	anomstackCmd.AddCommand(newAnomstackStopCmd())
	anomstackCmd.AddCommand(newAnomstackStatusCmd())
	anomstackCmd.AddCommand(newAnomstackConfigCmd())

	return anomstackCmd
}

func newAnomstackStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start Anomstack services",
		RunE: func(cmd *cobra.Command, args []string) error {
			anomstackPath := rootViper.GetString("anomstack.path")
			if anomstackPath == "" {
				// Try to find anomstack in common locations
				candidates := []string{
					"../anomstack",
					"../../anomstack",
					filepath.Join(os.Getenv("HOME"), "programming", "anomstack"),
					"/Users/jocham/programming/anomstack",
				}
				for _, candidate := range candidates {
					if _, err := os.Stat(filepath.Join(candidate, "docker-compose.yaml")); err == nil {
						anomstackPath = candidate
						break
					}
				}
				if anomstackPath == "" {
					return fmt.Errorf("anomstack not found. Set anomstack.path in config or ensure it's in a standard location")
				}
			}

			fmt.Printf("Starting Anomstack from %s\n", anomstackPath)

			// Check if Docker Compose is available
			if err := exec.Command("docker-compose", "version").Run(); err != nil {
				// Try docker compose (v2)
				if err := exec.Command("docker", "compose", "version").Run(); err != nil {
					return fmt.Errorf("docker compose not available: %v", err)
				}
			}

			// Change to anomstack directory
			originalDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get current directory: %w", err)
			}
			defer os.Chdir(originalDir)

			if err := os.Chdir(anomstackPath); err != nil {
				return fmt.Errorf("change to anomstack directory: %w", err)
			}

			// Copy example metrics config if needed
			metricsDir := filepath.Join(anomstackPath, "metrics", "kube-watcher")
			if _, err := os.Stat(metricsDir); os.IsNotExist(err) {
				if err := createAnomstackMetricsConfig(anomstackPath); err != nil {
					fmt.Printf("Warning: Could not create metrics config: %v\n", err)
				}
			}

			// Start services
			fmt.Println("Starting Anomstack services...")
			dockerCmd := exec.Command("docker-compose", "up", "-d")
			if err := exec.Command("docker", "compose", "version").Run() == nil {
				dockerCmd = exec.Command("docker", "compose", "up", "-d")
			}

			dockerCmd.Stdout = os.Stdout
			dockerCmd.Stderr = os.Stderr
			if err := dockerCmd.Run(); err != nil {
				return fmt.Errorf("failed to start docker compose: %w", err)
			}

			fmt.Println("\n✅ Anomstack started!")
			fmt.Println("📊 Dashboard: http://localhost:5001")
			fmt.Println("⚙️  Dagster UI: http://localhost:3000")
			fmt.Println("\nTo stop: kw anomstack stop")
			fmt.Println("To view logs: docker-compose logs -f")

			return nil
		},
	}

	cmd.Flags().String("path", "", "Path to anomstack directory")
	rootViper.BindPFlag("anomstack.path", cmd.Flags().Lookup("path"))

	return cmd
}

func newAnomstackStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop Anomstack services",
		RunE: func(cmd *cobra.Command, args []string) error {
			anomstackPath := rootViper.GetString("anomstack.path")
			if anomstackPath == "" {
				return fmt.Errorf("anomstack.path not set in config")
			}

			originalDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get current directory: %w", err)
			}
			defer os.Chdir(originalDir)

			if err := os.Chdir(anomstackPath); err != nil {
				return fmt.Errorf("change to anomstack directory: %w", err)
			}

			fmt.Println("Stopping Anomstack services...")
			dockerCmd := exec.Command("docker-compose", "down")
			if err := exec.Command("docker", "compose", "version").Run() == nil {
				dockerCmd = exec.Command("docker", "compose", "down")
			}

			dockerCmd.Stdout = os.Stdout
			dockerCmd.Stderr = os.Stderr
			if err := dockerCmd.Run(); err != nil {
				return fmt.Errorf("failed to stop docker compose: %w", err)
			}

			fmt.Println("✅ Anomstack stopped")
			return nil
		},
	}
}

func newAnomstackStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check Anomstack service status",
		RunE: func(cmd *cobra.Command, args []string) error {
			anomstackPath := rootViper.GetString("anomstack.path")
			if anomstackPath == "" {
				return fmt.Errorf("anomstack.path not set in config")
			}

			originalDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get current directory: %w", err)
			}
			defer os.Chdir(originalDir)

			if err := os.Chdir(anomstackPath); err != nil {
				return fmt.Errorf("change to anomstack directory: %w", err)
			}

			dockerCmd := exec.Command("docker-compose", "ps")
			if err := exec.Command("docker", "compose", "version").Run() == nil {
				dockerCmd = exec.Command("docker", "compose", "ps")
			}

			dockerCmd.Stdout = os.Stdout
			dockerCmd.Stderr = os.Stderr
			return dockerCmd.Run()
		},
	}
}

func newAnomstackConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Generate Anomstack metric configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			anomstackPath := rootViper.GetString("anomstack.path")
			if anomstackPath == "" {
				return fmt.Errorf("anomstack.path not set in config")
			}

			if err := createAnomstackMetricsConfig(anomstackPath); err != nil {
				return fmt.Errorf("create metrics config: %w", err)
			}

			fmt.Println("✅ Anomstack metric configuration created")
			fmt.Printf("📁 Location: %s\n", filepath.Join(anomstackPath, "metrics", "kube-watcher"))
			return nil
		},
	}
}

func createAnomstackMetricsConfig(anomstackPath string) error {
	metricsDir := filepath.Join(anomstackPath, "metrics", "kube-watcher")
	if err := os.MkdirAll(metricsDir, 0755); err != nil {
		return fmt.Errorf("create metrics directory: %w", err)
	}

	// Create a simple metric config that reads from DuckDB
	configContent := `metric_batch: "kube_watcher"
table_key: "metrics_kube_watcher"
ingest_cron_schedule: "*/1 * * * *"  # Every minute
train_cron_schedule: "*/30 * * * *"  # Every 30 minutes
score_cron_schedule: "*/5 * * * *"   # Every 5 minutes
alert_cron_schedule: "*/10 * * * *"  # Every 10 minutes
change_cron_schedule: "*/15 * * * *" # Every 15 minutes
llmalert_cron_schedule: "*/15 * * * *"
plot_cron_schedule: "*/10 * * * *"
alert_always: False
alert_metric_timestamp_max_days_ago: 7
disable_llmalert: False
alert_methods: "webhook"
ingest_fn: |
  import duckdb
  import pandas as pd
  from datetime import datetime, timedelta
  
  # Connect to DuckDB
  conn = duckdb.connect("/data/anomstack.db")
  
  # For now, return empty dataframe - metrics will be populated by kube-watcher processor
  # In the future, this could query from a kube_watcher_metrics table
  df = pd.DataFrame({
      'metric_timestamp': [datetime.now()],
      'metric_batch': ['kube_watcher'],
      'metric_name': ['cpu_usage'],
      'metric_value': [0.0],
      'metric_type': ['gauge'],
      'labels': [{}]
  })
  
  return df`

	configFile := filepath.Join(metricsDir, "kube_watcher.yaml")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	// Also create a Python ingestion script
	pyContent := `#!/usr/bin/env python3
"""
Kube-Watcher metric ingestion for Anomstack.
This script reads metrics from kube-watcher and loads them into DuckDB.
"""

import duckdb
import pandas as pd
from datetime import datetime, timedelta
import json
import os

def ingest_metrics():
    """
    Ingest metrics from kube-watcher into DuckDB.
    
    This function is called by Anomstack on the ingest_cron_schedule.
    It should return a pandas DataFrame with columns:
    - metric_timestamp (datetime)
    - metric_batch (str)
    - metric_name (str)
    - metric_value (float)
    - metric_type (str)
    - labels (dict)
    """
    
    # Connect to DuckDB
    db_path = os.getenv("ANOMSTACK_DUCKDB_PATH", "/data/anomstack.db")
    conn = duckdb.connect(db_path)
    
    # Check if kube_watcher_metrics table exists
    # If not, return empty dataframe for now
    # In production, this would query actual metrics
    
    # Create table if it doesn't exist
    conn.execute("""
        CREATE TABLE IF NOT EXISTS kube_watcher_metrics (
            metric_timestamp TIMESTAMP,
            metric_batch VARCHAR,
            metric_name VARCHAR,
            metric_value DOUBLE,
            metric_type VARCHAR,
            labels VARCHAR,
            agent_id VARCHAR,
            user_id VARCHAR,
            tenant_id VARCHAR
        )
    """)
    
    # Query recent metrics
    query = """
        SELECT 
            metric_timestamp,
            metric_batch,
            metric_name,
            metric_value,
            metric_type,
            labels,
            agent_id,
            user_id,
            tenant_id
        FROM kube_watcher_metrics
        WHERE metric_timestamp > CURRENT_TIMESTAMP - INTERVAL '1 hour'
        ORDER BY metric_timestamp DESC
        LIMIT 1000
    """
    
    try:
        result = conn.execute(query).fetchdf()
        if len(result) > 0:
            # Convert labels from JSON string to dict
            result['labels'] = result['labels'].apply(lambda x: json.loads(x) if x else {})
            return result
    except Exception as e:
        print(f"Error querying metrics: {e}")
    
    # Return empty dataframe if no data
    return pd.DataFrame(columns=[
        'metric_timestamp', 'metric_batch', 'metric_name', 
        'metric_value', 'metric_type', 'labels',
        'agent_id', 'user_id', 'tenant_id'
    ])

if __name__ == "__main__":
    df = ingest_metrics()
    print(f"Ingested {len(df)} metrics")
    if len(df) > 0:
        print(df.head())`

	pyFile := filepath.Join(metricsDir, "kube_watcher.py")
	if err := os.WriteFile(pyFile, []byte(pyContent), 0644); err != nil {
		return fmt.Errorf("write Python file: %w", err)
	}

	// Create README
	readmeContent := `# Kube-Watcher Metrics for Anomstack

This directory contains metric configurations for integrating kube-watcher with Anomstack.

## Current Integration

1. **kube_watcher.yaml** - Metric configuration for Anomstack
2. **kube_watcher.py** - Python ingestion script

## How It Works

1. Kube-watcher agents collect system and Docker metrics
2. Metrics are sent to ingestion API
3. Processor writes metrics to DuckDB (anomstack.db)
4. Anomstack reads metrics from DuckDB and performs anomaly detection
5. Alerts are sent to configured webhooks

## Next Steps

1. Update the processor to write metrics to DuckDB table \`kube_watcher_metrics\`
2. Configure alert webhooks in Anomstack
3. Customize anomaly detection thresholds

## File Structure

- \`kube_watcher.yaml\` - Main configuration
- \`kube_watcher.py\` - Ingestion script
- \`README.md\` - This file`

	readmeFile := filepath.Join(metricsDir, "README.md")
	if err := os.WriteFile(readmeFile, []byte(readmeContent), 0644); err != nil {
		return fmt.Errorf("write README: %w", err)
	}

	return nil
}