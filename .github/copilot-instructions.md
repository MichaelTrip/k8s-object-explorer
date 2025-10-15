# Kubernetes Object Explorer - Copilot Instructions

This is a Go-based web application for exploring Kubernetes cluster resources with real-time updates and caching.

## Architecture & Data Flow

**Core Pattern**: Single `cmd/main.go` server that hosts both REST APIs and static web files
- All API routes must be registered BEFORE the catch-all static file handler
- Kubernetes client initialized once at startup with graceful degradation if cluster unavailable
- Multi-level caching: API resource discovery (5min TTL) + per-namespace resource counts

**Request Flow**: Frontend JS → REST API → K8s client-go → Discovery/Dynamic clients → Cache → Response
- Resource discovery uses `ServerPreferredNamespacedResources()` with fallback for partial failures
- Dynamic client used for actual object counting/fetching per namespace  
- Synchronous request/response pattern with client-side caching

## Key Development Patterns

**Kubernetes Client (`internal/k8s/client.go`)**:
```go
// Always use context with timeout for K8s operations
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

// ResourceInfo struct is core data model - tracks name, kind, API group, count
type ResourceInfo struct {
    Name        string `json:"name"`
    FullName    string `json:"fullName"`    // name.apiGroup for uniqueness  
    DisplayName string `json:"displayName"` // name (apiGroup) for display
    // ...
}
```

**Error Handling**: Graceful degradation throughout - app starts even if K8s unavailable
- Discovery failures return core resource fallback (pods, services, deployments, etc.)
- Missing kubeconfig falls back to in-cluster config
- Cache misses trigger fresh discovery with exponential backoff
- Permission errors are suppressed for known problematic resources (bindings, tokenrequests, etc.)

**Request Deduplication**: Implemented directly in `cmd/main.go`
- All API handlers are methods on the `Server` struct  
- Direct integration with k8s client for resource operations
- Simplified architecture with single-file API implementation

**Frontend Architecture (`web/js/app.js`)**:
- Class-based ES6+ with no frameworks  
- Request cooldown (2s) prevents API spam during rapid interactions
- Filter debouncing with 300ms timeout on user input
- Embedded JavaScript in HTML file for simplified deployment

## Development Workflows

**Build & Run**:
```bash
# Standard build (creates bin/k8s-object-explorer)
go build -o bin/k8s-object-explorer cmd/main.go

# Development with debug output
DEBUG=true ./bin/k8s-object-explorer

# Auto-rebuild on changes (requires entr)
find . -name "*.go" | entr -r go run cmd/main.go
```

**Debug Features**:
- Server-sent events endpoint `/api/debug-stream/{namespace}` provides real-time discovery progress
- Debug mode shows resource counting details, cache hit/miss info, and processing times
- Cache status exposed in API responses when debug enabled

**Testing**: Minimal test coverage currently - only basic import test in `cmd/main_test.go`

**Configuration**:
- Port via `PORT` env var (default 8080)  
- Debug mode via `DEBUG=true` env var enables verbose logging
- Kubeconfig auto-discovery: `~/.kube/config` → in-cluster config
- Web assets served from `./web/` with fallback path resolution

## Integration Points

**Static File Serving**: Dynamic web directory resolution
```go
// Tries: ./web → ../web (if running from bin/) → ./web fallback
webDir := "web"
if _, err := os.Stat(webDir); os.IsNotExist(err) {
    execPath, _ := os.Executable()
    execDir := filepath.Dir(execPath)
    webDir = filepath.Join(execDir, "..", "web")
}
```

**API Endpoints**:
- `/api/namespaces` - List all namespaces
- `/api/resources/{namespace}` - Get resource counts for namespace (cached)
- `/api/objects/{namespace}/{resource}` - List objects of specific resource type
- `/api/object/{namespace}/{resource}/{name}` - Get single object details
- `/api/object-raw/{namespace}/{resource}/{name}` - Get raw K8s manifest
- `/api/export/{namespace}` - CSV export of resource inventory
- `/api/debug-stream/{namespace}` - Server-sent events for real-time discovery (debug mode)

**Static File Serving**: Serves web assets from `./web/` directory
- Dynamic path resolution handles different execution contexts
- All static files (HTML, CSS, JS) served directly by Go http.FileServer

**Resource Discovery**: Handles partial API discovery failures gracefully
- Falls back to core resources (pods, services, deployments, configmaps, secrets) if discovery fails
- Skips problematic resources by name to avoid permission errors
- Uses GVR (GroupVersionResource) for dynamic client operations

## Project-Specific Conventions

**Module Structure**: `k8s-object-explorer` module with `internal/` for private packages
**Logging**: Standard `log` package with `[DEBUG]` prefixes when debug enabled
**JSON Responses**: Consistent error response format with `ErrorResponse` struct
**Resource Naming**: Uses Kubernetes `FullName` pattern (resource.group) for uniqueness across API groups
**Caching Strategy**: Two-tier cache (API resources + per-namespace counts) with 5min TTL
**Dependencies**: Minimal external deps - Gorilla mux, client-go, no frameworks