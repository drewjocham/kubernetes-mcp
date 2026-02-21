# kube-watcher

A Kubernetes monitoring and analysis MCP (Model Context Protocol) server designed to provide cluster insights and monitoring capabilities.

## Features

- **Node Monitoring**: Detailed analysis of cluster nodes, health status, and resource allocation
- **Pod Analysis**: Comprehensive pod resource monitoring, restart tracking, and issue identification  
- **Cluster Analysis**: Full cluster health assessment with recommendations and alerting
- **MCP Protocol**: Standard Model Context Protocol interface for integration with AI systems
- **Go Architecture**: Clean, idiomatic Go design using interfaces and factory patterns

## Architecture

The project follows Go best practices with a clean separation of concerns:

```
kube-watcher/
├── cmd/           # Main application entry point
├── server/        # MCP server implementation
├── tools/         # Tool implementations (Command pattern)
├── kubernetes/    # Kubernetes client abstraction
└── main.go        # Convenience wrapper
```
## Installation

1. Clone the repository:
```bash
git clone <repository-url>
cd kube-watcher
```

2. Install dependencies:
```bash
go mod tidy
```

3. Ensure you have kubectl configured and access to a Kubernetes cluster.

## Usage

### Command Line Interface

#### List Available Tools
```bash
go run main.go --list-tools
```

#### Execute Specific Tools
```bash
# Node status analysis
go run main.go --exec get_node_status --args '{"include_metrics":true}'

# Pod resource monitoring
go run main.go --exec get_pod_resources --args '{"problematic_only":true}'

# Full cluster analysis  
go run main.go --exec analyze_cluster --args '{"include_pods":true,"include_events":true}'
```

#### Health Check
```bash
go run main.go --health
```

#### Interactive Server Mode
```bash
go run main.go --server
```

### Available Tools

#### `get_node_status`
Analyzes cluster nodes for health, taints, conditions, and resource allocation.

**Parameters:**
- `include_metrics` (boolean): Include resource metrics [default: true]
- `taints_only` (boolean): Only return tainted nodes [default: false]  
- `node_name` (string): Specific node to analyze [optional]

#### `get_pod_resources`
Monitors pod resource usage, restart counts, and identifies problematic pods.

**Parameters:**
- `namespace` (string): Filter by namespace [optional]
- `status_filter` (string): Filter by pod phase [optional]
- `high_restart_threshold` (number): Restart count threshold [default: 5]
- `include_containers` (boolean): Include container details [default: true]
- `problematic_only` (boolean): Only problematic pods [default: false]

#### `analyze_cluster`
Comprehensive cluster analysis with health scoring and recommendations.

**Parameters:**  
- `include_pods` (boolean): Include pod analysis [default: true]
- `include_events` (boolean): Include event analysis [default: true]
- `event_hours_back` (number): Hours of events to analyze [default: 24]
- `include_services` (boolean): Include service analysis [default: false]
- `detailed_analysis` (boolean): Generate recommendations [default: true]

## MCP Integration

The server implements the Model Context Protocol for integration with AI systems:

```go
server := server.NewMCPServer()

// Handle MCP requests
response, err := server.HandleMCPRequest(ctx, "tools/list", nil)
response, err := server.HandleMCPRequest(ctx, "tools/call", map[string]interface{}{
    "name": "analyze_cluster", 
    "arguments": map[string]interface{}{"include_pods": true},
})
```

## Development

### Adding New Tools

1. Create a new tool in `tools/` directory:
```go
type MyTool struct {
    BaseTool  
}

func NewMyTool(k8sManager kubernetes.ClientInterface) *MyTool {
    return &MyTool{BaseTool: NewBaseTool(k8sManager)}
}

func (t *MyTool) Name() string { return "my_tool" }
func (t *MyTool) Description() string { return "My custom tool" }
func (t *MyTool) Parameters() []ToolParameter { return []ToolParameter{} }
func (t *MyTool) Execute(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
    // Implementation
}
```

2. Register it in `server/server.go`:
```go
func (s *MCPServer) registerTools() {
    // ... existing tools
    myTool := tools.NewMyTool(s.k8sClient)
    s.tools[myTool.Name()] = myTool
}
```

### Testing

The architecture supports easy testing through interface injection:

```go
func TestMyTool(t *testing.T) {
    mockClient := &MockK8sClient{}
    tool := tools.NewMyTool(mockClient)
    
    result, err := tool.Execute(context.Background(), map[string]interface{}{})
    // Assertions...
}
```

## Configuration

The application automatically detects Kubernetes configuration:

1. **In-cluster**: Uses service account when running inside Kubernetes
2. **Local**: Uses `~/.kube/config` for local development
3. **Custom**: Specify custom kubeconfig path via `kubernetes.NewClientFromConfig()`

## Dependencies

- `k8s.io/client-go`: Official Kubernetes Go client library
- `k8s.io/api`: Kubernetes API types
- `k8s.io/apimachinery`: Kubernetes API machinery
- `go.uber.org/zap`: High-performance structured logging
- Go 1.25.1+

## Health and Monitoring

The server provides comprehensive health checks:

- **Server Status**: Application health
- **Kubernetes Connectivity**: Cluster access verification  
- **Tool Registration**: Available tools count
- **Cluster Information**: Basic cluster metrics

## Security

- Uses standard Kubernetes RBAC for authorization
- Respects kubeconfig security settings  
- No secrets stored or logged in plaintext
- Read-only operations by default

## Troubleshooting

### Common Issues

1. **"Failed to create kubernetes client"**: Check kubeconfig and cluster access
2. **"Tool execution failed"**: Verify cluster connectivity and RBAC permissions
3. **"Health check failed"**: Ensure kubectl works from same environment

### Logging Configuration

The application uses Zap for structured logging. Configure logging through environment variables:

```bash
# Log levels: debug, info, warn, error, fatal
export LOG_LEVEL=debug

# Log formats: console (development), json (production)
export LOG_FORMAT=json

# Enable debug mode (uses development logger)
export DEBUG=true

# Alternative environment variables
export KUBE_WATCHER_LOG_LEVEL=info
export KUBE_WATCHER_LOG_FORMAT=console
export KUBE_WATCHER_DEBUG=true
```

**Examples:**
```bash
# Development mode with colored console output
DEBUG=true go run main.go --health

# Production mode with JSON logs
LOG_FORMAT=json LOG_LEVEL=info go run main.go --health

# Debug level logging
LOG_LEVEL=debug go run main.go --exec analyze_cluster
```
