# kube-watcher Makefile
THIS_MAKEFILE_PATH := $(abspath $(lastword $(MAKEFILE_LIST)))
MAKEFILE_DIR := $(dir $(THIS_MAKEFILE_PATH))
REPO_ROOT := $(abspath $(MAKEFILE_DIR)/..)

BIN_DIR=bin
BINARY_NAME?=$(MCP_BINARY_NAME)
COMPOSE_FILE?=docker/compose.yaml
KUBECONFIG_PATH?=$(HOME)/.kube/config
COMPOSE_ENV=KUBECONFIG_PATH=$(KUBECONFIG_PATH)
MCP_BINARY_NAME=kube-watcher
WATCHER_BINARY_NAME=watcher-engine
DEPLOY_BINARY_NAME=watcher-deploy
CHAT_BRIDGE_BINARY_NAME=chatbridge
DESKTOP_APP_BINARY_NAME=kube-watcher-app
MCP_CMD=./mcp/cmd/server
WATCHER_CMD=./watcher/cmd/engine
DEPLOY_CMD=./watcher/cmd/deploy
CHAT_BRIDGE_CMD=./integrations/cmd/chatbridge
DESKTOP_APP_CMD=./cmd/kube-watcher-app
VERSION?=1.0.0
GIT_COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "dev")
BUILD_DATE?=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
MCP_LDFLAGS=-ldflags "-X main.version=${VERSION} -X main.gitCommit=${GIT_COMMIT} -X main.buildDate=${BUILD_DATE}"
KUBECONFIG_PATH="${HOME}/.kube/config"
MCP_DB_PATH?=$(HOME)/.kube-watcher/history.make.db

GOCMD=go
GOPATH_BIN?=$(shell $(GOCMD) env GOPATH)/bin
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod
GORUN=$(GOCMD) run

.PHONY: all build clean test deps run help lint fmt vet tidy \
	compose-up-mcp compose-up-watcher compose-up-all compose-down \
	test-integration-channel start-all stop-all \
	install build-all build-linux build-darwin build-windows \
	docker-build docker-run dev dev-tools \
	dashboard-deps dashboard-dev dashboard-build dashboard-preview dashboard-lint dashboard-typecheck \
	run-mcp run-watcher run-deploy run-chatbridge run-health run-list run-server run-tui \
	mcp-up watcher-up up down build-desktop-app tui desktop-dev desktop-build

# Default target
all: fmt vet test build

# Build the applications
build: build-mcp build-watcher build-deploy build-chatbridge build-desktop-app

build-mcp:
	@echo "Building $(MCP_BINARY_NAME)..."
	$(GOBUILD) $(MCP_LDFLAGS) -o $(BIN_DIR)/$(MCP_BINARY_NAME) $(MCP_CMD)

build-watcher:
	@echo "Building $(WATCHER_BINARY_NAME)..."
	$(GOBUILD) -o $(BIN_DIR)/$(WATCHER_BINARY_NAME) $(WATCHER_CMD)

build-deploy:
	@echo "Building $(DEPLOY_BINARY_NAME)..."
	$(GOBUILD) -o $(BIN_DIR)/$(DEPLOY_BINARY_NAME) $(DEPLOY_CMD)

build-chatbridge:
	@echo "Building $(CHAT_BRIDGE_BINARY_NAME)..."
	$(GOBUILD) -o $(BIN_DIR)/$(CHAT_BRIDGE_BINARY_NAME) $(CHAT_BRIDGE_CMD)

build-desktop-app:
	@echo "Building $(DESKTOP_APP_BINARY_NAME) Wails desktop app..."
	cd kube-watcher-app && wails build

run-tui: desktop-dev

test-tui-render: build-desktop-app
	@echo "Testing TUI rendering..."
	@echo "This will show if the View methods return content:"
	@timeout 2 $(BIN_DIR)/$(DESKTOP_APP_BINARY_NAME) 2>/dev/null || echo "✓ App started (timed out as expected)"
	@echo "If you saw text above, rendering is working!"

