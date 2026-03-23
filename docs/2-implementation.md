# kube-watcher — Implementation Deep-Dive

This document walks through each subsystem in detail: what it does, how it fits into the broader architecture, and the key design decisions.

## Repository Layout

```
kube-watcher/
├── mcp/                          # MCP server application
│   ├── cmd/server/main.go        # CLI: flags, startup, action routing
│   ├── server/server.go          # MCPServer: tool registry, alert loop, MCP protocol
│   ├── api/api.go                # REST API (chi router): /v1/tools, /v1/alerts, /v1/history
│   ├── tools/                    # 8 tool implementations, each with Execute()
│   │   ├── base_tool.go          # Shared BaseTool embedding + arg helpers
│   │   ├── node_status_tool.go
│   │   ├── pod_resources_tool.go
│   │   ├── cluster_analysis_tool.go
│   │   ├── cluster_events_tool.go
│   │   ├── namespace_list_tool.go
│   │   ├── recommendation_tool.go
│   │   ├── history_tool.go
│   │   └── version_tool.go
│   └── monitoring/
│       ├── history/history.go    # BadgerDB-backed incident store
│       └── recommendation/engine.go  # Alert → remediation steps
│
├── watcher/                      # Event engine application
│   ├── cmd/engine/main.go        # CLI: flags, pipeline construction, HTTP servers
│   ├── cmd/explorer/main.go      # Badger Explorer web UI
│   └── internal/
│       ├── config/types.go       # YAML config model + loader
│       ├── source/informer.go    # Kubernetes SharedInformer event source
│       ├── pipeline/pipeline.go  # Source → Filter → Enrich → Evaluate → Dispatch
│       ├── pipeline/enricher.go  # Extracts numeric fields from raw k8s objects
│       ├── rules/engine.go       # Condition/CEL evaluation + timer tracking
│       ├── actions/dispatcher.go # log / webhook dispatch with throttling
│       ├── tracker/              # Snapshot storage (memory + Badger)
│       ├── graph/renderer.go     # Spark-line PNG generation from history
│       ├── monitoring/metrics/exporter.go  # Prometheus gauges
│       └── events/events.go      # ResourceEvent type definition
│
├── pkg/                          # Shared libraries
│   ├── kube/client.go            # Kubernetes API wrapper (ClientInterface)
│   ├── kube/types.go             # NodeInfo, PodInfo, EventInfo, etc.
│   ├── kube/watch/watch.go       # Watch Manager: informers + scanner + dedup
│   ├── convert/numeric.go        # Shared ToFloat64 (handles string, int, float)
│   └── logging/logger.go         # slog-based logger with file output
│
├── docker/
│   ├── Dockerfile.template       # Multi-stage build (builder + debug + final)
│   └── compose.yaml              # MCP + watcher + Prometheus + Grafana + Explorer
│
└── Makefile                      # build, test, lint, run, Docker targets
```

## 1. MCP Server (`mcp/`)

### Startup Flow

`mcp/cmd/server/main.go` parses flags and routes to one of four actions via a `switch` statement:

1. `--list-tools` → prints registered tools.
2. `--exec <tool> --args '{}'` → runs a single tool and prints the result.
3. `--health` → calls `HealthCheck()` and exits 0/1.
4. (default) → calls `server.Start(ctx)` which runs the MCP stdio transport and alert loop.

### MCPServer

`mcp/server/server.go` is the central coordinator:

- **Tool registry**: iterates over all tool constructors in `setupTools()`, calls `registerTool()` for each. Every tool gets a JSON Schema generated from its `Parameters()` slice and a handler that unmarshals args → calls `Execute()` → marshals the result.
- **Alert loop**: `Start()` launches the Watch Manager, spawns `listenForAlerts()` which reads from the alert channel, generates a `Recommendation` via the engine, prepends the `AlertRecord` to an in-memory ring buffer (max 100), persists to BadgerDB history, and sends an MCP resource-updated notification.
- **Public API surface**: `ExecuteTool()`, `HealthCheck()`, `AlertsSnapshot()`, `IncidentHistory()`, `ToolSummaries()`, `ListTools()`.

### REST API

`mcp/api/api.go` wraps the MCPServer behind a chi HTTP router:

