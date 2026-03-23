# kube-watcher — Low-Level Internals

This document covers the byte-level data structures, storage layouts, goroutine model, concurrency patterns, and protocol details.

## 1. Goroutine Model

### MCP Server Goroutines

When `server.Start(ctx)` is called, the following goroutines are active:

```
main goroutine
  └── server.Start(ctx)
        ├── watchManager.Start(ctx)
        │     ├── informer factory goroutines (client-go internal)
        │     │     ├── pod informer reflector + processor
        │     │     └── event informer reflector + processor
        │     └── runScanner goroutine (ticker loop, 30s interval)
        │           └── performScan → GetNodes() → emit()
        ├── listenForAlerts goroutine
        │     └── select { case alert: processAlert() }
        └── mcp.Server.Run(ctx, StdioTransport)
              └── stdio read/write goroutines (MCP SDK internal)
```

Total steady-state goroutines: ~8–12 (varies with client-go informer internals).

### Watcher Engine Goroutines

```
main goroutine
  └── engineApp.run()
        ├── metricStore.CleanupLoop goroutine (5-min ticker)
        ├── HTTP server goroutine (internal API :8085)
        ├── HTTP server goroutine (metrics :9095, optional)
        ├── HTTP server shutdown goroutine (per server)
        └── pipeline.Start(ctx)
              ├── source.Run goroutine
              │     └── informer factory goroutines (Pods, HPAs, Nodes)
              └── ants worker pool (runtime.NumCPU() goroutines)
                    └── each worker: processEvent() → filter → enrich → evaluate → dispatch
                          └── dispatcher.pool (ants pool, 64 goroutines)
                                └── each worker: execute() → handleLog/handleNotification
```

Total steady-state goroutines: ~20–30+ depending on CPU count and pool sizes.

## 2. Concurrency Patterns

### sync.Map Usage

- **Watch Manager `history`**: stores `key → time.Time` for alert deduplication. Lock-free reads dominate (most alerts are not duplicates). Entries are never explicitly deleted; the map grows slowly since keys are bounded by the number of distinct alert keys in the cluster.
- **Rules Engine `timers`**: stores `resourceKey|ruleName → time.Time` for duration-based firing. Deleted on non-match or after firing. Write-heavy during rule evaluation but partitioned by key.
- **Rules Engine `celPrograms`**: write-once at startup, read-only during evaluation.

### sync.RWMutex Usage

- **MCPServer `alertsMu`**: protects the `[]AlertRecord` ring buffer. `RLock` for `AlertsSnapshot()`, `Lock` for `processAlert()`.
- **MemoryStore `mu`**: protects snapshot map and history slices. Read-heavy during rule evaluation, write on each event.
- **MetricStore `mu`**: protects metric point map. Write on each `Observe()`, read during `evictExpired()`.

### Channel Usage

- **Watch Manager `out`**: `chan Alert` buffered at 128. Non-blocking send with drop on full.
- **Pipeline `queue`**: `chan ResourceEvent` buffered at configurable depth (default 128). Source writes, worker pool reads.

### Worker Pool (ants)

Both the pipeline and dispatcher use `panjf2000/ants` goroutine pools:

- **Pipeline pool**: `ants.NewPoolWithFunc(workers, ...)` where `workers = runtime.NumCPU()`.
- **Dispatcher pool**: `ants.NewPoolWithFunc(64, ...)`.

Tasks are submitted via `pool.Invoke(task)`. If the pool is full, `Invoke` blocks until a worker is available.

## 3. BadgerDB Storage Layout

### History Store (MCP Server)

Database path: `~/.kube-watcher/history.db` (configurable via `--db-path`).

**Key schema**:

```
┌─────────────────────┬──────────────────┬─────────────────┐
│   kind (string)     │  : (separator)   │ timestamp (8B)  │  : │ id (string)
│   e.g. "pod_anomaly"│                  │ big-endian nanos│     │ uuid
└─────────────────────┴──────────────────┴─────────────────┘
```

