# kube-watcher

A Kubernetes monitoring and analysis MCP (Model Context Protocol) server designed to provide cluster insights and monitoring capabilities.

## Features

- **Node Monitoring**: Detailed analysis of cluster nodes, health status, and resource allocation
- **Pod Analysis**: Comprehensive pod resource monitoring, restart tracking, and issue identification  
- **Cluster Analysis**: Full cluster health assessment with recommendations and alerting

## Architecture

```
kube-watcher/
├── cli/               # Unified CLI for MCP, watcher, and operations
│   ├── root.go        # CLI root command
│   ├── agent.go       # Agent management commands
│   ├── ops.go         # Deployment operations
│   └── ...            # Other command implementations
├── mcp/               # MCP server app
│   ├── cmd/server     # MCP server entrypoint
│   ├── server/        # MCP server implementation
│   └── tools/         # Tool definitions
├── watcher/           # Event engine app
│   ├── cmd/engine     # Event-engine entrypoint
│   └── internal/      # Rules, pipeline, actions, trackers
├── services/          # Monitoring services (ingest, probe, processor)
│   ├── ingest/        # Telemetry ingestion service
│   ├── probe/         # Metrics collection service
│   └── processor/     # Data processing service
├── pkg/               # Reusable libraries (logging, kube client, etc.)
│   ├── kube/          # Kubernetes client wrapper
│   ├── logging/       # Structured logging utilities
│   ├── profile/       # Configuration profile management
│   └── ...            # Other shared packages
├── terminal/          # Terminal emulator (PTY + Bubble Tea TUI)
├── anomstack/         # Anomaly detection stack (submodule)
├── popeye/            # Kubernetes cluster sanitizer
├── helm/              # Helm charts for deployment
├── integrations/      # Integration bridges (Google Chat, etc.)
└── main.go            # CLI entry point
```
## Installation

1. Clone the repository:
```bash
git clone <repository-url>
cd kube-watcher
```

2. Install dependencies:
```bash
go mod tidy
```

3. Ensure you have kubectl configured and access to a Kubernetes cluster.

### Homebrew commands
If installed via Homebrew, the formula currently installs the `kube-watcher` binary.

### Local development
When building from source, the Makefile creates a `kw` symlink to `kw-cli` (the unified CLI) for convenience. 
You can install `kw` to your PATH using:
```bash
make install-local    # installs to ~/.local/bin
# or
make install-system   # installs to /usr/local/bin (may require sudo)
```

```bash
kube-watcher version
```
Prints the installed version, git commit, and build date.

```bash
kube-watcher mcp tools --output table
```
Lists available MCP tools exposed by the server.

```bash
kube-watcher mcp health --output yaml
```
Runs a health check (server wiring + Kubernetes connectivity).

```bash
kube-watcher mcp run --tool analyze_cluster --args '{"include_pods":true,"include_events":true}' --output yaml
```
Executes an MCP tool and prints formatted output.

```bash
kube-watcher deploy --action deploy --target kube --cluster-name <cluster-name> --prometheus-endpoint <prometheus-metrics-url>
```
Deploys anomaly detection to Kubernetes (default namespace `kubewatcher`, override with `--namespace`).

```bash
kube-watcher deploy --action deploy --target docker --cluster-name <cluster-name> --prometheus-endpoint <prometheus-metrics-url>
```
Runs anomaly detection locally in Docker.

```bash
kube-watcher deploy --action status --target kube --cluster-name <cluster-name>
kube-watcher deploy --action logs --target docker --cluster-name <cluster-name> --follow
```
Checks runtime status and streams logs using the unified CLI.

## Usage
### CLI HowTo (Quick Reference)

#### Show help
```bash
kw --help
```

#### Launch terminal module
```bash
kw terminal
# or via Makefile target
make run-terminal
# run terminal module tests
make test-terminal
```
Inside the terminal module, built-in system commands are available:
`/help`, `/agent`, `/widgets`, `/widgets refresh`.

#### Print example config
```bash
kw config example
```

#### Manage agents
```bash
kw agent create "sre-bot" "Specialist in CrashLoopBackOff analysis"
kw agent list
kw agent show sre-bot
```

#### Manage runtime profiles (config/env/secrets)
```bash
kw profile create dev --description "Local development"
kw profile set-config dev --key ops.target --value docker
kw profile set-env dev --key KUBECONFIG --value ~/.kube/config
kw profile set-secret-ref dev --key MCP_API_TOKEN --provider env --ref MCP_API_TOKEN
kw profile use dev
kw profile env dev --output yaml
```

#### MCP tools (formatted output)
```bash
kw view tools --output table
kw view run --tool analyze_cluster --args '{"include_pods":true,"include_events":true}' --output yaml
kw view status
kw view insights --hours 6
```

