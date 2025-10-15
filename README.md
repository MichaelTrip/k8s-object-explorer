# Kubernetes Object Explorer 🔍

A lightweight, web-based tool for exploring and auditing Kubernetes cluster resources. Perfect for DevOps engineers, cluster administrators, and anyone who needs to quickly understand what's deployed in their namespaces.

## Features

- 🔍 **Resource Discovery**: Automatically detects all namespaced API resources across your cluster
- 📊 **Object Counting**: Real-time counting of objects per resource type
- 🎨 **Modern UI**: Clean, responsive interface with intuitive navigation
- 🚀 **Fast & Cached**: Multi-level caching (API discovery + namespace resources) for optimal performance
- 🔎 **Advanced Filtering**: Search by name, kind, API group, and object count ranges
- 📤 **Export Options**: Export resource inventory as CSV
- 🐳 **Container Ready**: Multi-architecture Docker image (AMD64, ARM64)
- 🔒 **Secure**: Runs as non-root user, minimal RBAC permissions
- 🏥 **Health Checks**: Built-in health check endpoints
- ⚡ **Production Ready**: Go-based server with efficient resource handling

## Quick Start

### Docker

```bash
# Run with your kubeconfig
docker run -p 8080:8080 \
  -v ~/.kube/config:/home/app/.kube/config:ro \
  ghcr.io/michaeltrip/k8s-object-explorer:latest

# Visit in browser
open http://localhost:8080
```

### Docker Compose

```bash
# Start with docker-compose
docker-compose up -d

# View logs
docker-compose logs -f
```

### Kubernetes

```bash
# Deploy to Kubernetes
kubectl apply -f k8s/

# Port forward for testing
kubectl port-forward service/k8s-object-explorer 8080:80

# Visit
open http://localhost:8080
```

## Usage Examples

### Browser Access
Visit `http://localhost:8080` in your web browser to see:
- 📋 **Namespace selector** - Choose which namespace to explore
- 📊 **Resource table** - All resources with object counts and metadata
- 🔍 **Search & filters** - Narrow down resources by name, kind, or API group
- 📤 **Export button** - Download CSV inventory of all resources
- 🐛 **Debug mode** - Real-time resource discovery progress (when DEBUG=true)

### API Endpoints

| Endpoint | Description | Response Format |
|----------|-------------|-----------------|
| `/api/namespaces` | List all namespaces | JSON |
| `/api/resources/{namespace}` | Get resources with counts | JSON |
| `/api/objects/{namespace}/{resource}` | List objects of specific resource | JSON |
| `/api/object/{namespace}/{resource}/{name}` | Get single object details | JSON |
| `/api/object-raw/{namespace}/{resource}/{name}` | Get raw K8s manifest | JSON |
| `/api/export/{namespace}` | Export as CSV | CSV |
| `/api/debug` | Debug status | JSON |
| `/api/debug-stream/{namespace}` | Real-time discovery (SSE) | Event Stream |

### API Examples

```bash
# List all namespaces
curl http://localhost:8080/api/namespaces

# Get resources in default namespace
curl http://localhost:8080/api/resources/default

# Filter resources
curl "http://localhost:8080/api/resources/default?populated=true&apiGroup=apps"

# List pods
curl http://localhost:8080/api/objects/default/pods

# Get specific deployment
curl http://localhost:8080/api/object/default/deployments.apps/my-app

# Export CSV
curl http://localhost:8080/api/export/default -o resources.csv

# Debug stream (real-time)
curl http://localhost:8080/api/debug-stream/default
```

## Configuration

The application can be configured using environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Port to listen on |
| `DEBUG` | `false` | Enable debug mode with verbose logging |
| `KUBECONFIG` | `~/.kube/config` | Path to kubeconfig file |

### Docker Environment Variables

```bash
# Enable debug mode
docker run -p 8080:8080 \
  -v ~/.kube/config:/home/app/.kube/config:ro \
  -e DEBUG=true \
  ghcr.io/michaeltrip/k8s-object-explorer:latest

# Custom port
docker run -p 3000:3000 \
  -v ~/.kube/config:/home/app/.kube/config:ro \
  -e PORT=3000 \
  ghcr.io/michaeltrip/k8s-object-explorer:latest
```

### Kubernetes Environment Variables

```yaml
env:
  - name: PORT
    value: "8080"
  - name: DEBUG
    value: "false"
```

## Deployment Examples

### Docker

#### Simple Run
```bash
docker run -d --name k8s-explorer -p 8080:8080 \
  -v ~/.kube/config:/home/app/.kube/config:ro \
  ghcr.io/michaeltrip/k8s-object-explorer:latest
```

#### With Debug Mode
```bash
docker run -d --name k8s-explorer -p 8080:8080 \
  -v ~/.kube/config:/home/app/.kube/config:ro \
  -e DEBUG=true \
  ghcr.io/michaeltrip/k8s-object-explorer:latest
```