- `GET /healthz`, `GET /readyz` — liveness/readiness probes with pluggable `Checker` interface.
- `GET /v1/tools` — list tool summaries.
- `POST /v1/tools/{tool}` — execute a tool with JSON body args.
- `GET /v1/alerts` — current alert ring buffer.
- `GET /v1/history?window=72h` — incident history from BadgerDB.
- `GET /v1/status` — service name, version, status.

Error responses are **sanitized**: the `respondError` helper logs the real error but returns only a generic message to clients.

### Tools

Every tool implements the `server.Tool` interface:

```go
type Tool interface {
    Name() string
    Description() string
    Parameters() []tools.ToolParameter
    Execute(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error)
}
```

All tools embed `BaseTool` which provides `GetStringArg`, `GetBoolArg`, `GetIntArg` helpers and holds the `kube.ClientInterface`.

Key implementation details per tool:

- **NodeStatusTool**: calls `GetNodes()` or `GetNode(name)`, classifies each node as Ready/NotReady by inspecting `Conditions`, computes summary with unhealthy list, provides an overall `analysis.status`.
- **PodResourcesTool**: calls `GetPods(ns)` or `GetPodsAllNamespaces()`, filters by phase/status/restarts, identifies problematic pods (Pending with no node, Failed, high restarts).
- **ClusterAnalysisTool**: calls `GetClusterInfo()` + `GetNodes()`, computes `ready/total` ratio, marks `healthy` vs `degraded`.
- **RecommendationTool**: validates required `kind` + `name` args, constructs a `watch.Alert`, delegates to `recommendation.Engine.ForAlert()`.
- **HistoryInsightsTool**: calls `history.ListIncidents()` with optional kind/severity filters and a limit.

### History Store

`mcp/monitoring/history/history.go` wraps BadgerDB:

- **Key format**: `{kind}:{timestamp_nanos_big_endian}:{id}` — this allows prefix-scanning by kind and range-scanning by time.
- **Record()**: marshals `Incident` to JSON, builds the key, writes via `txn.Set`.
- **List()**: iterates keys with the given kind prefix between `start` and `now`, deserializes each value.
- **CompareFrequency()**: counts entries in two non-overlapping time ranges and computes percent change.
- **GC**: `StartGC()` runs a periodic loop that deletes entries older than the configured retention.

### Recommendation Engine

`mcp/monitoring/recommendation/engine.go`:

- Takes an `alert`, calls `store.CompareFrequency()` to get trend data.
- Builds evidence map from alert fields (namespace, name, reason, message).
- Selects remediation steps via `stepsForAlert()` switch on `alert.Kind`.
- Returns a `Recommendation` struct with title, summary, severity, steps, evidence, and frequency delta.

## 2. Watcher Event Engine (`watcher/`)

### Startup Flow

`watcher/cmd/engine/main.go`:

1. Parse flags (`--config`, `--debug`, `--listen`, `--health`).
2. Load YAML config via `config.Load()` (supports multiple config paths, merging).
3. Initialize Kubernetes client, BadgerDB or in-memory store, optional CEL environment.
4. Build the `pipeline.Pipeline` with all stages.
5. Start internal API server (health/ready) and optional Prometheus metrics server.
6. `pipe.Start(ctx)` — blocks until context cancellation (SIGINT/SIGTERM).

### Pipeline

`watcher/internal/pipeline/pipeline.go` implements a staged event processing pipeline:

```
InformerSource → queue (chan) → worker pool → Filter → Enrich → Evaluate → Dispatch
                                                                    ↓
                                                              record metrics
                                                              notify observers
```

- **Source**: `InformerSource` registers `SharedIndexInformer` handlers for Pods, HPAs, and Nodes. Events are converted to `ResourceEvent` structs and pushed to a buffered channel.
- **Worker pool**: uses `ants` goroutine pool (default: `runtime.NumCPU()` workers) to process events concurrently.
- **Filter**: `RuleAwareFilter` checks if any rule's kind+namespace matches the event; unmatched events are dropped early.
- **Enricher**: `PodEnricher` extracts numeric fields (restart_count, cpu_request, etc.) from raw Kubernetes objects into flat `map[string]interface{}` for rule evaluation.
- **Metrics**: `MetricStore.Observe()` computes delta and rate-per-second for tracked fields. Deltas are injected back into the event object.
- **Observers**: pluggable (e.g., `metrics.Exporter` for Prometheus gauges).

### Rules Engine

`watcher/internal/rules/engine.go`:

