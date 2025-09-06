# Ottoscaler Architecture

## Overview

Ottoscaler is a Kubernetes-native autoscaler that dynamically manages Otto Agent worker pods based on gRPC commands from otto-handler. It runs as a main controller pod within a Kubernetes cluster and provides pod orchestration services via gRPC API.

## System Architecture

```
┌─────────────────────┐         ┌─────────────────────────────────────────┐
│                     │         │         Kubernetes Cluster              │
│   Otto-handler      │  gRPC   │                                         │
│   (NestJS)          ├────────▶│  ┌──────────────┐   ┌──────────────┐  │
│                     │ :9090   │  │  Ottoscaler  │──▶│ Worker Pods  │  │
│                     │◀────────┤  │  (Main Pod)  │   │ (Otto Agent) │  │
│                     │  Logs   │  └──────────────┘   └──────────────┘  │
└─────────────────────┘         └─────────────────────────────────────────┘
```

## Core Components

### 1. Main Pod (Ottoscaler)
- **Location**: `cmd/app/main.go`
- **Role**: gRPC server and event-driven coordinator
- **Lifecycle**: Long-running daemon process
- **Responsibilities**:
  - Listen for gRPC scaling commands from otto-handler
  - Initialize and manage Kubernetes client
  - Orchestrate worker pod creation and cleanup
  - Stream worker logs back to otto-handler
  - Handle graceful shutdown signals

### 2. gRPC Server (`internal/grpc/`)
- **Purpose**: API endpoint for otto-handler integration
- **Key Components**:
  - `server.go`: Main gRPC server implementation
  - `scaling.go`: Scaling logic and worker configuration
  - `log_streaming.go`: Log forwarding implementation
  - `otto_handler_client.go`: Client for sending logs to otto-handler

### 3. Kubernetes Client (`internal/k8s/client.go`)
- **Purpose**: Kubernetes cluster interaction
- **Features**:
  - In-cluster and kubeconfig-based authentication
  - Pod CRUD operations
  - Pod status monitoring
  - Namespace-scoped operations

### 4. Worker Manager (`internal/worker/manager.go`)
- **Purpose**: Otto Agent pod lifecycle management
- **Features**:
  - Concurrent worker pod creation
  - Pod completion monitoring (2-second interval polling)
  - Automatic cleanup after completion
  - Error handling and recovery

### 5. Configuration (`internal/config/config.go`)
- **Purpose**: Centralized configuration management
- **Features**:
  - Environment-based configuration
  - Default values for all settings
  - Validation of required parameters

## Execution Flow

### 1. Startup Sequence
```go
main() {
    // 1. Load configuration
    config := loadConfig()
    
    // 2. Initialize Kubernetes client
    k8sClient := k8s.NewClient(config.Kubernetes)
    
    // 3. Create worker manager
    workerManager := worker.NewManager(k8sClient, config.Worker)
    
    // 4. Initialize gRPC server
    grpcServer := grpc.NewServer(config, k8sClient, workerManager)
    
    // 5. Start gRPC server
    go grpcServer.Start(config.GRPC.Port)
    
    // 6. Wait for shutdown signal
    <-sigChan // Block until SIGTERM/SIGINT
}
```

### 2. Scaling Request Handling
```go
ScaleUp(ctx, request) {
    // 1. Validate request
    if err := validateScaleRequest(request); err != nil {
        return nil, err
    }
    
    // 2. Create worker configurations
    configs := createWorkerConfigs(request)
    
    // 3. Launch workers concurrently
    results := workerManager.RunMultipleWorkers(ctx, configs)
    
    // 4. Return response with created pod names
    return &ScaleResponse{
        Status: SUCCESS,
        WorkerPodNames: results.PodNames,
    }, nil
}
```

### 3. Worker Pod Management
```go
RunMultipleWorkers(configs) {
    var wg sync.WaitGroup
    results := make(chan WorkerResult, len(configs))
    
    for _, config := range configs {
        wg.Add(1)
        go func(cfg WorkerConfig) {
            defer wg.Done()
            
            // Create pod
            pod := createPod(cfg)
            
            // Monitor until completion
            monitorPod(pod)
            
            // Cleanup
            deletePod(pod)
            
            results <- WorkerResult{...}
        }(config)
    }
    
    wg.Wait()
    return collectResults(results)
}
```

