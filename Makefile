# kube-watcher — root Makefile
# Can be invoked from any subdirectory; will forward to the repo root.
#
#   makefiles/mcp.mk        MCP server: build, run, docker, k8s ops
#   makefiles/watcher.mk    Watcher engine: build, run, compose stack
#   makefiles/monitoring.mk Prometheus, Grafana, Anomstack, dashboard, k8s observability
#   makefiles/helm.mk       Helm chart lifecycle
#   makefiles/testing.mk    Tests, coverage, lint, fmt, vet

# ROOT_DIR must be captured before any include changes MAKEFILE_LIST.
ROOT_DIR := $(patsubst %/,%,$(dir $(abspath $(lastword $(MAKEFILE_LIST)))))

ifeq ($(realpath $(CURDIR)),$(ROOT_DIR))
# ─────────────────────────────────────────────────────────────────────────────
# Running from the repo root — all real content below.
# ─────────────────────────────────────────────────────────────────────────────

# ─── Shared Variables ─────────────────────────────────────────────────────────

BIN_DIR              ?= $(ROOT_DIR)/bin
COMPOSE_FILE         ?= $(ROOT_DIR)/docker/compose.yaml
KUBECONFIG_PATH      ?= $(HOME)/.kube/config
COMPOSE_ENV           = KUBECONFIG_PATH=$(KUBECONFIG_PATH)

# Binary names
MCP_BINARY_NAME       = kube-watcher
WATCHER_BINARY_NAME   = watcher-engine
DEPLOY_BINARY_NAME    = watcher-deploy
CHAT_BRIDGE_BINARY_NAME = chatbridge
DESKTOP_APP_BINARY_NAME = kube-watcher-app

# Source entry points (relative to ROOT_DIR, used with go run/build)
MCP_CMD               = ./mcp/cmd/server
WATCHER_CMD           = ./watcher/cmd/engine
DEPLOY_CMD            = ./watcher/cmd/deploy
CHAT_BRIDGE_CMD       = ./integrations/cmd/chatbridge
DESKTOP_APP_CMD       = ./cmd/kube-watcher-app

# Build metadata
VERSION              ?= 1.0.0
GIT_COMMIT           ?= $(shell git -C "$(ROOT_DIR)" rev-parse --short HEAD 2>/dev/null || echo "dev")
BUILD_DATE           ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')
MCP_LDFLAGS           = -ldflags "-X main.version=$(VERSION) -X main.gitCommit=$(GIT_COMMIT) -X main.buildDate=$(BUILD_DATE)"
MCP_DB_PATH          ?= $(HOME)/.kube-watcher/history.make.db

# Go toolchain
GOCMD                 = go
GOPATH_BIN           ?= $(shell $(GOCMD) env GOPATH)/bin
GOBUILD               = cd "$(ROOT_DIR)" && $(GOCMD) build
GOCLEAN               = cd "$(ROOT_DIR)" && $(GOCMD) clean
GOTEST                = cd "$(ROOT_DIR)" && $(GOCMD) test
GOMOD                 = cd "$(ROOT_DIR)" && $(GOCMD) mod
GORUN                 = cd "$(ROOT_DIR)" && $(GOCMD) run

# Helm / Kubernetes
HELM_CHART_DIR       ?= $(ROOT_DIR)/Helm/kube-watcher
HELM_RELEASE         ?= kube-watcher
HELM_VALUES          ?= $(HELM_CHART_DIR)/values.yaml
KW_NAMESPACE         ?= kube-watcher
ANOMSTACK_NS         ?= kw-anomaly

# Dashboard
DASHBOARD_DIR        ?= $(ROOT_DIR)/dashboard
DASHBOARD_PREVIEW_PORT ?= 4173

# ─── Includes ─────────────────────────────────────────────────────────────────

include $(ROOT_DIR)/makefiles/mcp.mk
include $(ROOT_DIR)/makefiles/watcher.mk
include $(ROOT_DIR)/makefiles/monitoring.mk
include $(ROOT_DIR)/makefiles/helm.mk
include $(ROOT_DIR)/makefiles/testing.mk

# ─── Top-Level Targets ────────────────────────────────────────────────────────

.PHONY: all build clean help

all: fmt vet test build

build: build-mcp build-watcher build-deploy build-chatbridge build-desktop-app

clean:
	@echo "Cleaning build artifacts..."
	$(GOCLEAN)
	rm -rf $(BIN_DIR)/

# ─── Help ─────────────────────────────────────────────────────────────────────

