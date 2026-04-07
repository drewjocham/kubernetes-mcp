# kube-watcher Helm Chart

Helm chart for deploying the kube-watcher monitoring platform, including the MCP server, watcher event engine, anomaly detection stack, and observability components.

## Overview

This chart deploys the following components:

- **MCP Server**: Model Context Protocol server exposing Kubernetes cluster state via HTTP/stdio
- **Watcher Event Engine**: Real-time rule engine with CEL/YAML condition evaluation
- **Badger Explorer**: Sidecar for exploring BadgerDB history
- **Prometheus**: Metrics collection and alerting
- **SigNoz**: Distributed tracing and observability platform
- **Anomstack**: Dagster-based anomaly detection pipeline
- **K8sGPT**: Kubernetes AI-powered diagnostics

## Prerequisites

- Kubernetes 1.24+
- Helm 3.12+
- PersistentVolume provisioner (optional, for data persistence)

## Installation

### Add the Helm repository (if published)

```bash
helm repo add kube-watcher https://your-org.github.io/kube-watcher
helm repo update
```

### Install the chart

```bash
# Create namespace
kubectl create namespace kube-watcher

# Install with default values
helm install kube-watcher kube-watcher/kube-watcher \
  --namespace kube-watcher \
  --version 0.1.0
```

### Upgrade an existing release

```bash
helm upgrade kube-watcher kube-watcher/kube-watcher \
  --namespace kube-watcher \
  --version 0.1.0
```

### Uninstall

```bash
helm uninstall kube-watcher --namespace kube-watcher
kubectl delete namespace kube-watcher
```

## Configuration