Example key bytes: `pod_anomaly:0000018E5A3B1C00:550e8400-e29b-41d4-a716-446655440000`

**Value**: JSON-encoded `history.Incident`:

```json
{
  "id": "550e8400-...",
  "timestamp": "2026-03-07T10:00:00Z",
  "kind": "pod_anomaly",
  "severity": "high",
  "namespace": "default",
  "name": "api-server",
  "reason": "CrashLoopBackOff",
  "message": "container app: 12 restarts",
  "occurrences": 1,
  "metadata": {}
}
```

**Scan patterns**:

- `List(kind, since)`: seek to `{kind}:{startTimestamp}`, iterate forward until `{kind}:{nowTimestamp}`.
- `CompareFrequency(kind, recent, previous)`: two `countRange` calls, each iterating the key range without deserializing values (`PrefetchValues: false`).
- `GC`: iterate all keys per kind prefix, delete any with timestamp before `now - retention`.

### Tracker Store (Watcher Engine)

Database path: configurable in YAML `resource_tracking.path` (default: `event-engine-badger`).

**Current snapshot key**: `{kind}/{namespace}/{name}` (e.g., `pod/default/api-server`).

**History entry key**:

```
┌───────────────────────────────┬───┬──────────────────┐
│   resource key (string)       │ : │ timestamp (8B)   │
│   e.g. "pod/default/api"     │   │ big-endian nanos  │
└───────────────────────────────┴───┴──────────────────┘
```

**Value**: JSON-encoded `tracker.Snapshot`:

```json
{
  "resourceVersion": "12345",
  "values": {
    "restart_count": 5,
    "cpu_request": "100m",
    "desired_replicas": 3
  },
  "timestamp": "2026-03-07T10:00:00Z"
}
```

History entries use BadgerDB TTL (`WithTTL(historyTTL)`) for automatic expiry (default 1h).

**Scan pattern for History()**: seek to `{key}:0xFFFFFFFFFFFFFFFF` in reverse, collect up to `limit` entries. This returns most-recent-first, then the result is reversed to chronological order.

## 4. MCP Protocol Details

The MCP server uses the `modelcontextprotocol/go-sdk` library over **stdio transport** (stdin/stdout JSON-RPC 2.0).

### Registered Resources

| URI | Name | MIME |
|-----|------|------|
| `kube://alerts/current` | Current Alerts | `application/json` |
| `kube://history/incidents` | Incident History | `application/json` |

Resources are read-only. When a new alert is processed, the server sends a `notifications/resources/updated` notification with the alerts URI.

### Tool Registration

Each tool is registered via `mcp.Server.AddTool()` with:

- A `mcp.Tool` struct containing `Name`, `Description`, and `InputSchema` (JSON Schema object).
- A handler function that:
  1. Unmarshals `req.Params.Arguments` (raw JSON) into `map[string]interface{}`.
  2. Calls `tool.Execute(ctx, args)`.
  3. Marshals the result to JSON and returns it as `mcp.TextContent`.
  4. On error, returns `IsError: true` with the error message.

### JSON Schema Generation

`generateSchema()` converts `[]ToolParameter` to a JSON Schema:

```go
// For each parameter:
props[p.Name] = {"type": mapped_type, "description": p.Description}

// Type mapping:
// "boolean" → "boolean"
// "number"/"integer" → "number"
// everything else → "string"

// Wrapped as:
{"type": "object", "properties": props}
```

## 5. HTTP API Wire Format

### Request/Response Examples

**Execute Tool**:

```
POST /v1/tools/get_node_status
Content-Type: application/json

{"include_metrics": true, "node_name": "worker-1"}
```

```json
{
  "nodes": [{"name": "worker-1", "status": "Ready", ...}],
  "summary": {"total": 1, "ready": 1, "unhealthy": []},
  "analysis": {"status": "healthy"}
}
```

