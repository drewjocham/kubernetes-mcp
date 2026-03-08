# Kube-Watcher Alert Dashboard (Nuxt 3 + Naive UI)
This dashboard is the UI/API module for the full incident workflow:
- **Observer**: `kube-watcher` detects failures (CrashLoopBackOff, OOMKilled, ImagePullBackOff)
- **Thinker**: MCP endpoint enriches alert with logs, `describe`, and cluster events
- **Actor**: Agent endpoint generates RCA + remediation actions
- **UI**: realtime alert state updates via SSE (`detected` → `thinking` → `report_ready`)

## 1) Install and run the dashboard
From repository root:

```bash
npm install --prefix dashboard
npm run --prefix dashboard dev
```

Dashboard runs on `http://localhost:3000` by default.

## 2) Configure the UI workflow endpoints
Open the dashboard and set:
- **MCP diagnostics endpoint**: HTTP endpoint that returns diagnostics JSON
- **Agent RCA endpoint**: HTTP endpoint that returns RCA JSON
- Optional API keys for each endpoint

Configuration is stored in-memory in the dashboard server process.

## 3) Wire kube-watcher to the dashboard API
Use a `webhook` action in watcher config (`watcher/internal/config/config.yaml` or your custom config):

```yaml
actions:
  dashboard-webhook:
    type: webhook
    config:
      url: "http://localhost:3000/api/alerts/ingest"
    template: |
      {
        "kind": "CrashLoopBackOff",
        "cluster": "prod-eu",
        "namespace": "{{ .Event.Namespace }}",
        "pod": "{{ .Event.Name }}",
        "container": "",
        "ruleName": "{{ .RuleName }}",
        "severity": "critical",
        "source": "kube-watcher"
      }

rules:
  - name: Pod-Restarting-Frequently
    kind: Pod
    condition: "evt.restart_delta > 0 && evt.waiting_reason == 'CrashLoopBackOff'"
    actions:
      - dashboard-webhook
```

Then run watcher:

```bash
go run ./watcher/cmd/engine --config watcher/internal/config/config.yaml
```

## 4) API contract used by the dashboard
### Ingest endpoint
- `POST /api/alerts/ingest`
- Body:

```json
{
  "kind": "CrashLoopBackOff",
  "cluster": "prod-eu",
  "namespace": "payments",
  "pod": "checkout-api-7ff98f4bcf-6jwkq",
  "container": "checkout-api",
  "ruleName": "Pod-Restarting-Frequently",
  "severity": "critical",
  "reason": "restart spike detected",
  "source": "kube-watcher"
}
```

### Realtime stream
- `GET /api/alerts/stream` (SSE)
- emits events: `ready`, `alerts`, `heartbeat`

### Read APIs
- `GET /api/alerts`
- `GET /api/alerts/:id`
- `GET /api/config`
- `POST /api/config`

## 5) Expected MCP and Agent response shapes
### MCP diagnostics response
Any JSON containing these fields is supported:

```json
{
  "lastLogLines": ["..."],
  "describeOutput": "...",
  "clusterEvents": ["..."]
}
```

### Agent RCA response

```json
{
  "generatedAt": "2026-03-08T10:00:00Z",
  "summary": "Short incident summary",
  "rootCause": "Primary cause",
  "recommendedActions": ["Action 1", "Action 2"],
  "confidence": "medium"
}
```

If endpoints are not configured, the dashboard uses local fallback diagnostics/RCA generation so you can still test the flow.

## 6) Quick end-to-end test (without kube-watcher)
Trigger a synthetic alert:

```bash
curl -X POST http://localhost:3000/api/alerts/ingest \
  -H 'content-type: application/json' \
  -d '{
    "kind":"OOMKilled",
    "cluster":"dev-cluster",
    "namespace":"orders",
    "pod":"orders-api-abc123",
    "source":"manual",
    "reason":"container killed by OOM"
  }'
```

Then open the UI and verify:
- row appears as `detected`
- transitions to `thinking`
- final RCA appears as `report_ready`

## 7) Troubleshooting
- **No alerts in UI**: verify watcher action URL and dashboard is running on the same reachable host.
- **Alert stuck in `thinking`**: check configured MCP/agent endpoints; if unreachable, clear endpoints to use fallback mode.
- **Webhook non-2xx in watcher logs**: inspect dashboard API logs and payload template JSON validity.
- **No realtime updates**: confirm browser can reach `/api/alerts/stream` and no proxy buffering SSE.

## 8) Production recommendations
- Persist alerts/reports in Postgres or Mongo instead of in-memory store.
- Add auth in front of `/api/alerts/ingest` and `/api/config`.
- Add rate limits and idempotency keys for repeated watcher events.
- Place dashboard + watcher behind internal network policies/TLS.