### Docker Compose

See [`docker-compose.yml`](docker-compose.yml) for a complete example with:
- Environment variable configuration
- Volume mounts for kubeconfig
- Network configuration
- Health checks

```bash
docker-compose up -d
```

### Kubernetes

The `k8s/` directory contains complete Kubernetes manifests:

- **Deployment**: Multi-replica deployment with health checks and RBAC
- **Service**: ClusterIP service for internal access
- **Ingress**: Optional ingress for external access

#### Basic Deployment
```bash
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml
```

#### With Ingress
```bash
# Update host in k8s/ingress.yaml first
kubectl apply -f k8s/ingress.yaml
```

#### Port Forward for Testing
```bash
kubectl port-forward service/k8s-object-explorer 8080:80
```

See [k8s/README.md](k8s/README.md) for detailed Kubernetes deployment instructions.

## Development

### Local Development

```bash
# Clone the repository
git clone https://github.com/MichaelTrip/k8s-object-explorer.git
cd k8s-object-explorer

# Install dependencies
go mod download

# Run the application
go run cmd/main.go

# Or with debug mode
DEBUG=true go run cmd/main.go
```

### Building the Container

```bash
# Build locally
docker build -t k8s-object-explorer .

# Run your build
docker run -p 8080:8080 \
  -v ~/.kube/config:/home/app/.kube/config:ro \
  k8s-object-explorer

# Build for multiple architectures
docker buildx build --platform linux/amd64,linux/arm64 -t k8s-object-explorer .
```

### Testing

```bash
# Build and run
go build -o bin/k8s-object-explorer cmd/main.go
./bin/k8s-object-explorer

# Test API endpoints
curl http://localhost:8080/api/namespaces
curl http://localhost:8080/api/resources/default

# Test health endpoint
curl http://localhost:8080/api/debug
```

## Architecture

The application follows a simple, efficient architecture:

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Web Browser   │    │   Load Balancer  │    │  K8s Object     │
│                 │────▶│     /Ingress     │────▶│  Explorer       │
│                 │    │                  │    │                 │
└─────────────────┘    └──────────────────┘    │   Go Server     │
                                               │   + K8s Client  │
┌─────────────────┐                            │                 │
│   curl/API      │────────────────────────────▶│   Port 8080     │
│   Client        │                            │                 │
└─────────────────┘                            └─────────────────┘
                                                       │
                                                       ▼
                                               ┌─────────────────┐
                                               │  Kubernetes API │
                                               │     Server      │
                                               └─────────────────┘
```

### Key Components

- **Frontend**: Vanilla JavaScript (ES6+) with no frameworks
- **Backend**: Go with client-go library for Kubernetes interaction
- **Caching**: Two-tier cache (API discovery + namespace resources, 5min TTL)
- **API**: RESTful JSON API with CSV export capability

## Use Cases

- **Cluster Auditing**: Quick overview of what's deployed in each namespace
- **Resource Discovery**: Find all resource types available in your cluster
- **Debugging**: Understand resource distribution and identify unused resources
- **Documentation**: Export CSV inventory for compliance and documentation
- **Learning**: Explore Kubernetes API structure and available resources
- **Monitoring**: Track resource counts across namespaces

## RBAC Requirements

For proper operation in Kubernetes, the service account needs:

```yaml
rules:
- apiGroups: ["*"]
  resources: ["*"]
  verbs: ["get", "list"]
- apiGroups: [""]
  resources: ["namespaces"]
  verbs: ["get", "list"]
```

This provides **read-only** access to all resources, which is required for resource discovery and counting.

## Performance & Caching

The application uses intelligent caching to minimize API calls:

- **API Resource Discovery**: Cached for 5 minutes
- **Namespace Resource Counts**: Cached per namespace for 5 minutes
- **Request Deduplication**: Concurrent requests for the same namespace share results

This allows the application to handle:
- ✅ Clusters with 1000+ resources
- ✅ Multiple concurrent users
- ✅ Rapid namespace switching

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'feat: add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Commit Convention

This project uses [Conventional Commits](https://www.conventionalcommits.org/):

- `feat:` for new features
- `fix:` for bug fixes
- `docs:` for documentation changes
- `chore:` for maintenance tasks

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Support

- 🐛 **Issues**: [GitHub Issues](https://github.com/MichaelTrip/k8s-object-explorer/issues)
- 📖 **Documentation**: This README and [k8s/README.md](k8s/README.md)
- 💬 **Discussions**: [GitHub Discussions](https://github.com/MichaelTrip/k8s-object-explorer/discussions)

## Related Projects

- [myipcontainer](https://github.com/MichaelTrip/myipcontainer) - Simple IP address display container
- [lmsensors-container](https://github.com/MichaelTrip/lmsensors-container) - Hardware sensor monitoring for Kubernetes

---

Made with ❤️ by [MichaelTrip](https://github.com/MichaelTrip)