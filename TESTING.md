# Local End-to-End Testing

This guide walks through testing the No-K8s monitoring architecture locally using Docker Compose.

## Prerequisites

- Docker and Docker Compose
- Go 1.25+ (optional, for building binaries)
- `gcloud` CLI (optional, for Pub/Sub emulator interaction)

## Components

1. **Pub/Sub Emulator**: Google Cloud Pub/Sub emulator.
2. **Ingest API**: Receives telemetry from agents, publishes to Pub/Sub.
3. **Processor**: Subscribes to Pub/Sub, stores metrics in DuckDB, checks for anomalies, sends webhooks.
4. **Agent**: Collects system metrics (CPU, memory, disk, network, Docker containers) and sends to ingest API.
5. **Anomstack**: Anomaly detection system (runs separately via CLI).

## Step 1: Start Anomstack

Run the following from the project root:

```bash
go run main.go anomstack start
```

This will start Anomstack services (dashboard, Dagster) using its own Docker Compose. Wait for all services to be healthy (may take a minute). The dashboard will be available at http://localhost:3000.

## Step 2: Start the monitoring stack

Create a directory for DuckDB data:

```bash
mkdir -p data
```

Copy metric configuration (already present in `anomstack-metrics/`).

Start the stack:

```bash
docker-compose -f docker-compose.test.yaml up --build
```

This will:
- Start Pub/Sub emulator on port 8085
- Build and start ingest API on port 8080
- Build and start processor (subscribes to Pub/Sub, initializes DuckDB)
- Build and start agent (begins sending metrics every 30 seconds)

## Step 3: Verify metrics flow

Check logs to ensure no errors. You should see agent collecting metrics, ingest receiving, processor storing.

To verify metrics are stored in DuckDB:

```bash
docker exec -it kube-watcher_processor_1 duckdb /data/anomstack.db "SELECT COUNT(*) FROM metrics;"
```

## Step 4: Register a test webhook

Currently webhook registration is done by inserting into the `user_webhooks` table. Example:

```bash
docker exec -it kube-watcher_processor_1 duckdb /data/anomstack.db "INSERT INTO user_webhooks (user_id, webhook_url) VALUES ('test-user', 'http://localhost:8080/webhook');"
```

You can use a tool like `ngrok` or `webhook.site` to capture webhooks, or run a local HTTP server.

## Step 5: Trigger anomalies

Anomstack will periodically score metrics and write scores to the same `metrics` table with `metric_type = 'score'`. The processor will pick up scores above threshold (0.8) and send webhooks.

To simulate an anomaly, you can manually insert a high score:

```bash
docker exec -it kube-watcher_processor_1 duckdb /data/anomstack.db "INSERT INTO metrics (metric_timestamp, metric_batch, metric_name, metric_value, metric_type, metadata) VALUES (CURRENT_TIMESTAMP, 'system_metrics', 'user_test-user_cpu_usage_percent', 0.95, 'score', '{\"user_id\":\"test-user\"}');"
```

Check processor logs for webhook sending.

## Step 6: View anomalies in Anomstack dashboard

Navigate to http://localhost:3000 to see metrics and anomaly scores.

## Troubleshooting

- **Dashboard “Tools” page / MCP HTTP API**: The server proxies to `toolsEndpoint` from workflow config. Allowed hosts default to `localhost`, `127.0.0.1`, `::1`, and `host.docker.internal`. To use another host, set `TOOLS_ENDPOINT_ALLOWED_HOSTS` (comma-separated hostnames) in the Nitro/server environment.
- **Google Chat / webhook URLs**: Do not commit real URLs or keys in `watcher/internal/config/config.yaml`. Merge a local override or inject URLs in your deploy pipeline.
- **Pub/Sub emulator connectivity**: Ensure `PUBSUB_EMULATOR_HOST` is set correctly (should be `pubsub-emulator:8085` inside containers).
- **DuckDB CGO errors**: Processor Dockerfile includes CGO support. If building locally, set `CGO_ENABLED=1`.
- **Missing metric configuration**: Ensure `anomstack-metrics/system_metrics/system_metrics.yaml` exists and matches the metric batch used by processor (`system_metrics`).
- **Webhooks not sent**: Check `user_webhooks` table has correct user_id matching the prefix in metric names (e.g., `user_test-user_`).

## Cleaning up

Stop the stack with Ctrl+C, then:

```bash
docker-compose -f docker-compose.test.yaml down
go run main.go anomstack stop
```

## Next Steps

- Implement OAuth2/OIDC for user authentication
- Add REST API for webhook registration
- Improve anomaly detection integration (ensure anomstack writes scores, processor reads them)
- Add multi‑tenant webhook routing with per‑user alert preferences