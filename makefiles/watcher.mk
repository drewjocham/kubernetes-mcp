
.PHONY: build-watcher \
	run-watcher watcher-up up down start-all stop-all \
	k8s-logs-watcher k8s-logs-explorer \
	k8s-pf-explorer k8s-restart-watcher

# ── Build ──────────────────────────────────────────────────────────────────────

build-watcher:
	@echo "Building $(WATCHER_BINARY_NAME)..."
	$(GOBUILD) -o $(BIN_DIR)/$(WATCHER_BINARY_NAME) $(WATCHER_CMD)

# ── Local Run ──────────────────────────────────────────────────────────────────

run-watcher:
	@echo "Running $(WATCHER_BINARY_NAME)..."
	$(GORUN) $(WATCHER_CMD) $(ARGS)

# ── Docker Compose Stack ───────────────────────────────────────────────────────

watcher-up:
	$(COMPOSE_ENV) docker compose -f $(COMPOSE_FILE) up --build watcher

up:
	$(COMPOSE_ENV) docker compose -f $(COMPOSE_FILE) up --build -d

down:
	$(COMPOSE_ENV) docker compose -f $(COMPOSE_FILE) down

start-all:
	@echo "Starting full stack (Docker + dashboard)..."
	$(COMPOSE_ENV) docker compose -f $(COMPOSE_FILE) up --build -d
	@sleep 5
	@( cd dashboard && CI=true npm run dev > /tmp/kube-watcher-dashboard.log 2>&1 & echo $$! > /tmp/kube-watcher-dashboard.pid )
	@sleep 3
	@echo ""
	@echo "  Dashboard  → http://localhost:3000"
	@echo "  MCP API    → http://localhost:8080"
	@echo "  Metrics    → http://localhost:9095/metrics"
	@echo ""
	@echo "Logs: tail -f /tmp/kube-watcher-dashboard.log"
	@echo "Stop: make stop-all"

stop-all:
	@echo "Stopping all services..."
	-@if [ -f /tmp/kube-watcher-dashboard.pid ]; then \
		kill $$(cat /tmp/kube-watcher-dashboard.pid) 2>/dev/null || true; \
		rm -f /tmp/kube-watcher-dashboard.pid; \
	fi
	$(COMPOSE_ENV) docker compose -f $(COMPOSE_FILE) down
	@echo "All services stopped."

# ── Kubernetes ─────────────────────────────────────────────────────────────────

k8s-logs-watcher:
	kubectl logs -n $(KW_NAMESPACE) -l app=watcher -c engine --tail=100 -f

# Badger Explorer runs as a sidecar in the watcher pod
k8s-logs-explorer:
	kubectl logs -n $(KW_NAMESPACE) -l app=watcher -c badger-explorer --tail=100 -f

k8s-pf-explorer:
	@echo "Badger Explorer → http://localhost:4101"
	kubectl port-forward -n $(KW_NAMESPACE) svc/watcher 4101:4101

k8s-restart-watcher:
	kubectl rollout restart deployment/watcher -n $(KW_NAMESPACE)
	kubectl rollout status deployment/watcher -n $(KW_NAMESPACE)