#### Deploy components (docker/binary/kube)
```bash
# Docker
kw ops deploy mcp --target docker
kw ops deploy watcher --target docker

# Local binaries
kw ops deploy mcp --target binary --binary-mcp /path/to/mcp-server
kw ops deploy watcher --target binary --binary-watcher /path/to/watcher-engine

# Kubernetes
kw ops deploy mcp --target kube --kube-namespace kubewatcher
kw ops deploy watcher --target kube --kube-namespace kubewatcher
```

#### Ops status/logs/cleanup
```bash
kw ops status mcp
kw ops logs watcher --tail 200 --follow
kw ops cleanup mcp
```

### Command Line Interface

#### List Available Tools
```bash
kube-watcher mcp tools --output table
```

```shell
    export KUBECONFIG_PATH="${HOME}/.kube/config"
    docker compose -f docker/compose.yaml up 
```

#### Execute Specific Tools
```bash
# Node status analysis
kube-watcher mcp run --tool get_node_status --args '{"include_metrics":true}' --output yaml

# Pod resource monitoring
kube-watcher mcp run --tool get_pod_resources --args '{"problematic_only":true}' --output yaml

# Full cluster analysis  
kube-watcher mcp run --tool analyze_cluster --args '{"include_pods":true,"include_events":true}' --output yaml
```

#### Health Check
```bash
kube-watcher mcp health --output yaml
```

#### Interactive Server Mode
```bash
kube-watcher mcp serve
```
### Watcher Event Engine

The watcher analyzes cluster events based on your YAML rules/configuration:

```bash
go run ./watcher/cmd/engine --config /path/to/event-engine.yaml
```

Pass `--debug` or `--listen :8085` to enable verbose logging and health endpoints.
If you omit `--config`, the engine automatically loads `watcher/internal/config/config.yaml`.

### Kubernetes anomaly detection deploy CLI
Use the deploy helper to run `kube-anomaly-detection` from a Docker image (or build it from a local source path) with minimal required inputs:
- `--cluster-name`
- `--prometheus-endpoint`

Defaults:
- namespace: `kubewatcher` (override with `--namespace`)
- deployment name: `kube-anomaly-detection-<cluster-name>`
- for `--target kube`, namespace creation is enabled by default
- Kubernetes data storage uses a PVC by default:
  - claim name: `<deployment-name>-data` (override with `--pvc-name`)
  - claim size: `5Gi` (override with `--pvc-size`)
- Google Chat webhook URL can be provided via `GOOGLE_CHAT_WEBHOOK_URL` environment variable (recommended for security) or `--google-chat-webhook-url` flag

```bash
# Deploy to Kubernetes from image
kube-watcher deploy \
  --action deploy \
  --target kube \
  --cluster-name dev-cluster \
  --prometheus-endpoint http://prom-prometheus-server.monitoring.svc.cluster.local:80/metrics\
  --pvc-size 10Gi
```

```bash
# Deploy to Kubernetes by building from local source path first
kube-watcher deploy \
  --action deploy \
  --target kube \
  --cluster-name dev-cluster \
  --prometheus-endpoint http://prom-prometheus-server.monitoring.svc.cluster.local:80/metrics \
  --source-path /absolute/path/to/kubernetes-anomaly-detection-source \
  --image kube-anomaly-detection:latest
```

```bash
# Run locally in Docker (use local or port-forwarded Prometheus endpoint)
kube-watcher deploy \
  --action deploy \
  --target docker \
  --cluster-name dev-cluster \
  --prometheus-endpoint http://host.docker.internal:9090/metrics
```

```bash
# Cleanup
kube-watcher deploy --action cleanup --target kube --cluster-name dev-cluster
kube-watcher deploy --action cleanup --target docker --cluster-name dev-cluster

# Optional: delete the namespace created for deployment
kube-watcher deploy --action cleanup --target kube --cluster-name dev-cluster --delete-namespace

# Runtime status and logs
kube-watcher deploy --action status --target kube --cluster-name dev-cluster
kube-watcher deploy --action logs --target docker --cluster-name dev-cluster --tail-lines 200 --follow
```

### Explorer CLI (with hotkeys)
The explorer is launched from CLI and serves a local web UI for browsing watcher Badger snapshots/history:

```bash
go run ./watcher/cmd/explorer --db event-engine-badger --listen :4101
```

Then open `http://localhost:4101`.

#### Hotkeys
- `/` focus search field
- `g` focus kind field
- `n` focus namespace field
- `r` refresh resources now
- `j` or `↓` move selection to next resource row
- `k` or `↑` move selection to previous resource row
- `Enter` load history for selected row
- `?` toggle hotkey help

### Available Tools

#### `get_node_status`
Analyzes cluster nodes for health, taints, conditions, and resource allocation.

**Parameters:**
- `include_metrics` (boolean): Include resource metrics [default: true]
- `taints_only` (boolean): Only return tainted nodes [default: false]  
- `node_name` (string): Specific node to analyze [optional]

