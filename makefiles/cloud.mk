# ─── Cloud Service ──────────────────────────────────────────────────────────────
# Covers: build, local run, Docker, Kubernetes operations for cloud service

.PHONY: build-cloud run-cloud cloud-up docker-build-cloud docker-run-cloud \
	k8s-logs-cloud k8s-pf-cloud k8s-restart-cloud

# ── Build ──────────────────────────────────────────────────────────────────────

build-cloud:
	@echo "Building $(CLOUD_BINARY_NAME)..."
	$(GOBUILD) -o $(BIN_DIR)/$(CLOUD_BINARY_NAME) $(CLOUD_CMD)

# ── Local Run ──────────────────────────────────────────────────────────────────

run-cloud:
	@echo "Running $(CLOUD_BINARY_NAME)..."
	$(GORUN) $(CLOUD_CMD) $(ARGS)

run-cloud-server:
	@echo "Running cloud server on port 9090..."
	$(GORUN) $(CLOUD_CMD) --http-addr :9090

# ── Docker ─────────────────────────────────────────────────────────────────────

docker-build-cloud:
	@echo "Building cloud Docker image..."
	docker build -f $(ROOT_DIR)/Dockerfile.cloud -t kube-watcher-cloud:latest $(ROOT_DIR)

docker-run-cloud:
	@echo "Running cloud container..."
	docker run --rm -p 9090:9090 -e CLOUD_DB_PATH=/data/cloud.db kube-watcher-cloud:latest --http-addr :9090

cloud-up:
	$(COMPOSE_ENV) docker compose -f $(COMPOSE_FILE) up --build cloud

# ── Kubernetes ─────────────────────────────────────────────────────────────────

k8s-logs-cloud:
	kubectl logs -n $(KW_NAMESPACE) -l app=cloud --tail=100 -f

k8s-pf-cloud:
	@echo "Cloud API → http://localhost:9090"
	@echo "Cloud WebSocket → ws://localhost:9090/ws"
	kubectl port-forward -n $(KW_NAMESPACE) svc/cloud 9090:9090

k8s-restart-cloud:
	kubectl rollout restart deployment/cloud -n $(KW_NAMESPACE)
	kubectl rollout status deployment/cloud -n $(KW_NAMESPACE)