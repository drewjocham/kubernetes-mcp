# AGENTS.md: Development & Review Guidelines

## 1. System Overview & Scope
This ecosystem manages Kubernetes observability and automation across three tiers:
* **MCP (Mesh Control Protocol):** Central plane for 3rd-party sync (Google/iCloud), Markdown rendering, and UI widgets.
* **Watcher:** Modular monitoring agent for files, K8s ops logs, and provider-specific changes.
* **Kube-Watcher-App:** User interface for cluster alerts (pager) and searchable, toggleable streaming logs.

## 2. Development Commands
### Build
* `make build` – Build all binaries (MCP, CLI, watcher, deploy, chatbridge)
* `make build-mcp` – Build MCP server binary (`kube-watcher`)
* `make build-cli` – Build unified CLI (`kw-cli`) and symlink `kw`
* `make build-watcher` – Build watcher engine (`watcher-engine`)
* `make build-desktop-app` – Build Wails desktop app (requires `kube-watcher-app` directory)

### Run
* `make run-server` – Run MCP HTTP server on `:8080` (required for desktop app)
* `make run-watcher` – Run watcher engine locally (loads `watcher/internal/config/config.yaml` if no `--config`)
* `make desktop-dev` – Start desktop app in dev mode (requires MCP server running)
* `make terminal` / `run-terminal` – Launch terminal TUI via `kw terminal`

### Testing & Quality
* `make test` – Run all unit tests
* `make lint` – Run `golangci-lint` (must pass 100%)
* `make fmt` – Format Go code
* `make vet` – Go vet
* `make test-integration-channel` – Integration test for engine→UI→MCP alert flow

### Deployment & Operations
* `make up` / `down` – Docker Compose stack (MCP + watcher)
* `make start-all` / `stop-all` – Full stack + dashboard dev server
* `make helm-install` – Install full stack via Helm (`kube-watcher` namespace)
* `make k8s-pf-mcp` – Port-forward MCP service to `localhost:8080`

## 3. Architecture Notes
* **Entrypoints:**
  * CLI: `main.go` → `cli.Execute()` → `cmd.Execute()` (root command `kw`)
  * MCP server: `mcp/cmd/server` (`--http-addr :8080`)
  * Watcher engine: `watcher/cmd/engine` (`--config` defaults to internal config)
  * Desktop app: `kube-watcher-app` (Wails + Vue)
* **Dependencies:**
  * `make desktop-dev` requires MCP server running (`make run-server`) or `KW_TOOLS_ENDPOINT` env var.
  * Watcher engine loads rules from YAML config; default location is `watcher/internal/config/config.yaml`.
  * CLI uses profile management (`kw profile`) for config/env/secrets.
* **Component Boundaries:**
  * `pkg/` – Reusable libraries (kube client, logging, profile, audit)
  * `mcp/` – MCP server implementation and tools
  * `watcher/` – Event engine with rules, pipeline, actions, trackers
  * `services/` – Monitoring services (ingest, probe, processor)
  * `integrations/` – Bridges (Google Chat, etc.)
  * `terminal/` – PTY + Bubble Tea TUI

## 4. AI Code Review Template (MANDATORY)
### Design & Architecture
* **Concurrency:** Audit Goroutine lifecycles and `context.Context` propagation in log streams.
* **Kube-Native Patterns:** Check for `SharedIndexInformers`. Ensure updates to MCP are idempotent.
* **Decoupling:** Verify the `Watcher` remains provider-agnostic via clean interfaces.

### Coding Style & Golang Best Practices
* **DRY Audit:** Identify shared logic between `watcher` and `kube-watcher-app` (e.g., K8s event types).
* **Error Handling:** Use `%w` for wrapping. No "stuttering" (e.g., `log.Error(err); return err`).
* **Performance:** Check for slice/map pre-allocation and efficient regex in log scanning.

### Security & Hardening
* **RBAC:** Verify least-privilege K8s roles (limit to `watch/list` where possible).
* **Data Integrity:** Ensure PII in logs is not persisted to Mongo during alert generation.
* **Resilience:** Ensure `panic` recovery in long-running watcher loops.

## 5. Cursor / Copilot Rules
* **Rule 1 (Context):** Always read `DECISION_LOG.md` before suggesting architectural refactors.
* **Rule 2 (No Stubs):** Do not provide "TODO" blocks for streaming or alert‑highlighting logic.
* **Rule 3 (Formatting):** Follow `anomstack/.cursor/rules/` for specific linting and import ordering.
* **Rule 4 (DRY):** If a new utility is needed, check existing packages in `pkg/` and `internal/` before creating a local helper.

## 6. Key Feature Logic
* **Streaming Logs:** Must support search and `ON/OFF` toggles without restarting the watcher.
* **Log‑Based Alerts:** Highlighting a string in the UI must trigger a direct write to the built‑in Tasks/Mongo store.
* **Integrations:** Google Workspace and iCloud sync are “first‑class”—prioritize robust OAuth2 handling.

## 7. Workflow Gotchas
* **MCP Server Required:** Desktop app (`make desktop-dev`) will fail with “Failed to load workspace” if MCP server is not reachable. Run `make run-server` first.
* **Kubeconfig:** Ensure `~/.kube/config` exists or set `KUBECONFIG_PATH`. In‑cluster config is used when running inside Kubernetes.
* **Profile Overrides:** CLI config can be overridden by active profile (`kw profile use`). Check `~/.kube‑watcher/profiles.yaml`.
* **Secret Management:** Webhook URLs and API tokens should be passed via environment variables, not command‑line flags.

---

*Last updated based on codebase inspection 2025‑04‑12.*