## Concurrency Model

### Goroutine Structure
```
Main Thread
├── Signal Handler (blocking wait for SIGTERM/SIGINT)
│
├── gRPC Server Goroutine
│   ├── ScaleUp Handler
│   ├── ScaleDown Handler
│   └── GetWorkerStatus Handler
│
├── Log Streaming Goroutine
│   └── Forward worker logs to otto-handler
│
└── Worker Management Goroutines (per worker)
    ├── Worker 1: Create → Monitor → Cleanup
    ├── Worker 2: Create → Monitor → Cleanup
    └── Worker N: Create → Monitor → Cleanup
```

## gRPC Services

### OttoscalerService
Primary service for scaling operations:

```protobuf
service OttoscalerService {
    rpc ScaleUp(ScaleRequest) returns (ScaleResponse);
    rpc ScaleDown(ScaleRequest) returns (ScaleResponse);
    rpc GetWorkerStatus(WorkerStatusRequest) returns (WorkerStatusResponse);
}
```

### OttoHandlerLogService
Service for log forwarding:

```protobuf
service OttoHandlerLogService {
    rpc ForwardWorkerLogs(stream WorkerLogEntry) returns (stream LogForwardResponse);
    rpc NotifyWorkerStatus(WorkerStatusNotification) returns (WorkerStatusAck);
}
```

## Configuration

### Environment Variables
```bash
# gRPC Server
GRPC_PORT=9090
OTTO_HANDLER_HOST=otto-handler:8080

# Kubernetes
NAMESPACE=default
OTTO_AGENT_IMAGE=busybox:latest

# Worker Resources
WORKER_CPU_LIMIT=500m
WORKER_MEMORY_LIMIT=128Mi

# Logging
LOG_LEVEL=info
```

## Deployment

### Kubernetes Resources
- **ServiceAccount**: `ottoscaler`
- **ClusterRole**: Pod management permissions
- **ClusterRoleBinding**: Binds role to ServiceAccount
- **Deployment**: Single replica main pod
- **Service**: gRPC endpoint exposure (optional)

### Development Commands
```bash
# Infrastructure
make setup-user USER=한진우    # Setup Kind cluster
make build                     # Build Docker image
make deploy                    # Deploy to Kubernetes

# Testing
kubectl port-forward deployment/ottoscaler 9090:9090
./test-scaling -action scale-up -workers 3

# Monitoring
make logs                      # View ottoscaler logs
kubectl get pods -l managed-by=ottoscaler  # View worker pods
```

## Security Considerations

### Kubernetes Security
- **RBAC**: Least privilege pod management permissions
- **ServiceAccount**: Dedicated account for authentication
- **Namespace Isolation**: Multi-tenant support

### gRPC Security
- **Development**: Currently using insecure connections
- **Production**: TLS/mTLS planned
- **Authentication**: Token-based auth to be added

## Performance Characteristics

### Scalability
- Single main pod (no HA currently)
- Can manage hundreds of worker pods
- 2-second polling interval for status checks
- Concurrent worker creation

### Resource Usage
- **Main Pod**: ~50MB memory, minimal CPU
- **Worker Pods**: Configurable limits
- **Network**: Minimal gRPC traffic

## Current Limitations

1. **High Availability**: Single point of failure (main pod)
2. **Scale Down**: Not fully implemented
3. **Log Collection**: Basic implementation, needs enhancement
4. **Metrics**: Limited observability
5. **Error Recovery**: Basic error handling, no retry mechanisms

## Future Enhancements

### Short Term
- Complete log streaming implementation
- Implement scale-down functionality
- Add retry mechanisms for failed workers
- Enhance error handling and recovery

### Long Term
- High availability with leader election
- Prometheus metrics and Grafana dashboards
- OpenTelemetry tracing
- Advanced scheduling and resource management
- WebSocket support for real-time updates