**Error Response** (sanitized):

```json
{"error": "tool execution failed"}
```

The raw error is logged server-side with the request path but never sent to the client.

**Alerts**:

```
GET /v1/alerts
```

```json
{
  "alerts": [
    {
      "alert": {"kind": "pod", "severity": "high", "name": "api", ...},
      "recommendation": {"title": "pod: high", "steps": [...], ...}
    }
  ]
}
```

## 6. Kubernetes API Interactions

### Client Methods and API Calls

| Method | Kubernetes API | Scope |
|--------|---------------|-------|
| `GetNodes()` | `GET /api/v1/nodes` | Cluster |
| `GetNode(name)` | `GET /api/v1/nodes/{name}` | Single |
| `GetPods(ns)` | `GET /api/v1/namespaces/{ns}/pods` | Namespace |
| `GetPodsAllNamespaces()` | `GET /api/v1/pods` | Cluster |
| `GetEvents(ns)` | `GET /api/v1/namespaces/{ns}/events` | Namespace |
| `GetEventsAllNamespaces()` | `GET /api/v1/events` | Cluster |
| `GetNamespaces()` | `GET /api/v1/namespaces` | Cluster |
| `GetClusterInfo()` | `GET /version` + namespace/node counts | Cluster |
| `HealthCheck()` | `GET /api/v1/namespaces` (lightweight) | Cluster |

### Informer Resources

The watcher's `InformerSource` watches:

- `core/v1/pods` — for restart detection, state changes.
- `autoscaling/v2/horizontalpodautoscalers` — for HPA stall detection.
- `core/v1/nodes` — for node condition changes.

The MCP server's `Watch Manager` watches:

- `core/v1/pods` — via UpdateFunc for crash loop detection.
- `core/v1/events` — via AddFunc for warning events.
- Nodes — via periodic poll (not informer), checking status ≠ Ready.

## 7. Configuration YAML Schema

```yaml
resource_tracking:
  enabled: bool          # Enable snapshot storage (default: true)
  storage: string        # "memory" or "badger" (default: "memory")
  path: string           # BadgerDB directory path
  retention: duration    # History TTL (default: "1h")
  fields: []string       # Additional fields to track

rules:
  - name: string         # Unique rule name (required)
    kind: string         # Kubernetes resource kind (required)
    namespace: string    # Scope filter (optional, "" = all)
    logic: string        # "all" (default) or "any"
    duration: duration   # How long conditions must hold (e.g., "5m")
    expression: string   # CEL expression (alternative to conditions)
    conditions:          # Field-level conditions
      - field: string    # gjson path into the resource object
        operator: string # eq, ne, gt, lt, changed
        value: any       # Comparison value
    actions: []string    # Action IDs to fire on match (required)

actions:
  action_id:
    type: string         # "log", "notification", "alert"
    template: string     # Go text/template for message rendering
    throttle:
      max_per_minute: int
      max_per_hour: int
    config:
      url: string        # Webhook URL (for "notification" type)

settings:
  cel:
    enabled: bool        # Enable CEL expression support
  queue_depth: int       # Pipeline buffer size (default: 128)
  metrics:
    enabled: bool        # Enable Prometheus exporter
    listen: string       # Metrics HTTP address (default: ":9095")
```

## 8. Build Artifacts

`make build` produces two binaries:

| Binary | Source | ldflags |
|--------|--------|---------|
| `bin/kube-watcher` | `mcp/cmd/server` | `-X main.version`, `-X main.gitCommit`, `-X main.buildDate` |
| `bin/watcher-engine` | `watcher/cmd/engine` | (none) |

Cross-compilation targets (`make build-all`): `linux/amd64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`.

Docker images use a multi-stage build (`docker/Dockerfile.template`) with `golang:1.25-alpine` builder and `alpine:3.23` runtime. The final image runs as non-root user `65532:65532`.