The chart can be customized via a values file. See [Values](#values) section for all available options.

### Example custom values

```yaml
# values-custom.yaml
namespaces:
  kubeWatcher: kube-watcher
  anomstack: kube-watcher

mcp:
  enabled: true
  image:
    repository: watcher-mcp
    tag: latest
  resources:
    requests:
      cpu: 100m
      memory: 128Mi
    limits:
      cpu: 500m
      memory: 256Mi

watcher:
  enabled: true
  persistence:
    enabled: true
    size: 10Gi
    storageClass: standard

prometheus:
  enabled: true

signoz:
  enabled: true
  replicaCount: 1
  ingress:
    enabled: true
    host: signoz.example.com

anomstack:
  enabled: false

k8sgpt:
  enabled: false
```

Install with custom values:

```bash
helm install kube-watcher kube-watcher/kube-watcher \
  --namespace kube-watcher \
  -f values-custom.yaml
```

## Values

### Global

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `namespaces.kubeWatcher` | string | `"kube-watcher"` | Namespace for core components |
| `namespaces.anomstack` | string | `"kube-watcher"` | Namespace for anomaly detection stack |
| `serviceAccount.create` | bool | `true` | Create a service account |
| `serviceAccount.name` | string | `"kube-watcher"` | Service account name |

### MCP Server

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `mcp.enabled` | bool | `true` | Enable MCP server deployment |
| `mcp.image.repository` | string | `"watcher-mcp"` | MCP server image repository |
| `mcp.image.tag` | string | `"latest"` | MCP server image tag |
| `mcp.image.pullPolicy` | string | `"IfNotPresent"` | Image pull policy |
| `mcp.port` | int | `8080` | HTTP port |
| `mcp.logLevel` | string | `"info"` | Log level |
| `mcp.dbPath` | string | `"/home/nonroot/.kube-watcher/history.db"` | BadgerDB path |
| `mcp.resources` | object | `{requests: {cpu: 100m, memory: 128Mi}, limits: {cpu: 500m, memory: 256Mi}}` | Resource requests/limits |
| `mcp.persistence.enabled` | bool | `false` | Enable persistent volume for BadgerDB |
| `mcp.persistence.size` | string | `"1Gi"` | PVC size |
| `mcp.persistence.storageClass` | string | `""` | Storage class (empty uses default) |

### Watcher Event Engine

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `watcher.enabled` | bool | `false` | Enable watcher engine deployment |
| `watcher.image.repository` | string | `"watcher"` | Watcher engine image repository |
| `watcher.image.tag` | string | `"latest"` | Watcher engine image tag |
| `watcher.image.pullPolicy` | string | `"IfNotPresent"` | Image pull policy |
| `watcher.ports.http` | int | `8085` | HTTP API port |
| `watcher.ports.metrics` | int | `9095` | Metrics port |
| `watcher.logLevel` | string | `"info"` | Log level |
| `watcher.resources` | object | `{requests: {cpu: 100m, memory: 128Mi}, limits: {cpu: 500m, memory: 256Mi}}` | Resource requests/limits |
| `watcher.persistence.enabled` | bool | `false` | Enable persistent volume for BadgerDB |
| `watcher.persistence.size` | string | `"1Gi"` | PVC size |
| `watcher.persistence.storageClass` | string | `""` | Storage class |

### Badger Explorer

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `badgerExplorer.enabled` | bool | `true` | Enable Badger Explorer sidecar |
| `badgerExplorer.image.repository` | string | `"watcher-badger-explorer"` | Badger Explorer image repository |
| `badgerExplorer.image.tag` | string | `"latest"` | Badger Explorer image tag |
| `badgerExplorer.image.pullPolicy` | string | `"IfNotPresent"` | Image pull policy |
| `badgerExplorer.port` | int | `4101` | HTTP port |
| `badgerExplorer.historyLimit` | int | `120` | History limit |

### Prometheus

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `prometheus.enabled` | bool | `true` | Enable Prometheus deployment |
| `prometheus.image.repository` | string | `"prom/prometheus"` | Prometheus image repository |
| `prometheus.image.tag` | string | `"latest"` | Prometheus image tag |
| `prometheus.image.pullPolicy` | string | `"IfNotPresent"` | Image pull policy |
| `prometheus.port` | int | `9090` | HTTP port |
| `prometheus.resources` | object | `{requests: {cpu: 100m, memory: 256Mi}, limits: {cpu: 500m, memory: 512Mi}}` | Resource requests/limits |
| `prometheus.persistence.enabled` | bool | `false` | Enable persistent volume for metrics |
| `prometheus.persistence.size` | string | `"10Gi"` | PVC size |
| `prometheus.persistence.storageClass` | string | `""` | Storage class |

### SigNoz

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `signoz.enabled` | bool | `true` | Enable SigNoz deployment |
| `signoz.replicaCount` | int | `1` | Number of replicas |
| `signoz.image.repository` | string | `"signoz/signoz"` | SigNoz image repository |
| `signoz.image.tag` | string | `"v0.117.1"` | SigNoz image tag |
| `signoz.image.pullPolicy` | string | `"IfNotPresent"` | Image pull policy |
| `signoz.clickhouse.image.repository` | string | `"clickhouse/clickhouse-server"` | ClickHouse image repository |
| `signoz.clickhouse.image.tag` | string | `"24.8-alpine"` | ClickHouse image tag |
| `signoz.clickhouse.image.pullPolicy` | string | `"IfNotPresent"` | Image pull policy |
| `signoz.resources` | object | `{requests: {cpu: 100m, memory: 256Mi}, limits: {cpu: 500m, memory: 512Mi}}` | SigNoz resource requests/limits |
| `signoz.clickhouse.resources` | object | `{requests: {cpu: 100m, memory: 512Mi}, limits: {cpu: 500m, memory: 1024Mi}}` | ClickHouse resource requests/limits |
| `signoz.otelCollector.enabled` | bool | `true` | Enable OpenTelemetry Collector |
| `signoz.otelCollector.replicaCount` | int | `1` | Number of collector replicas |
| `signoz.otelCollector.image.repository` | string | `"otel/opentelemetry-collector"` | Collector image repository |
| `signoz.otelCollector.image.tag` | string | `"0.102.1"` | Collector image tag |
| `signoz.otelCollector.image.pullPolicy` | string | `"IfNotPresent"` | Image pull policy |
| `signoz.otelCollector.resources` | object | `{requests: {cpu: 100m, memory: 128Mi}, limits: {cpu: 500m, memory: 256Mi}}` | Collector resource requests/limits |
| `signoz.ingress.enabled` | bool | `true` | Enable ingress for SigNoz UI |
| `signoz.ingress.host` | string | `"signoz.local"` | Ingress host |
| `signoz.ingress.path` | string | `"/"` | Ingress path |
| `signoz.ingress.annotations` | object | `{}` | Ingress annotations |
| `signoz.ingress.tls` | list | `[]` | TLS configuration |
| `signoz.persistence.enabled` | bool | `false` | Enable persistent volumes for SigNoz and ClickHouse |
| `signoz.persistence.size` | string | `"10Gi"` | PVC size |
| `signoz.persistence.storageClass` | string | `""` | Storage class |

### Anomstack

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `anomstack.enabled` | bool | `false` | Enable Anomstack deployment |
| `anomstack.dagster.image.repository` | string | `"anomstack_dagster_image"` | Dagster image repository |
| `anomstack.dagster.image.tag` | string | `"kw-local"` | Dagster image tag |
| `anomstack.dagster.image.pullPolicy` | string | `"IfNotPresent"` | Image pull policy |
| `anomstack.dashboard.image.repository` | string | `"anomstack_dashboard_image"` | Dashboard image repository |
| `anomstack.dashboard.image.tag` | string | `"kw-local"` | Dashboard image tag |
| `anomstack.dashboard.image.pullPolicy` | string | `"IfNotPresent"` | Image pull policy |
| `anomstack.env.duckdbPath` | string | `"/data/anomstack.db"` | DuckDB path |
| `anomstack.env.home` | string | `"/opt/dagster/app"` | Home directory |
| `anomstack.env.tableKey` | string | `"tmp.metrics"` | Table key |
| `anomstack.env.dashboardPort` | string | `"8080"` | Dashboard port |
| `anomstack.env.llmPlatform` | string | `"openai"` | LLM platform |
| `anomstack.env.llmalertModels` | string | `"gpt-4o-mini"` | LLM alert models |
| `anomstack.persistence.size` | string | `"5Gi"` | PVC size |
| `anomstack.persistence.storageClass` | string | `""` | Storage class |
| `anomstack.resources.daemon` | object | `{requests: {cpu: 100m, memory: 256Mi}, limits: {cpu: 500m, memory: 512Mi}}` | Daemon resource requests/limits |
| `anomstack.resources.webserver` | object | `{requests: {cpu: 100m, memory: 256Mi}, limits: {cpu: 500m, memory: 512Mi}}` | Webserver resource requests/limits |
| `anomstack.resources.dashboard` | object | `{requests: {cpu: 100m, memory: 256Mi}, limits: {cpu: 500m, memory: 512Mi}}` | Dashboard resource requests/limits |

### K8sGPT

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `k8sgpt.enabled` | bool | `false` | Enable K8sGPT deployment |
| `k8sgpt.image.repository` | string | `"k8sgpt/k8sgpt"` | K8sGPT image repository |
| `k8sgpt.image.tag` | string | `"latest"` | K8sGPT image tag |
| `k8sgpt.image.pullPolicy` | string | `"IfNotPresent"` | Image pull policy |
| `k8sgpt.port` | int | `8089` | HTTP port |
| `k8sgpt.mcpPort` | int | `8089` | MCP port |
| `k8sgpt.backend` | string | `"openai"` | AI backend |
| `k8sgpt.model` | string | `"gpt-4o"` | AI model |
| `k8sgpt.apiKey` | string | `""` | API key (optional) |
| `k8sgpt.enableMCP` | bool | `true` | Enable MCP server |
| `k8sgpt.resources` | object | `{requests: {cpu: 100m, memory: 128Mi}, limits: {cpu: 500m, memory: 256Mi}}` | Resource requests/limits |

### Advanced Configuration

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `global.imagePullSecrets` | list | `[]` | Image pull secrets for all deployments |
| `global.nodeSelector` | object | `{}` | Node selector for all deployments |
| `global.tolerations` | list | `[]` | Tolerations for all deployments |
| `global.affinity` | object | `{}` | Affinity for all deployments |
| `global.podAnnotations` | object | `{}` | Annotations added to all pods |
| `global.podLabels` | object | `{}` | Labels added to all pods |
| `global.priorityClassName` | string | `""` | Priority class name for all pods |

## Persistence

By default, all components use `emptyDir` volumes for data storage. You can enable persistent volumes for:

- **MCP Server**: BadgerDB history
- **Watcher Engine**: BadgerDB event store
- **Prometheus**: Metrics storage
- **SigNoz**: SigNoz and ClickHouse data
- **Anomstack**: DuckDB and pipeline data

Enable persistence in the values file:

```yaml
mcp:
  persistence:
    enabled: true
    size: 5Gi
    storageClass: standard

watcher:
  persistence:
    enabled: true
    size: 10Gi
    storageClass: standard

prometheus:
  persistence:
    enabled: true
    size: 20Gi
    storageClass: fast

signoz:
  persistence:
    enabled: true
    size: 50Gi
    storageClass: standard
```

## Security

All pods run with non-root users where possible. Security contexts are configured to:

- Run as non-root user (65532 for MCP, 65534 for Prometheus, 10001 for Anomstack)
- Disable privilege escalation
- Use read-only root filesystem
- Drop all Linux capabilities

The watcher engine runs as root (`runAsUser: 0`) by default for local development simplicity. For production, consider adjusting the security context.

## Network Policies

Network policies are deployed to restrict traffic between components:

- **signoz-allow-monitoring**: Allows ingress from kube-watcher and kube-system namespaces to SigNoz OTEL collector ports (4317, 4318)
- **prometheus-allow-egress**: Allows egress from Prometheus to the anomaly stack namespace on ports 8080 and 3000

## Contributing

Contributions to improve this Helm chart are welcome. Please submit pull requests or open issues in the repository.

## License

[Apache 2.0](LICENSE)
