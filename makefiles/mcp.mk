# ─── MCP Server ───────────────────────────────────────────────────────────────
# Covers: build, local run, Docker, cross-compilation, desktop app, compose

.PHONY: build-mcp build-deploy build-chatbridge build-desktop-app \
	build-all build-linux build-darwin build-windows \
	run run-mcp run-deploy run-chatbridge run-health run-list run-server \
	mcp-up install docker-build docker-run \
	dev desktop-dev desktop-build tui run-tui

# ── Build ──────────────────────────────────────────────────────────────────────

build-mcp:
	@echo "Building $(MCP_BINARY_NAME)..."
	$(GOBUILD) $(MCP_LDFLAGS) -o $(BIN_DIR)/$(MCP_BINARY_NAME) $(MCP_CMD)

build-deploy:
	@echo "Building $(DEPLOY_BINARY_NAME)..."
	$(GOBUILD) -o $(BIN_DIR)/$(DEPLOY_BINARY_NAME) $(DEPLOY_CMD)

build-chatbridge:
	@echo "Building $(CHAT_BRIDGE_BINARY_NAME)..."
	$(GOBUILD) -o $(BIN_DIR)/$(CHAT_BRIDGE_BINARY_NAME) $(CHAT_BRIDGE_CMD)

build-desktop-app:
	@echo "Building $(DESKTOP_APP_BINARY_NAME) Wails desktop app..."
	cd kube-watcher-app && wails build

# Cross-compilation (MCP binary only)
build-all: build-linux build-darwin build-windows

build-linux:
	@echo "Building for Linux..."
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(MCP_LDFLAGS) -o $(BIN_DIR)/$(MCP_BINARY_NAME)-linux-amd64 $(MCP_CMD)

build-darwin:
	@echo "Building for macOS..."
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(MCP_LDFLAGS) -o $(BIN_DIR)/$(MCP_BINARY_NAME)-darwin-amd64 $(MCP_CMD)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(MCP_LDFLAGS) -o $(BIN_DIR)/$(MCP_BINARY_NAME)-darwin-arm64 $(MCP_CMD)

build-windows:
	@echo "Building for Windows..."
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(MCP_LDFLAGS) -o $(BIN_DIR)/$(MCP_BINARY_NAME)-windows-amd64.exe $(MCP_CMD)

# ── Local Run ──────────────────────────────────────────────────────────────────

run: run-mcp

run-mcp:
	@echo "Running $(MCP_BINARY_NAME)..."
	$(GORUN) $(MCP_CMD) $(ARGS)

run-deploy:
	@echo "Running $(DEPLOY_BINARY_NAME)..."
	$(GORUN) $(DEPLOY_CMD) $(ARGS)

run-chatbridge:
	@echo "Running $(CHAT_BRIDGE_BINARY_NAME)..."
	$(GORUN) $(CHAT_BRIDGE_CMD) $(ARGS)

run-health:
	$(GORUN) $(MCP_CMD) --db-path $(MCP_DB_PATH) --health --health-allow-degraded

run-list:
	$(GORUN) $(MCP_CMD) --db-path $(MCP_DB_PATH) --list-tools

run-server:
	@echo "Running MCP server on port 8080..."
	$(GORUN) $(MCP_CMD) --db-path $(MCP_DB_PATH) --http-addr :8080

dev: fmt vet
	$(GORUN) $(MCP_CMD) --db-path $(MCP_DB_PATH)

install: build-mcp
	@echo "Installing $(MCP_BINARY_NAME) to $(GOPATH_BIN)..."
	@mkdir -p $(GOPATH_BIN)
	cp $(BIN_DIR)/$(MCP_BINARY_NAME) $(GOPATH_BIN)/

# ── Docker ─────────────────────────────────────────────────────────────────────

docker-build:
	@echo "Building Docker image..."
	$(COMPOSE_ENV) docker compose -f $(COMPOSE_FILE) build mcp

docker-run:
	@echo "Running Docker container (health check)..."
	docker run --rm -v ~/.kube:/home/nonroot/.kube watcher-mcp:latest --health

mcp-up:
	$(COMPOSE_ENV) docker compose -f $(COMPOSE_FILE) up --build mcp

# ── Desktop App ────────────────────────────────────────────────────────────────

desktop-dev:
	@echo "Starting Kube-Watcher Desktop App (Wails + Vue) in development mode..."
	@echo "Prerequisites:"
	@echo "  MCP server: make run-server (or set KW_TOOLS_ENDPOINT)"
	cd kube-watcher-app && wails dev

desktop-build: build-desktop-app

tui: desktop-dev
run-tui: desktop-dev

# ── Kubernetes ─────────────────────────────────────────────────────────────────

k8s-logs-mcp:
	kubectl logs -n $(KW_NAMESPACE) -l app=mcp --tail=100 -f

k8s-pf-mcp:
	@echo "MCP API → http://localhost:8080"
	kubectl port-forward -n $(KW_NAMESPACE) svc/mcp 8080:8080

k8s-restart-mcp:
	kubectl rollout restart deployment/mcp -n $(KW_NAMESPACE)
	kubectl rollout status deployment/mcp -n $(KW_NAMESPACE)
