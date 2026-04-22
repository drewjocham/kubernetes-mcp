# Pub/Sub Integration for Real-time Anomaly Detection

This document describes the Pub/Sub integration between Anomstack (anomaly detection) and Watcher (event engine) for real-time anomaly alerting.

## Overview

The integration replaces the previous HTTP polling mechanism with a Google Cloud Pub/Sub based publish-subscribe pattern:

1. **Anomstack** publishes detected anomalies to a Pub/Sub topic
2. **Watcher** subscribes to the topic and processes anomalies in real-time
3. **Rules engine** evaluates anomalies and triggers actions (webhooks, alerts, etc.)

## Architecture

```
┌─────────────┐     publish     ┌─────────────┐     consume     ┌─────────────┐
│  Anomstack  │ ──────────────▶ │  Pub/Sub    │ ──────────────▶ │  Watcher    │
│  (Publisher)│   anomalies     │   Topic     │   anomalies     │ (Subscriber)│
└─────────────┘                 └─────────────┘                 └─────────────┘
        │                                                              │
        │ detect anomalies                                             │ evaluate rules
        ▼                                                              ▼
┌─────────────┐                                                 ┌─────────────┐
│  Metrics    │                                                 │  Actions    │
│  Database   │                                                 │ (webhooks,  │
│             │                                                 │   alerts)   │
└─────────────┘                                                 └─────────────┘
```

## Configuration

### 1. Google Cloud Project Setup

Ensure you have a Google Cloud Project with Pub/Sub API enabled:

```bash
# Set your project ID
export GOOGLE_CLOUD_PROJECT="your-project-id"

# Enable Pub/Sub API
gcloud services enable pubsub.googleapis.com
```

### 2. Service Account Credentials

Create a service account with Pub/Sub permissions:

```bash
# Create service account
gcloud iam service-accounts create kube-watcher-pubsub \
    --display-name="Kube Watcher Pub/Sub"

# Grant Pub/Sub permissions
gcloud projects add-iam-policy-binding $GOOGLE_CLOUD_PROJECT \
    --member="serviceAccount:kube-watcher-pubsub@${GOOGLE_CLOUD_PROJECT}.iam.gserviceaccount.com" \
    --role="roles/pubsub.publisher"

gcloud projects add-iam-policy-binding $GOOGLE_CLOUD_PROJECT \
    --member="serviceAccount:kube-watcher-pubsub@${GOOGLE_CLOUD_PROJECT}.iam.gserviceaccount.com" \
    --role="roles/pubsub.subscriber"

# Create and download key
gcloud iam service-accounts keys create kube-watcher-pubsub-key.json \
    --iam-account=kube-watcher-pubsub@${GOOGLE_CLOUD_PROJECT}.iam.gserviceaccount.com
```

### 3. Kubernetes Deployment

Add the service account key as a Kubernetes secret:

```bash
kubectl create secret generic gcp-pubsub-credentials \
    --namespace=kube-watcher \
    --from-file=credentials.json=kube-watcher-pubsub-key.json
```

Update Helm values (`helm/values.yaml`):

```yaml
# Anomstack configuration
anomstack:
  env:
    googleCloudProject: "your-project-id"
    anomstackPubsubTopic: "anomaly-events-topic"
  
# Watcher configuration  
watcher:
  configYaml: |
    settings:
      pubsub:
        enabled: true
        project_id: "your-project-id"
        subscription_id: "anomaly-events-sub"
        topic_id: "anomaly-events-topic"
  
# Add environment variables to pods
extraEnv:
  - name: GOOGLE_CLOUD_PROJECT
    value: "your-project-id"
  - name: GOOGLE_APPLICATION_CREDENTIALS
    value: "/var/run/secrets/gcp/credentials.json"
  
extraVolumes:
  - name: gcp-credentials
    secret:
      secretName: gcp-pubsub-credentials

extraVolumeMounts:
  - name: gcp-credentials
    mountPath: /var/run/secrets/gcp
    readOnly: true
```

### 4. Topic and Subscription Creation

The system automatically creates the topic and subscription if they don't exist. Default names:

- **Topic**: `anomaly-events-topic`
- **Subscription**: `anomaly-events-sub`

You can override these in the configuration.

## Message Format

Anomstack publishes messages in the following JSON format:

```json
{
  "anomalies": [
    {
      "id": "python_ingest_simple_test_metric_2024-04-19T18:30:00Z",
      "title": "Test Metric",
      "severity": "critical",
      "message": "Anomaly detected for test_metric at 2024-04-19T18:30:00Z. Score: 0.95, value: 42.5",
      "timestamp": 1713544200
    }
  ]
}
```

## Rules Configuration

Enable anomaly detection rules in Watcher:

