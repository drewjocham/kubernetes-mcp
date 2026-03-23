# kube-watcher — High-Level Overview

## What It Is

kube-watcher is a Kubernetes monitoring platform that combines two independent applications:

1. **MCP Server** — exposes cluster state to AI assistants (and a REST API) via the [Model Context Protocol](https://modelcontextprotocol.io).
2. **Watcher Event Engine** — a real-time rule engine that watches Kubernetes resources, evaluates user-defined rules, and fires actions (log, webhook, alert).

Both applications share the same Go module and reusable `pkg/` libraries but are compiled and deployed separately.

```
                                  ┌──────────────────┐
         kubectl / AI assistant   │  MCP Server      │
         ───────────────────────▶ │  (stdio / HTTP)  │
                                  │                  │
                                  │  ┌────────────┐  │
                                  │  │ Tools      │  │  get_node_status
                                  │  │ (7 total)  │◀─┤  get_pod_resources
                                  │  └────────────┘  │  analyze_cluster
                                  │  ┌────────────┐  │  cluster_events
                                  │  │ Watch Mgr  │──┤  namespaces
                                  │  └────────────┘  │  recommendations
                                  │  ┌────────────┐  │  history_insights
                                  │  │ History DB │  │  version
                                  │  │ (BadgerDB) │  │
                                  │  └────────────┘  │
                                  └──────────────────┘

                                  ┌──────────────────┐
         k8s API server           │  Watcher Engine   │
         ◀────────────────────── │  (informers)      │
                                  │                  │
                                  │  ┌────────────┐  │
                                  │  │ Pipeline   │  │  Source → Filter → Enrich
                                  │  └──────┬─────┘  │         → Evaluate → Dispatch
                                  │         │        │
                                  │  ┌──────▼─────┐  │
                                  │  │ Rules Eng. │  │  YAML conditions + CEL
                                  │  └──────┬─────┘  │
                                  │         │        │
                                  │  ┌──────▼─────┐  │
                                  │  │ Dispatcher │  │  log / webhook / alert
                                  │  └────────────┘  │
                                  └──────────────────┘
```

## Core Concepts

### MCP Tools

The MCP server registers **7 tools** that an AI assistant (or HTTP client) can invoke:

| Tool | Purpose |
|------|---------|
| `get_node_status` | Node health, taints, allocatable resources |
| `get_pod_resources` | Pod status, restart tracking, problem detection |
| `analyze_cluster` | Full cluster health score with recommendations |
| `cluster_events` | Warning/Normal events with filtering |
| `namespaces` | Namespace listing with pod counts |
| `get_recommendation` | AI-friendly remediation steps for an alert |
| `history_insights` | Incident trend analysis from BadgerDB |
| `get_server_version` | Build metadata |

### Alert Detection

The MCP server's **Watch Manager** polls the Kubernetes API on a configurable interval (default 30 s) and also uses **SharedInformers** for real-time push:

- **Nodes**: any node whose status ≠ `Ready` emits a `critical` alert.
- **Pods**: containers with `RestartCount > 5` or `CrashLoopBackOff` emit a `high`/`critical` alert.
- **Events**: any `Warning`-type event is forwarded.

Alerts are **deduplicated** over a 5-minute sliding window using a `sync.Map` keyed by `kind:namespace:name:reason`.

### Watcher Rule Engine

The watcher evaluates rules from a YAML config against live Kubernetes objects. Each rule specifies:

- **kind** — the Kubernetes resource type (Pod, Node, HPA, …).
- **namespace** — optional scope filter.
- **conditions** — field-level comparisons (`eq`, `ne`, `gt`, `lt`, `changed`).
- **expression** — an optional CEL expression for complex logic.
- **duration** (`for`) — how long conditions must hold before firing.
- **logic** — `all` (default) or `any` for condition aggregation.
- **actions** — which action IDs to invoke on match.

## Math & Logic

### Frequency Comparison (Trend Detection)

The history store's `CompareFrequency` method compares incident counts across two time windows to detect whether a problem is getting worse:

```
recent_window  = [now − 6h,  now]        → recentCount
previous_window = [now − 30h, now − 6h]  → previousCount

                  recentCount − previousCount
percentChange = ───────────────────────────── × 100
                      previousCount

(when previousCount = 0 and recentCount > 0, percentChange = 100.0)
(when both are 0, percentChange = 0.0)
```

This is exposed in the `history_insights` tool and powers the recommendation engine's `FrequencyDelta` field.

### Rule Condition Evaluation

Each condition compares a field from the live Kubernetes object (extracted via gjson path) against a threshold:

```
value = gjson.GetBytes(objectJSON, condition.Field)

result = compare(value, condition.Value)

where compare(a, b):
  if both numeric: return sign(toFloat(a) − toFloat(b))
  else:            return strings.Compare(fmt.Sprint(a), fmt.Sprint(b))
```

Operators: `eq` (== 0), `ne` (!= 0), `gt` (> 0), `lt` (< 0), `changed` (differs from stored snapshot).

When `logic = "all"`: every condition must pass (short-circuit false).
When `logic = "any"`: at least one must pass (short-circuit true).

### Duration-Based Firing (`for` field)

Rules with a `for` duration use a **timer map** keyed by `resourceKey|ruleName`:

```
on first match: store timestamp, do NOT fire
on subsequent match:
  if now − storedTimestamp ≥ rule.For → fire, delete timer
  else → do not fire
on non-match: delete timer
```

This prevents transient blips from triggering actions.

### Throttle Logic

Actions carry per-minute and per-hour throttle limits:

```
key = "ruleName:actionID"
window = 1 min (if maxPerMinute > 0), else 1 hour

cooldown = window / limit

if timeSince(lastRun[key]) < cooldown → suppress
else → allow, update lastRun
```

### Metric Delta Tracking

The watcher pipeline computes deltas for numeric fields (restart_count, replicas, etc.):

```
delta    = currentValue − previousValue
rate/sec = delta / elapsed_seconds

These are injected into the event object as `{field}_delta` before rule evaluation.
```

### Recommendation Steps

The recommendation engine maps `alert.Kind` to a static set of remediation steps:

- **node** → check kubelet, inspect infrastructure, drain/cordon
- **pod** → inspect events, validate resource limits, restart/rollback
- **event** → review warnings, audit controllers
- **unknown** → generic manual review

The `FrequencyDelta` from history is attached so the consumer knows if the problem is trending up or down.

## Deployment Options

1. **Standalone binary** — `make build-mcp` / `make build-watcher`
2. **Docker Compose** — `make up` (MCP + watcher + Prometheus + Grafana + Badger Explorer)
3. **Homebrew** — `brew install drewjocham/tap/kube-watcher` (see Homebrew section in repo)

## Configuration

- **MCP server**: command-line flags (`--db-path`, `--interval`, `--debug`, `--log-file`).
- **Watcher engine**: YAML config file with `resource_tracking`, `rules`, `actions`, and `settings` sections. See `watcher/internal/config/config.yaml` for the default.
