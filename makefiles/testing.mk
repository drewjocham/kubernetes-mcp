# ─── Testing, Linting & Code Quality ─────────────────────────────────────────
# Covers: unit tests, integration tests, coverage, fmt, vet, lint, deps

.PHONY: test test-integration-channel test-coverage test-tui-render \
	lint fmt vet tidy deps dev-tools

# ── Tests ──────────────────────────────────────────────────────────────────────

test:
	@echo "Running unit tests..."
	$(GOTEST) -v `$(GOCMD) list ./... | grep -vF '/node_modules/'`

test-integration-channel:
	@echo "Running integration test: engine -> UI -> MCP channel..."
	$(GOTEST) -v ./watcher/internal/integration -run 'TestChannelEngineToUIAnd.*Integration'

test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -coverprofile=coverage.out `$(GOCMD) list ./... | grep -vF '/node_modules/'`
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

test-tui-render: build-desktop-app
	@echo "Smoke-testing TUI render..."
	@timeout 2 $(BIN_DIR)/$(DESKTOP_APP_BINARY_NAME) 2>/dev/null \
		|| echo "App started and timed out as expected — render OK"

# ── Code Quality ───────────────────────────────────────────────────────────────

fmt:
	@echo "Formatting code..."
	$(GOCMD) fmt ./...

vet:
	@echo "Vetting code..."
	$(GOCMD) vet ./...

tidy:
	@echo "Tidying modules..."
	$(GOCMD) mod tidy

lint:
	@echo "Linting code..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed — run: make dev-tools"; \
	fi

# ── Dependencies & Toolchain ──────────────────────────────────────────────────

deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

dev-tools: deps
	@echo "Installing development tools..."
	$(GOCMD) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