```yaml
rules:
  - name: Anomstack-Critical-Anomaly
    kind: Anomaly
    condition: "evt.severity == 'critical'"
    actions:
      - dashboard-webhook
      - log_change
      
  - name: Anomstack-Warning-Anomaly
    kind: Anomaly
    condition: "evt.severity == 'warning'"
    actions:
      - log_change
```

## Testing the Integration

### 1. Local Testing with Emulator

Use the Google Cloud Pub/Sub emulator for local testing:

```bash
# Start emulator
docker run -d -p 8085:8085 google/cloud-sdk gcloud beta emulators pubsub start --project=test-project --host-port=0.0.0.0:8085

# Set emulator host
export PUBSUB_EMULATOR_HOST=localhost:8085
export GOOGLE_CLOUD_PROJECT=test-project

# Run services
make run-server  # MCP server
make run-watcher # Watcher with pubsub source
```

### 2. Trigger Test Anomaly

Manually trigger an anomaly in Anomstack:

```bash
# Port-forward to anomstack dashboard
kubectl port-forward -n kube-watcher svc/anomstack-dashboard 8080:8080

# Trigger anomaly (example)
curl -X POST http://localhost:8080/api/trigger-anomaly \
  -H "Content-Type: application/json" \
  -d '{"metric_batch": "python_ingest_simple", "metric_name": "test_metric"}'
```

### 3. Verify Message Flow

Check Watcher logs:

```bash
kubectl logs -n kube-watcher -l app=watcher --tail=50 -f
```

Expected output:
```
{"level":"INFO","msg":"pubsub source starting","subscription":"anomaly-events-sub","topic":"anomaly-events-topic"}
{"level":"INFO","msg":"processed pubsub message","anomalies":1}
{"level":"INFO","msg":"rule action","rule":"Anomstack-Critical-Anomaly","action":"log_change","message":"Anomstack-Critical-Anomaly triggered for anomstack/python_ingest_simple_test_metric_2024-04-19T18:30:00Z"}
```

## Fallback Behavior

If Pub/Sub credentials are not available, the system gracefully degrades:

1. **Anomstack**: Skips Pub/Sub publishing, falls back to other alert methods (email, Slack)
2. **Watcher**: Logs warning and skips Pub/Sub source initialization

This ensures the system continues to function without Pub/Sub.

## Monitoring

### Metrics

Watcher exposes Prometheus metrics for Pub/Sub:

- `watcher_pubsub_messages_received_total`: Total messages received
- `watcher_pubsub_messages_processed_total`: Total messages processed
- `watcher_pubsub_errors_total`: Total Pub/Sub errors

### Logs

Key log events to monitor:

- `"pubsub source starting"`: Successful initialization
- `"pubsub credentials not found"`: Missing credentials (warning)
- `"failed to create pubsub client"`: Connection error (error)
- `"processed pubsub message"`: Message processing success

## Troubleshooting

### Common Issues

1. **Missing credentials**
   ```
   "pubsub credentials not found, skipping pubsub source"
   ```
   **Solution**: Ensure `GOOGLE_APPLICATION_CREDENTIALS` is set and points to a valid service account key.

2. **Permission denied**
   ```
   "failed to create pubsub client: rpc error: code = PermissionDenied"
   ```
   **Solution**: Verify the service account has required Pub/Sub roles.

3. **Topic/Subscription not found**
   ```
   "failed to check topic existence"
   ```
   **Solution**: The system auto-creates topics/subscriptions. Ensure the service account has create permissions.

4. **Emulator connection issues**
   ```
   "failed to create pubsub client: dialing: google: could not find default credentials"
   ```
   **Solution**: When using emulator, set `PUBSUB_EMULATOR_HOST` and unset `GOOGLE_APPLICATION_CREDENTIALS`.

### Debug Mode

Enable debug logging for detailed Pub/Sub operations:

```yaml
watcher:
  logLevel: debug
```

## Migration from HTTP Polling

The previous HTTP polling mechanism is still available but disabled by default. To re-enable:

```yaml
watcher:
  configYaml: |
    settings:
      anomstack:
        enabled: true
        endpoint: http://anomstack-dashboard:8080/anomalies
        interval: 30s
      pubsub:
        enabled: false  # Disable Pub/Sub
```

## Performance Considerations

- **Message size**: Keep anomaly messages under 10MB (Pub/Sub limit)
- **Batch size**: Anomstack batches anomalies to reduce publish calls
- **Ack deadline**: Default 30 seconds, adjustable in configuration
- **Retention**: Default 7 days, adjustable in configuration

## Security

- **Encryption**: Pub/Sub encrypts messages at rest and in transit
- **IAM**: Use least-privilege service accounts
- **Secret management**: Store credentials in Kubernetes secrets, not in configuration files
- **Audit logging**: Enable Cloud Audit Logs for Pub/Sub operations