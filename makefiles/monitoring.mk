
.PHONY: dashboard-deps dashboard-dev dashboard-build dashboard-preview \
	dashboard-lint dashboard-typecheck \
	k8s-status k8s-events k8s-top \
	k8s-logs-prometheus k8s-logs-anomstack \
	k8s-pf-prometheus k8s-pf-anomstack k8s-pf-all \
	k8s-restart-anomstack

# ── Vue Dashboard ──────────────────────────────────────────────────────────────

dashboard-deps:
	@echo "Installing dashboard dependencies..."
	cd $(DASHBOARD_DIR) && npm install

dashboard-dev:
	@echo "Running dashboard in development mode..."
	cd $(DASHBOARD_DIR) && CI=true npm run dev

dashboard-build:
	@echo "Building dashboard..."
	cd $(DASHBOARD_DIR) && npm run build

dashboard-preview:
	@echo "Previewing dashboard on port $(DASHBOARD_PREVIEW_PORT)..."
	cd $(DASHBOARD_DIR) && CI=true npm run preview -- --port $(DASHBOARD_PREVIEW_PORT)

dashboard-lint:
	@echo "Linting dashboard..."
	cd $(DASHBOARD_DIR) && npm run lint

dashboard-typecheck:
	@echo "Typechecking dashboard..."
	cd $(DASHBOARD_DIR) && npm run typecheck

# ── Kubernetes — Cluster Observability ────────────────────────────────────────

k8s-status:
	@echo "=== Deployments & Services: $(KW_NAMESPACE) ==="
	kubectl get all -n $(KW_NAMESPACE)
	@echo ""
	@echo "=== Deployments & Services: $(ANOMSTACK_NS) ==="
	kubectl get all -n $(ANOMSTACK_NS)
	@echo ""
	@echo "=== PersistentVolumeClaims ==="
	kubectl get pvc -n $(KW_NAMESPACE)
	@kubectl get pvc -n $(ANOMSTACK_NS) 2>/dev/null || true

k8s-events:
	@echo "=== Warning Events: $(KW_NAMESPACE) ==="
	kubectl get events -n $(KW_NAMESPACE) --sort-by='.lastTimestamp' \
		--field-selector type=Warning 2>/dev/null \
		|| kubectl get events -n $(KW_NAMESPACE) --sort-by='.lastTimestamp'
	@echo ""
	@echo "=== Warning Events: $(ANOMSTACK_NS) ==="
	@kubectl get events -n $(ANOMSTACK_NS) --sort-by='.lastTimestamp' 2>/dev/null || true

k8s-top:
	@echo "=== Node resource usage ==="
	kubectl top nodes
	@echo ""
	@echo "=== Pod resource usage: $(KW_NAMESPACE) ==="
	kubectl top pods -n $(KW_NAMESPACE)
	@echo ""
	@echo "=== Pod resource usage: $(ANOMSTACK_NS) ==="
	@kubectl top pods -n $(ANOMSTACK_NS) 2>/dev/null || true

# ── Kubernetes — Logs ─────────────────────────────────────────────────────────

k8s-logs-prometheus:
	kubectl logs -n $(KW_NAMESPACE) -l app=prometheus --tail=100 -f



k8s-logs-anomstack:
	kubectl logs -n $(ANOMSTACK_NS) -l app=anomstack-webserver --tail=100 -f

# ── Kubernetes — Port-Forwards ────────────────────────────────────────────────



k8s-pf-prometheus:
	@echo "Prometheus → http://localhost:9090"
	kubectl port-forward -n $(KW_NAMESPACE) svc/prometheus 9090:9090

k8s-pf-anomstack:
	@echo "Anomstack Dagster UI → http://localhost:3001"
	kubectl port-forward -n $(ANOMSTACK_NS) svc/anomstack-webserver 3001:3000

k8s-pf-all:
	@echo "Starting all port-forwards in background..."
	kubectl port-forward -n $(KW_NAMESPACE) svc/mcp 8080:8080 &
	kubectl port-forward -n $(KW_NAMESPACE) svc/prometheus 9090:9090 &
	kubectl port-forward -n $(KW_NAMESPACE) svc/watcher 4101:4101 &
	kubectl port-forward -n $(ANOMSTACK_NS) svc/anomstack-webserver 3001:3000 &
	@echo ""
	@echo "  MCP API        → http://localhost:8080"
	@echo "  Prometheus     → http://localhost:9090"
	@echo "  Badger Explorer→ http://localhost:4101"
	@echo "  Anomstack UI   → http://localhost:3001"
	@echo ""
	@echo "Kill all: pkill -f 'kubectl port-forward'"

# ── Kubernetes — Rolling Restarts ─────────────────────────────────────────────

k8s-restart-anomstack:
	kubectl rollout restart \
		deployment/anomstack-daemon \
		deployment/anomstack-webserver \
		deployment/anomstack-dashboard \
		-n $(ANOMSTACK_NS)