- **Evaluate()**: iterates all rules, checks kind+namespace match, then either evaluates the CEL expression or iterates conditions.
- **CEL path**: pre-compiled programs are stored in a `sync.Map`; evaluation passes `evt`, `kind`, `ns`, `name` as variables.
- **Condition path**: for each condition, extract the field via gjson, compare using the operator. `changed` operator compares against the previously stored snapshot.
- **Timer tracking**: rules with `for > 0` use `sync.Map` to track when conditions first became true. Only fires after the duration elapses with continuous matches.
- **Action mapping**: matched rules produce `ActionInvocation` structs containing the rule name, action config, event context, and values.
- **Persistence**: after evaluation, current field values are stored as a `Snapshot` in the tracker store for future `changed` comparisons.

### Dispatcher

`watcher/internal/actions/dispatcher.go`:

- Uses an `ants` goroutine pool for async action execution.
- **log**: renders Go template against the invocation context, logs the result.
- **notification**: renders template, POSTs JSON `{"text": "..."}` to the configured webhook URL (Google Chat format). Uses `task.ctx` for cancellation.
- **Throttling**: `isAllowed()` enforces per-minute or per-hour rate limits per rule+action pair.

### Tracker Store

Two implementations of `tracker.Store`:

- **MemoryStore** (`tracker/memory.go`): `sync.RWMutex`-protected maps for current snapshots and bounded history slices.
- **BadgerStore** (`tracker/badger.go`): persists snapshots to disk with TTL-based history entries. History keys use `{resourceKey}:{timestamp_nanos}` for ordered time-series queries.

### Graph Renderer

`watcher/internal/graph/renderer.go`: takes a field name and snapshot history, extracts numeric values, generates a PNG spark-line chart using `go-chart`. Attached to action invocations when a graph action is configured.

### Prometheus Exporter

`watcher/internal/monitoring/metrics/exporter.go`: implements the `Observer` interface. Exports two Prometheus gauges:

- `kube_watcher_pod_restart_count{namespace, pod}` — current restart count.
- `kube_watcher_pod_cpu_limit_gap_milli{namespace, pod}` — difference between CPU limit and request in millicores.

## 3. Shared Libraries (`pkg/`)

### Kubernetes Client

`pkg/kube/client.go` implements `ClientInterface` — a high-level wrapper over `client-go` that returns domain types (`NodeInfo`, `PodInfo`, etc.) instead of raw Kubernetes objects. Supports both in-cluster and kubeconfig-based authentication.

### Watch Manager

`pkg/kube/watch/watch.go`:

- Combines **SharedInformers** (real-time pod and event updates) with a **polling scanner** (periodic node status checks).
- Emits `Alert` structs on a buffered channel (128).
- Deduplicates using `sync.Map` with a 5-minute window.
- `PodEvaluator` is a typed function `Evaluator[*corev1.Pod]` that checks restart count and waiting state.

### Numeric Conversion

`pkg/convert/numeric.go`: single `ToFloat64(v interface{}) (float64, bool)` function that handles `float64`, `float32`, `int`, `int64`, `int32`, `string` (via `strconv.ParseFloat`), and `json.Number`. Used by the rules engine, pipeline, graph renderer, and metrics exporter.

### Logger

`pkg/logging/logger.go`: creates a structured `slog.Logger` with optional file output (permissions `0600`). Debug mode enables `slog.LevelDebug`.

## 4. Configuration System

`watcher/internal/config/types.go`:

- **Config discovery**: if no `--config` flag is given, searches for `config.yaml` and `event-engine.yaml` by walking up from the working directory and executable directory.
- **Multi-file merging**: multiple config files are loaded and merged (rules and actions are appended/merged).
- **Validation**: rules must have names and actions; referenced action IDs must exist in the actions map.
- **Defaults**: retention defaults to 1h, storage defaults to "memory", queue depth defaults to 128.

## 5. Badger Explorer

`watcher/cmd/explorer/main.go`: a standalone HTTP server that opens the watcher's BadgerDB in read-only mode and provides:

- A server-rendered HTML UI for browsing stored resource snapshots.
- `GET /api/resources?search=&kind=&namespace=&limit=50` — list snapshots.
- `GET /api/history?key=pod/default/api&limit=120` — time-series history for a resource.

This is useful for debugging what the watcher engine has tracked.
