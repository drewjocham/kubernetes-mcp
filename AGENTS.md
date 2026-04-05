# AGENTS.md - kube-watcher Development Guide

This file provides instructions for agentic coding agents working on the kube-watcher repository.

## Project Overview

kube-watcher is a Kubernetes monitoring platform with two independent Go applications:
1. **MCP Server** (`mcp/`) - Exposes cluster state via Model Context Protocol (stdio + HTTP)
2. **Watcher Event Engine** (`watcher/`) - Real-time rule engine with CEL/YAML condition evaluation

Supporting components: Nuxt 3 dashboard (`dashboard/`), Google Chat bridge (`integrations/chatbridge/`), unified CLI (`cmd/`).

## Build/Lint/Test Commands

### Go (root module)
```bash
make build           # Build all binaries
make build-mcp       # Build MCP server only  
make build-watcher   # Build watcher engine only
make test            # Run all tests (excludes node_modules)
make test-integration-channel  # Engine → UI → MCP integration test
make test-coverage   # Tests with HTML coverage report
make lint            # golangci-lint
make fmt             # go fmt
make vet             # go vet
make dev             # fmt + vet + run MCP in dev mode
```

### Docker
```bash
make up              # docker compose up --build (all services)
make down            # docker compose down
make start-all       # Docker + dashboard
```

### Dashboard (Nuxt 3)
```bash
make dashboard-dev       # Dev server on :3000
make dashboard-build     # Production build
make dashboard-lint      # ESLint
make dashboard-typecheck # TypeScript checks
```

### Running Single Tests
```bash
go test ./mcp/tools/... -run TestNodeStatus
go test ./watcher/internal/rules/... -run TestEngine
go test ./pkg/kube/... -v
```

### Terminal Dashboard (kube-watcher-app/)
```bash
cd kube-watcher-app
go build -o kube-watcher-app ./cmd/kube-watcher-app/
./kube-watcher-app
make desktop-dev      # Development mode (Wails + Vue)
```

## Code Style Guidelines

### Imports
Group imports in order:
1. Standard library
2. Third-party packages
3. Local packages

```go
import (
    "context"
    "fmt"
    "strings"

    "github.com/spf13/cobra"
    corev1 "k8s.io/api/core/v1"

    "kube-watcher/pkg/kube"
)
```

### Naming Conventions
- **Exported types/functions**: `PascalCase`
- **Unexported**: `camelCase`
- **Interfaces**: `Interface` suffix (e.g., `ClientInterface`)
- **Constants**: `UPPER_SNAKE_CASE`
- **Test files**: `*_test.go` with `Test` prefix for functions
- **Test helpers**: `helper_test.go`

### Error Handling
- Use `fmt.Errorf` with `%w` for wrapping errors
- Define error variables at package level
- Return `(result, error)` tuples
- Log errors with structured logging (slog)

```go
var ErrCreateKubeConfig = errors.New("failed to create kubernetes config")

func NewClient(logger *slog.Logger) (*Client, error) {
    config, err := rest.InClusterConfig()
    if err != nil {
        return nil, fmt.Errorf("%w: %w", ErrCreateKubeConfig, err)
    }
}
```

### Types and Structs
- Define types in `pkg/kube/types.go` for shared types
- Use JSON tags for serialization
- Embed interfaces, not concrete types

```go
type NodeInfo struct {
    Name        string            `json:"name"`
    Status      string            `json:"status"`
    Conditions  []NodeCondition   `json:"conditions"`
    Age         time.Duration     `json:"age"`
}

type NodeStatusTool struct {
    BaseTool  // Embedded interface
}
```

### Function Signatures
- `context.Context` as first parameter
- Return interface types, not concrete implementations
- Use variadic parameters for optional configuration

```go
func (t *NodeStatusTool) Execute(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
    // ...
}
```

## Architectural Patterns

### 1. Handler Pattern
Separate request/event signaling from execution logic. Handlers have single responsibility and are decoupled from transport layers (HTTP/gRPC).

### 2. Adapter Pattern
All third-party SDKs, APIs, and database drivers must be wrapped in an interface. Do not let external library types leak into core domain or business logic.