help:
	@echo ""
	@echo "kube-watcher — available targets"
	@echo "  (see makefiles/*.mk for implementation)"
	@echo ""
	@echo "General:"
	@echo "  all               fmt + vet + test + build"
	@echo "  build             build all binaries"
	@echo "  clean             remove bin/ artifacts"
	@echo ""
	@echo "MCP Server  (makefiles/mcp.mk):"
	@echo "  build-mcp         build MCP binary"
	@echo "  build-all         cross-compile for linux/darwin/windows"
	@echo "  run-server        run MCP HTTP server on :8080"
	@echo "  run-health        health check (degraded K8s allowed)"
	@echo "  run-list          list registered tools"
	@echo "  mcp-up            docker compose up mcp"
	@echo "  docker-build      build MCP Docker image"
	@echo "  docker-run        run MCP container (health check)"
	@echo "  install           install binary to GOPATH/bin"
	@echo "  desktop-dev       Wails desktop app in dev mode"
	@echo "  desktop-build     build Wails desktop app"
	@echo "  k8s-logs-mcp      tail MCP pod logs"
	@echo "  k8s-pf-mcp        port-forward MCP → localhost:8080"
	@echo "  k8s-restart-mcp   rolling restart MCP deployment"
	@echo ""
	@echo "Watcher Engine  (makefiles/watcher.mk):"
	@echo "  build-watcher     build watcher binary"
	@echo "  run-watcher       run watcher engine locally"
	@echo "  watcher-up        docker compose up watcher"
	@echo "  up                docker compose up (full stack)"
	@echo "  down              docker compose down"
	@echo "  start-all         compose up + dashboard dev server"
	@echo "  stop-all          stop dashboard PID + compose down"
	@echo "  k8s-logs-watcher  tail watcher engine container logs"
	@echo "  k8s-logs-explorer tail Badger Explorer sidecar logs"
	@echo "  k8s-pf-explorer   port-forward Badger Explorer → localhost:4101"
	@echo "  k8s-restart-watcher rolling restart watcher deployment"
	@echo ""
	@echo "Monitoring  (makefiles/monitoring.mk):"
	@echo "  dashboard-deps    npm install for Vue dashboard"
	@echo "  dashboard-dev     run Vue dashboard dev server"
	@echo "  dashboard-build   production build of Vue dashboard"
	@echo "  dashboard-lint    lint Vue dashboard"
	@echo "  dashboard-typecheck typecheck Vue dashboard"
	@echo "  k8s-status        kubectl get all (both namespaces)"
	@echo "  k8s-events        warning events (both namespaces)"
	@echo "  k8s-top           node + pod resource usage"
	@echo "  k8s-logs-prometheus tail Prometheus logs"
	@echo "  k8s-logs-grafana  tail Grafana logs"
	@echo "  k8s-logs-anomstack tail Anomstack webserver logs"
	@echo "  k8s-pf-grafana    port-forward Grafana → localhost:3000"
	@echo "  k8s-pf-prometheus port-forward Prometheus → localhost:9090"
	@echo "  k8s-pf-anomstack  port-forward Anomstack UI → localhost:3001"
	@echo "  k8s-pf-all        all port-forwards in background"
	@echo "  k8s-restart-grafana rolling restart Grafana"
	@echo "  k8s-restart-anomstack rolling restart all Anomstack deployments"
	@echo ""
	@echo "Helm  (makefiles/helm.mk):"
	@echo "  helm-lint         lint the chart"
	@echo "  helm-template     render templates to stdout"
	@echo "  helm-dry-run      server-side dry-run with debug"
	@echo "  helm-install      install (or upgrade) the full stack"
	@echo "  helm-upgrade      atomic upgrade — rolls back on failure"
	@echo "  helm-uninstall    uninstall the Helm release"
	@echo "  helm-status       release status + revision history"
	@echo "  helm-diff         diff pending changes (helm-diff plugin)"
	@echo "  helm-rollback     roll back one revision"
	@echo ""
	@echo "Testing  (makefiles/testing.mk):"
	@echo "  test              run unit tests"
	@echo "  test-integration-channel  engine→UI→MCP integration test"
	@echo "  test-coverage     tests with HTML coverage report"
	@echo "  lint              golangci-lint"
	@echo "  fmt               go fmt"
	@echo "  vet               go vet"
	@echo "  tidy              go mod tidy"
	@echo "  deps              download + tidy modules"
	@echo "  dev-tools         install golangci-lint"
	@echo ""

else
# ─────────────────────────────────────────────────────────────────────────────
# Running from a subdirectory — transparently forward to the repo root.
# ─────────────────────────────────────────────────────────────────────────────
.PHONY: $(MAKECMDGOALS)
$(or $(MAKECMDGOALS),all):
	@$(MAKE) --no-print-directory -C "$(ROOT_DIR)" $(MAKECMDGOALS)
endif
