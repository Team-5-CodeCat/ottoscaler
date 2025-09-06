# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Ottoscaler is a Kubernetes-native auto-scaler written in Go that dynamically manages Otto agent pods based on Redis Streams events. It runs as a main controller pod within a Kubernetes cluster and creates/manages worker pods on demand.

### System Architecture

```
External Redis → Kubernetes Main Pod (Ottoscaler) → Otto Agent Pods
```

- **Main Pod**: Event-driven coordinator running continuously in Kubernetes
- **Event Processing**: Consumes scale_up/scale_down events from Redis Streams
- **Worker Management**: Dynamically creates, monitors, and cleans up Otto Agent pods
- **Concurrent Processing**: Manages multiple worker pods simultaneously

## Development Environment

This project uses a containerized development environment for cross-platform consistency.

### Essential Commands

**Environment Management:**
```bash
make dev-start    # Start development environment (Redis + dev container)
make dev-shell    # Enter development container
make dev-stop     # Stop development environment
make dev-clean    # Complete cleanup (containers + cache)
```

**Development Workflow (inside container):**
```bash
go run ./cmd/app     # Run the application
make test-event      # Send test scaling event to Redis
make test            # Run tests
make fmt             # Format code
make lint            # Run linter
```

**Production Commands:**
```bash
make build           # Build production image
make deploy          # Deploy to Kubernetes
make logs            # View deployed pod logs
```

## Code Architecture

### Core Components

- **Main Pod** (`cmd/app/main.go`): Event-driven coordinator that runs continuously
- **Redis Client** (`internal/redis/client.go`): Manages consumer groups and polls events every 2 seconds
- **Kubernetes Client** (`internal/k8s/client.go`): Handles pod CRUD operations and cluster interaction
- **Worker Manager** (`internal/worker/manager.go`): Manages Otto Agent pod lifecycle

### Go Project Structure (Standard Layout)

```
ottoscaler/
├── cmd/app/                 # Main application entry point
├── internal/                # Private packages (not importable)
│   ├── redis/               # Redis Streams client
│   ├── k8s/                 # Kubernetes API client  
│   ├── worker/              # Worker pod lifecycle management
│   └── app/                 # Application-specific logic
├── k8s/                     # Kubernetes manifests
├── docs/                    # Documentation
└── scripts/                 # Build and utility scripts
```

### Execution Model

- **Main Thread**: Waits for termination signals (graceful shutdown)
- **Event Processing Goroutine**: Redis event consumption and worker coordination
- **Redis Listener Goroutine**: 2-second polling of Redis Streams (blocking with timeout)
- **Worker Management Goroutines**: Independent creation→monitoring→cleanup for each worker pod


## Key Technologies

### Dependencies
- `k8s.io/client-go` - Kubernetes Go client for cluster interaction
- `github.com/redis/go-redis/v9` - Redis client for consuming Redis Streams messages

### Environment Variables
```bash
REDIS_HOST=host.docker.internal    # Redis server address
REDIS_PORT=6379                    # Redis server port
REDIS_STREAM=otto:scale:events     # Redis stream name
REDIS_CONSUMER_GROUP=ottoscaler    # Consumer group name
OTTO_AGENT_IMAGE=busybox:latest    # Worker pod image
```

### Redis Event Format
```bash
XADD otto:scale:events * type scale_up pod_count 3 task_id task-123
```

## Development Workflow

### Local Development with Kind (Recommended)

1. **Set up Kind Cluster** (one-time setup):
   ```bash
   # Install Kind and kubectl
   curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.24.0/kind-linux-amd64
   chmod +x ./kind && sudo mv ./kind /usr/local/bin/kind
   
   curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
   sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl
   
   # Create Kind cluster
   kind create cluster --name ottoscaler-dev
   
   # Apply RBAC configuration
   kubectl apply -f k8s/rbac.yaml
   ```

2. **Start Redis** (if not running):
   ```bash
   docker run -d --name redis-hanjinwoo -p 6379:6379 redis:7-alpine
   ```

3. **Development Cycle**:
   ```bash
   # Terminal 1: Run Ottoscaler (connects to Kind cluster)
   make run-app
   # or: go run ./cmd/app
   
   # Terminal 2: Send test events
   make test-event
   # or: go run ./cmd/test-event
   
   # Terminal 3: Monitor Worker Pods
   kubectl get pods -w
   ```

### Container Development (Alternative)

1. **Start Environment**: `make dev-start` (once)
2. **Enter Container**: `make dev-shell`
3. **Edit Code**: Use local IDE (changes sync automatically via volume mount)
4. **Run Application**: `go run ./cmd/app` (inside container)
5. **Test**: `make test-event` to send scaling events
6. **Monitor**: `kubectl get pods -w` to watch worker pod lifecycle

The development environment includes:
- Go 1.24 + complete toolchain
- golangci-lint for code quality
- kubectl for Kubernetes interaction
- Redis CLI for debugging
- Starship prompt (🐹 indicator in container)

## Testing

### Run Tests
```bash
make test           # Run all tests
go test ./...       # Alternative direct command
go test -race ./... # Check for race conditions
```

### Test Events
```bash
make test-event     # Send test scale_up event to Redis (Go-based)
go run ./cmd/test-event  # Direct command
make redis-cli      # Access Redis CLI for manual testing
```

### Monitor Worker Pods
```bash
kubectl get pods -l managed-by=ottoscaler  # List worker pods
kubectl get pods -w                        # Watch pod lifecycle in real-time
kubectl logs -l app=ottoscaler -f          # Follow main pod logs (when deployed)
```

### Kind Cluster Management
```bash
# Cluster info
kubectl cluster-info --context kind-ottoscaler-dev
kubectl get nodes

# Cleanup (when needed)
kind delete cluster --name ottoscaler-dev

# Recreate cluster
kind create cluster --name ottoscaler-dev
kubectl apply -f k8s/rbac.yaml
```

## Code Standards

### Go Conventions
- Use standard Go project layout
- `internal/` packages are not importable externally
- `pkg/` packages are public APIs
- Follow Go naming conventions (PascalCase for exported, camelCase for unexported)
- All long-running operations must use `context.Context`
- Proper error handling and logging with `log.Printf`


## Kubernetes Deployment

### RBAC and ServiceAccount
- ServiceAccount: `ottoscaler`
- ClusterRole: Pod management permissions
- Deployment: Single replica main pod
- Labels: Worker pods labeled with `managed-by=ottoscaler`

### Resource Management
- Main pod runs continuously in cluster
- Worker pods created on-demand from scaling events
- Automatic cleanup after worker completion
- Monitors worker status every 2 seconds

## Current Limitations

- Scale-down functionality not yet implemented
- No resource limits on worker pods
- Basic error handling without retry mechanisms
- Limited observability and metrics

## Important Notes

- The main pod runs **inside** the Kubernetes cluster, not externally
- Local execution (`go run ./cmd/app`) is for development only
- Production deployment uses `make deploy` to create Kubernetes resources
- ServiceAccount permissions allow pod CRUD operations within the cluster