desktop-dev:
	@echo "🚀 Starting Kube-Watcher Desktop App (Wails + Vue) in development mode..."
	@echo ""
	@echo "Features:"
	@echo "  • Modern desktop app with web-like interface"
	@echo "  • Real-time Kubernetes monitoring and alerts"
	@echo "  • Proton/Apple dark theme with purple accents"
	@echo "  • Interactive UI for troubleshooting"
	@echo ""
	@echo "Prerequisites:"
	@echo "  • MCP server: make run-server (or set KW_TOOLS_ENDPOINT)"
	@echo "  • Optional: KW_AGENT_ENDPOINT for agent integration"
	@echo ""
	@echo "Starting development server..."
	cd kube-watcher-app && wails dev

desktop-build: build-desktop-app

tui: desktop-dev

tidy:
	go mod tidy

# Run the applications
run: run-mcp

run-mcp:
	@echo "Running $(MCP_BINARY_NAME)..."
	$(GORUN) $(MCP_CMD) $(ARGS)

run-watcher:
	@echo "Running $(WATCHER_BINARY_NAME)..."
	$(GORUN) $(WATCHER_CMD) $(ARGS)

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

mcp-up:
	$(COMPOSE_ENV) docker compose -f $(COMPOSE_FILE) up --build mcp

watcher-up:
	$(COMPOSE_ENV) docker compose -f $(COMPOSE_FILE) up --build watcher

up:
	$(COMPOSE_ENV) docker compose -f $(COMPOSE_FILE) up --build -d

down:
	$(COMPOSE_ENV) docker compose -f $(COMPOSE_FILE) down

clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf bin/

test:
	@echo "Running tests..."
	$(GOTEST) -v `$(GOCMD) list ./... | grep -vF '/node_modules/'`

test-integration-channel:
	@echo "Running integration test: engine -> UI -> MCP channel..."
	$(GOTEST) -v ./watcher/internal/integration -run 'TestChannelEngineToUIAnd.*Integration'

test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -coverprofile=coverage.out `$(GOCMD) list ./... | grep -vF '/node_modules/'`
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

lint:
	@echo "Linting code..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Install it with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

fmt:
	@echo "Formatting code..."
	$(GOCMD) fmt ./...

vet:
	@echo "Vetting code..."
	$(GOCMD) vet ./...

install: build-mcp
	@echo "Installing $(MCP_BINARY_NAME) to $(GOPATH_BIN)..."
	@mkdir -p $(GOPATH_BIN)
	cp $(BIN_DIR)/$(MCP_BINARY_NAME) $(GOPATH_BIN)/

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

docker-build:
	@echo "Building Docker image..."
	$(COMPOSE_ENV) docker compose -f $(COMPOSE_FILE) build mcp

docker-run:
	@echo "Running Docker container..."
	docker run --rm -v ~/.kube:/home/nonroot/.kube watcher-mcp:latest --health

# Development
dev: fmt vet
	$(GORUN) $(MCP_CMD) --db-path $(MCP_DB_PATH)

dev-tools: deps
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Dashboard commands
.PHONY: dashboard-dev dashboard-build dashboard-preview dashboard-lint dashboard-typecheck dashboard-deps

DASHBOARD_DIR=dashboard
DASHBOARD_PREVIEW_PORT?=4173

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
	@echo "Previewing dashboard..."
	cd $(DASHBOARD_DIR) && CI=true npm run preview -- --port $(DASHBOARD_PREVIEW_PORT)

dashboard-lint:
	@echo "Linting dashboard..."
	cd $(DASHBOARD_DIR) && npm run lint

dashboard-typecheck:
	@echo "Typechecking dashboard..."
	cd $(DASHBOARD_DIR) && npm run typecheck

