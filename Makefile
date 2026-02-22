# kube-watcher Makefile

BIN_DIR=bin
BINARY_NAME?=$(MCP_BINARY_NAME)
MCP_BINARY_NAME=kube-watcher
WATCHER_BINARY_NAME=watcher-engine
MCP_CMD=./mcp/cmd/server
WATCHER_CMD=./watcher/cmd/engine
VERSION?=1.0.0
GIT_COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "dev")
BUILD_DATE?=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
MCP_LDFLAGS=-ldflags "-X main.version=${VERSION} -X main.gitCommit=${GIT_COMMIT} -X main.buildDate=${BUILD_DATE}"

GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod
GORUN=$(GOCMD) run

.PHONY: all build clean test deps run help lint fmt vet

# Default target
all: fmt vet test build

# Build the applications
build: build-mcp build-watcher

build-mcp:
	@echo "Building $(MCP_BINARY_NAME)..."
	$(GOBUILD) $(MCP_LDFLAGS) -o $(BIN_DIR)/$(MCP_BINARY_NAME) $(MCP_CMD)

build-watcher:
	@echo "Building $(WATCHER_BINARY_NAME)..."
	$(GOBUILD) -o $(BIN_DIR)/$(WATCHER_BINARY_NAME) $(WATCHER_CMD)

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

run-health:
	$(GORUN) $(MCP_CMD) --health

run-list:
	$(GORUN) $(MCP_CMD) --list-tools

run-server:
	$(GORUN) $(MCP_CMD) --server

# Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf bin/

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# Lint code
lint:
	@echo "Linting code..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Install it with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

# Format code
fmt:
	@echo "Formatting code..."
	$(GOCMD) fmt ./...

# Vet code
vet:
	@echo "Vetting code..."
	$(GOCMD) vet ./...

# Install the binary
install: build-mcp
	@echo "Installing $(MCP_BINARY_NAME)..."
	cp $(BIN_DIR)/$(MCP_BINARY_NAME) $(GOPATH)/bin/

# Build for multiple platforms
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

# Docker targets
docker-build:
	@echo "Building Docker image..."
	docker build -t $(BINARY_NAME):$(VERSION) .

docker-run:
	@echo "Running Docker container..."
	docker run --rm -it -v ~/.kube:/root/.kube $(BINARY_NAME):$(VERSION)

# Development helpers
dev: fmt vet
	$(GORUN) $(MCP_CMD) --server

dev-tools: deps
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Show help
help:
	@echo "Available targets:"
	@echo "  build         - Build the application binary"
	@echo "  run           - Run the MCP server (alias for run-mcp)"
	@echo "  run-mcp       - Run the MCP server (use ARGS= for arguments)"
	@echo "  run-watcher   - Run the watcher event engine (use ARGS= for arguments)"
	@echo "  run-health    - Run health check"
	@echo "  run-list      - List available tools"
	@echo "  run-server    - Run in server mode"
	@echo "  clean         - Clean build artifacts"
	@echo "  test          - Run tests"
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
	@echo "  help          - Show this help message"
	@echo ""
	@echo "Examples:"
	@echo "  make run ARGS='--health'"
	@echo "  make run ARGS='--exec analyze_cluster --args \"{\\\"include_pods\\\":true}\"'"
	@echo "  make build VERSION=1.1.0"