#### `get_pod_resources`
Monitors pod resource usage, restart counts, and identifies problematic pods.

**Parameters:**
- `namespace` (string): Filter by namespace [optional]
- `status_filter` (string): Filter by pod phase [optional]
- `high_restart_threshold` (number): Restart count threshold [default: 5]
- `include_containers` (boolean): Include container details [default: true]
- `problematic_only` (boolean): Only problematic pods [default: false]

#### `list_namespaces`
Returns namespace inventory with per-namespace pod counts/status breakdowns (and optional quota analysis).

**Parameters:**
- `include_system` (boolean): Include system namespaces like `kube-system` [default: false]
- `include_quotas` (boolean): Include ResourceQuota details [default: false]

#### `get_pod_logs`
Fetches logs for a specific pod/container in a namespace.

**Parameters:**
- `namespace` (string): Namespace containing the pod [required]
- `pod_name` (string): Pod name [required]
- `container` (string): Container name for multi-container pods [optional]
- `tail_lines` (number): Number of trailing log lines to return [default: 200]
- `since_seconds` (number): Only logs newer than this many seconds [default: 0]
- `previous` (boolean): Return logs for previous container instance [default: false]

#### `analyze_cluster`
Comprehensive cluster analysis with health scoring and recommendations.

**Parameters:**  
- `include_pods` (boolean): Include pod analysis [default: true]
- `include_events` (boolean): Include event analysis [default: true]
- `event_hours_back` (number): Hours of events to analyze [default: 24]
- `include_services` (boolean): Include service analysis [default: false]
- `detailed_analysis` (boolean): Generate recommendations [default: true]

## MCP Integration

## Security

### Secret Management
- **Webhook URLs and API tokens**: Use environment variables instead of command-line arguments to prevent exposure in process listings.
- **Google Chat Webhook**: Set `GOOGLE_CHAT_WEBHOOK_URL` environment variable instead of using `--google-chat-webhook-url` flag.
- **ChatBridge tokens**: Configure via `auth_token_env` and `webhook_url_env` fields in YAML configuration to read from environment variables.
- **Model API keys**: Use `api_key_env` configuration to read from environment variables.

### Input Validation
- Kubernetes resource names (namespaces, deployment names) are validated as DNS labels to prevent injection attacks.
- Shell command execution uses secure `exec.Command` with separate arguments instead of shell string evaluation.

### Security Best Practices
- Run with minimal required Kubernetes RBAC permissions
- Regularly update dependencies for security patches
- Monitor logs for unauthorized access attempts
- Use network policies to restrict access to monitoring components

### Audit Logging & RBAC Validation
- **Audit Logging**: All Kubernetes API operations are logged with structured audit events including:
  - Timestamp, resource type, namespace, name, action, and outcome
  - Service account used for the operation
  - Success/failure status and error details
  - Events are logged at INFO level for successes and WARN level for failures/denials
- **RBAC Validation**: Optional pre-flight RBAC checks using Kubernetes SubjectAccessReview API
  - Enabled via `KUBE_WATCHER_AUDIT_RBAC_ENABLED=true` environment variable
  - Service account name can be set via `KUBE_WATCHER_SERVICE_ACCOUNT` (default: "unknown")
  - Helps identify permission issues before actual API calls
- **Audit Events Include**:
  - Authentication events (authn)
  - Authorization events (authz)  
  - Resource access events (access)
  - Configuration changes (config)
  - Security events (security)

## Configuration

The application automatically detects Kubernetes configuration:

1. **In-cluster**: Uses service account when running inside Kubernetes
2. **Local**: Uses `~/.kube/config` for local development
3. **Custom**: Specify custom kubeconfig path via `kubernetes.NewClientFromConfig()`

If your kubeconfig is not at `${HOME}/.kube/config`, update `KUBECONFIG_PATH` in `Makefile`.

## Health and Monitoring

The server provides comprehensive health checks:

- **Server Status**: Application health
- **Kubernetes Connectivity**: Cluster access verification  
- **Tool Registration**: Available tools count
- **Cluster Information**: Basic cluster metrics



**Examples:**
```bash
# Development mode with colored console output
DEBUG=true go run main.go --health

# Production mode with JSON logs
LOG_FORMAT=json LOG_LEVEL=info go run main.go --health

# Debug level logging
LOG_LEVEL=debug go run main.go --exec analyze_cluster
```

## Alert dashboard UI module
For the Nuxt 3 + Naive UI alert dashboard setup and end-to-end wiring (`kube-watcher` → dashboard API → MCP/agent RCA), see `dashboard/README.md`.

## Integration tests (engine → UI → MCP channel)
Run the watcher integration test that validates alert flow from engine dispatch to UI ingest webhook and MCP enrichment handoff:

```bash
go test ./watcher/internal/integration -run 'TestChannelEngineToUIAnd.*Integration' -v
```