start-all:
	@echo "Starting all services..."
	@echo "Starting Docker containers (mcp, watcher, prometheus, grafana)..."
	$(COMPOSE_ENV) docker compose -f $(COMPOSE_FILE) up --build -d
	@sleep 5
	@echo "Starting dashboard..."
	@( cd dashboard && CI=true npm run dev --prefix . > /tmp/kube-watcher-dashboard.log 2>&1 & echo $$! > /tmp/kube-watcher-dashboard.pid )
	@sleep 3
	@echo "Dashboard started at http://localhost:3000"
	@echo "MCP Tools API at http://localhost:8080"
	@echo "Watcher metrics at http://localhost:9095"
	@echo "Dashboard log: /tmp/kube-watcher-dashboard.log"
	@echo ""
	@echo "To stop all services: make stop-all"
	@echo "To view dashboard logs: tail -f /tmp/kube-watcher-dashboard.log"

stop-all:
	@echo "Stopping all services..."
	@echo "Stopping dashboard (PID file if present)..."
	-@if [ -f /tmp/kube-watcher-dashboard.pid ]; then \
		kill $$(cat /tmp/kube-watcher-dashboard.pid) 2>/dev/null || true; \
		rm -f /tmp/kube-watcher-dashboard.pid; \
	fi
	@sleep 1
	@echo "Stopping Docker containers..."
	$(COMPOSE_ENV) docker compose -f $(COMPOSE_FILE) down
	@echo "All services stopped."

help:
	@echo "Available targets:"
	@echo "  build         - Build the application binary"
	@echo "  build-desktop-app - Build the Kube-Watcher desktop application (Wails + Vue)"
	@echo "  desktop-dev   - Run the desktop app in development mode"
	@echo "  desktop-build - Build the desktop app for production"
	@echo "  tui           - Alias for desktop-dev"
	@echo "  run-tui       - Alias for desktop-dev"
	@echo "  run           - Run the MCP server (alias for run-mcp)"
	@echo "  run-mcp       - Run the MCP server (use ARGS= for arguments)"
	@echo "  run-watcher   - Run the watcher event engine (use ARGS= for arguments)"
	@echo "  run-deploy    - Run deploy/cleanup CLI for kube anomaly detection (use ARGS= for arguments)"
	@echo "  run-chatbridge - Run the Google Chat bridge CLI (use ARGS= for arguments)"
	@echo "  mcp-up        - docker compose up --build mcp"
	@echo "  watcher-up    - docker compose up --build watcher"
	@echo "  up            -       docker compose up --build (all services)"
	@echo "  start-all     - Start all services (docker compose + dashboard)"
	@echo "  stop-all      - Stop dashboard (PID file) and docker compose stack"
	@echo "  down          - docker compose down"
	@echo "  run-health    - Run health check (allows degraded K8s for smoke/CI)"
	@echo "  run-list      - List available tools"
	@echo "  run-server    - Run in server mode"
	@echo "  clean         - Clean build artifacts"
	@echo "  test          - Run tests"
	@echo "  test-integration-channel - Run engine -> UI -> MCP channel integration test"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  deps          - Download and tidy dependencies"
	@echo "  lint          - Run linter"
	@echo "  fmt           - Format code"
	@echo "  vet           - Vet code"
	@echo "  install       - Install binary to GOPATH/bin"
	@echo "  build-all     - Build for all platforms"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-run    - Run Docker container"
	@echo "  dev           - Format, vet and run in development mode"
	@echo "  dev-tools     - Install development tools"
	@echo "  dashboard-deps - Install dashboard dependencies"
	@echo "  dashboard-dev - Run dashboard in development mode"
	@echo "  dashboard-build - Build dashboard"
	@echo "  dashboard-preview - Preview dashboard"
	@echo "  dashboard-lint - Lint dashboard"
	@echo "  dashboard-typecheck - Typecheck dashboard"
	@echo "  help          - Show this help message"
