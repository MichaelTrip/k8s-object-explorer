# Kubernetes Object Explorer - Copilot Instructions

This project is a comprehensive Go-based Kubernetes Resource Checker Application with the following features:

## Project Overview
- **Language**: Go 1.21+
- **Type**: Web-based Kubernetes resource management application
- **Architecture**: REST API backend with modern web frontend
- **Real-time Updates**: WebSocket support for live data
- **Export Capabilities**: JSON, CSV, PDF export functionality

## Key Components
- **Backend**: Go server with Kubernetes client-go integration
- **Frontend**: Vanilla JavaScript with responsive CSS
- **APIs**: RESTful endpoints for resource management
- **WebSocket**: Real-time updates and notifications
- **Authentication**: Kubernetes RBAC integration

## Development Status
✅ Project fully scaffolded and customized
✅ All dependencies installed and resolved
✅ Application compiles successfully 
✅ Documentation complete with comprehensive README
✅ Ready for development and deployment

## Quick Start
```bash
# Build the application
go build -o bin/k8s-object-explorer cmd/main.go

# Run the application
./bin/k8s-object-explorer

# Access the web interface
open http://localhost:8080
```

## Project Structure
```
├── cmd/                    # Application entry point
├── internal/
│   ├── api/               # REST API handlers  
│   ├── k8s/               # Kubernetes client integration
│   └── websocket/         # Real-time WebSocket communication
├── web/                   # Frontend assets (HTML, CSS, JS)
├── configs/               # Configuration files
└── README.md              # Comprehensive documentation
```