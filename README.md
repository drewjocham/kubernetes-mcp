# kube-watcher

A Kubernetes monitoring and analysis MCP (Model Context Protocol) server designed to provide cluster insights and monitoring capabilities.

## Features

- **Node Monitoring**: Detailed analysis of cluster nodes, health status, and resource allocation
- **Pod Analysis**: Comprehensive pod resource monitoring, restart tracking, and issue identification  
- **Cluster Analysis**: Full cluster health assessment with recommendations and alerting

## Architecture

```
kube-watcher/
├── mcp/               # MCP server app
│   ├── cmd/server     # CLI entrypoint
│   ├── server/        # MCP server implementation
│   └── tools/         # Tool definitions
├── watcher/           # Event engine app
│   ├── cmd/engine     # Event-engine entrypoint
│   └── internal/      # Rules, pipeline, actions, trackers
├── monitoring/        # Shared history/recommendation services
├── pkg/               # Reusable libraries (logging, kube client, etc.)
└── main.go            # Convenience wrapper for MCP development
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

## Usage

### Command Line Interface

#### List Available Tools
```bash
go run ./mcp/cmd/server --list-tools
```

```shell
    export KUBECONFIG_PATH="${HOME}/.kube/config"
    docker compose -f docker/compose.yaml up 
```

#### Execute Specific Tools
```bash
# Node status analysis
go run ./mcp/cmd/server --exec get_node_status --args '{"include_metrics":true}'

# Pod resource monitoring
go run ./mcp/cmd/server --exec get_pod_resources --args '{"problematic_only":true}'

# Full cluster analysis  
go run ./mcp/cmd/server --exec analyze_cluster --args '{"include_pods":true,"include_events":true}'
```

#### Health Check
```bash
go run ./mcp/cmd/server --health
```

#### Interactive Server Mode
```bash
go run ./mcp/cmd/server --server
```
### Watcher Event Engine

The watcher analyzes cluster events based on your YAML rules/configuration:

```bash
go run ./watcher/cmd/engine --config /path/to/event-engine.yaml
```

Pass `--debug` or `--listen :8085` to enable verbose logging and health endpoints.
If you omit `--config`, the engine automatically loads `watcher/internal/config/config.yaml`.

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