### 3. Strategy Pattern
Replace complex conditional branching with a Strategy interface when logic depends on configuration, state, or external providers.

### 4. DRY (Don't Repeat Yourself)
Abstract any logic repeated more than twice.

## Testing Guidelines

### Table-Driven Tests
All unit tests must use a struct-based table with `name`, `input`, `expected`, `wantErr` fields.

```go
func TestNodeStatusTool_Execute(t *testing.T) {
    tests := []struct {
        name      string
        nodes     []kube.NodeInfo
        nodesErr  error
        args      map[string]any
        wantErr   bool
        checkFunc func(t *testing.T, res map[string]any)
    }{
        {
            name: "AllNodesHealthy",
            nodes: []kube.NodeInfo{
                {Name: "node-1", Status: "Ready"},
            },
            args: map[string]any{},
            checkFunc: func(t *testing.T, res map[string]any) {
                // assertions
            },
        },
    }
}
```

### Mocking
- Use interface-based mocking
- Create mock implementations in `*_test.go` files
- Test both success and error paths

### Integration Tests
- Use `Test*Integration` naming convention
- Test across component boundaries
- Clean up resources after tests

## Project Structure

### Key Packages
| Package | Role |
|---|---|
| `pkg/kube/` | Kubernetes client wrapper (`ClientInterface`), NodeInfo/PodInfo types |
| `pkg/kube/watch/` | Watch Manager: informers, scanner, alert deduplication |
| `mcp/server/` | MCPServer: tool registry, alert loop, MCP protocol |
| `mcp/tools/` | Tool implementations (NodeStatus, PodResources, etc.) |
| `mcp/monitoring/history/` | BadgerDB-backed incident store with TTL/GC |
| `mcp/monitoring/recommendation/` | Alert → remediation steps with frequency analysis |
| `watcher/internal/pipeline/` | 5-stage event processing pipeline |
| `watcher/internal/rules/` | Condition + CEL expression evaluation |
| `watcher/internal/actions/` | Throttled action dispatcher (log, webhook) |

### File Organization
- `cmd/` - Application entry points
- `internal/` - Private application code
- `pkg/` - Public libraries
- `api/` - HTTP API definitions
- `mcp/` - Model Context Protocol server
- `watcher/` - Event engine

## Workflow for Agents

1. **Before making changes**: Run `make fmt vet lint` to ensure code quality
2. **After making changes**: Run `make test` to verify tests pass
3. **For complex changes**: Run `make test-coverage` to check coverage
4. **Before committing**: Run `make build` to ensure compilation succeeds
5. **For UI changes**: Run `make dashboard-lint dashboard-typecheck`

## Environment Variables

### MCP Server
```
KUBECONFIG_PATH        # Path to kubeconfig (default: ~/.kube/config)
MCP_DB_PATH            # BadgerDB path for history (default: ~/.kube-watcher/history.make.db)
```

### Terminal Dashboard
```
KW_TOOLS_ENDPOINT     # MCP server (default: http://localhost:8080/v1)
KW_TOOLS_API_TOKEN    # MCP bearer token
KW_AGENT_ENDPOINT     # Agent chat endpoint (default: http://localhost:3000/api/agent)
KW_PROMETHEUS_URL     # Prometheus (default: http://localhost:9090)
```

### K8sGPT Integration
```
K8SGPT_BASE_URL       # K8sGPT MCP server URL (default: http://localhost:8089)
K8SGPT_API_KEY        # API key for AI provider (OpenAI, etc.)
K8SGPT_MODEL          # AI model to use (default: gpt-4o)
K8SGPT_BACKEND        # AI backend (openai, azureopenai, ollama, etc.)
```

## Commit Guidelines

- Use conventional commit messages
- Reference issue numbers when applicable
- Keep commits focused on single logical changes
- Include both Go and TypeScript changes in same commit when related

## Go Module

- Go 1.25.1, single `go.mod` at root
- Key deps: `k8s.io/client-go`, `github.com/mark3labs/mcp-go`, `github.com/dgraph-io/badger/v4`
- Run `go mod tidy` after adding dependencies