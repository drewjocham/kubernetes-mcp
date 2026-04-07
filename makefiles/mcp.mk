# ─── MCP Server ───────────────────────────────────────────────────────────────
# Covers: build, local run, Docker, cross-compilation, desktop app, compose

.PHONY: build-mcp build-cli build-deploy build-chatbridge build-desktop-app \
	build-all build-linux build-darwin build-windows \
	run run-mcp run-deploy run-chatbridge run-health run-list run-server \
	mcp-up install install-system install-local docker-build docker-run \
	dev desktop-dev desktop-build tui run-tui

# ── Build ──────────────────────────────────────────────────────────────────────

build-mcp:
	@echo "Building $(MCP_BINARY_NAME)..."
	$(GOBUILD) $(MCP_LDFLAGS) -o $(BIN_DIR)/$(MCP_BINARY_NAME) $(MCP_CMD)

build-cli:
	@echo "Building $(CLI_BINARY_NAME)..."
	$(GOBUILD) -o $(BIN_DIR)/$(CLI_BINARY_NAME) $(CLI_CMD)
	@echo "Creating kw symlink to $(CLI_BINARY_NAME)..."
	@ln -sf $(CLI_BINARY_NAME) $(BIN_DIR)/kw

build-deploy:
	@echo "Building $(DEPLOY_BINARY_NAME)..."
	$(GOBUILD) -o $(BIN_DIR)/$(DEPLOY_BINARY_NAME) $(DEPLOY_CMD)

build-chatbridge:
	@echo "Building $(CHAT_BRIDGE_BINARY_NAME)..."
	$(GOBUILD) -o $(BIN_DIR)/$(CHAT_BRIDGE_BINARY_NAME) $(CHAT_BRIDGE_CMD)

build-desktop-app:
	@if [ -d "kube-watcher-app" ]; then \
		echo "Building $(DESKTOP_APP_BINARY_NAME) Wails desktop app..."; \
		cd kube-watcher-app && wails build; \
	else \
		echo "Skipping desktop app: kube-watcher-app directory not found"; \
	fi

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



# ── Docker ─────────────────────────────────────────────────────────────────────

docker-build:
	@echo "Building Docker image..."
	docker build -f $(ROOT_DIR)/Dockerfile.mcp -t watcher-mcp:latest $(ROOT_DIR)

docker-run:
	@echo "Running Docker container (health check)..."
	docker run --rm -v ~/.kube:/home/nonroot/.kube watcher-mcp:latest --health

mcp-up:
	$(COMPOSE_ENV) docker compose -f $(COMPOSE_FILE) up --build mcp

# ── Desktop App ────────────────────────────────────────────────────────────────

desktop-dev:
	@if [ -d "kube-watcher-app" ]; then \
		echo "Starting Kube-Watcher Desktop App (Wails + Vue) in development mode..."; \
		echo "Prerequisites:"; \
		echo "  MCP server: make run-server (or set KW_TOOLS_ENDPOINT)"; \
		cd kube-watcher-app && KW_CLI_BINARY_PATH="$(ROOT_DIR)/bin/kw-cli" KW_BINARY_PATH="$(ROOT_DIR)/bin/kw" wails dev; \
	else \
		echo "Desktop app not available: kube-watcher-app directory not found"; \
		echo "To use the desktop app, clone the kube-watcher-app repository"; \
	fi

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

# ── Installation ──────────────────────────────────────────────────────────────

install: build-mcp build-cli
	@echo "Installing $(MCP_BINARY_NAME) and $(CLI_BINARY_NAME) to $(GOPATH_BIN)..."
	@mkdir -p $(GOPATH_BIN)
	cp $(BIN_DIR)/$(MCP_BINARY_NAME) $(GOPATH_BIN)/
	cp $(BIN_DIR)/$(CLI_BINARY_NAME) $(GOPATH_BIN)/
	@echo "Creating kw symlink in $(GOPATH_BIN) to $(CLI_BINARY_NAME)..."
	@ln -sf $(CLI_BINARY_NAME) $(GOPATH_BIN)/kw
	@echo "$(MCP_BINARY_NAME) and $(CLI_BINARY_NAME) installed to $(GOPATH_BIN)"
	@echo "kw symlink created. Ensure $(GOPATH_BIN) is in your PATH"

install-system: build-cli
	@echo "Installing $(CLI_BINARY_NAME) to /usr/local/bin..."
	@cp -f $(BIN_DIR)/$(CLI_BINARY_NAME) /usr/local/bin/$(CLI_BINARY_NAME) 2>/dev/null || \
		echo "Failed to install to /usr/local/bin. Try: sudo make install"
	@echo "Creating kw symlink to $(CLI_BINARY_NAME)..."
	@ln -sf $(CLI_BINARY_NAME) /usr/local/bin/kw 2>/dev/null || \
		echo "Failed to create symlink. Try: sudo make install"
	@echo "$(CLI_BINARY_NAME) installed successfully"

install-local: build-cli
	@echo "Installing $(CLI_BINARY_NAME) to ~/.local/bin..."
	@mkdir -p ~/.local/bin
	@cp -f $(BIN_DIR)/$(CLI_BINARY_NAME) ~/.local/bin/$(CLI_BINARY_NAME)
	@echo "Creating kw symlink to $(CLI_BINARY_NAME)..."
	@ln -sf $(CLI_BINARY_NAME) ~/.local/bin/kw
	@echo "$(CLI_BINARY_NAME) installed to ~/.local/bin. Ensure ~/.local/bin is in your PATH"
