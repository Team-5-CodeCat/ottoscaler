# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Ottoscaler is a Kubernetes-native autoscaler written in Go that dynamically manages Otto Agent worker pods. It runs as a main controller pod within a Kubernetes cluster and creates/manages worker pods on demand based on gRPC calls from otto-handler.

### System Architecture

```
Otto-handler (NestJS) → gRPC → Ottoscaler (Main Pod) → Worker Pods
```

- **Main Pod**: Event-driven coordinator running continuously in Kubernetes
- **gRPC Server**: Receives scaling commands from otto-handler on port 9090
- **Worker Management**: Dynamically creates, monitors, and cleans up Otto Agent pods
- **Log Streaming**: Forwards worker logs to otto-handler via gRPC

## Essential Commands

### Multi-User Environment Setup
```bash
make setup-user USER=한진우    # Auto-configure Kind cluster + namespace
make status                    # Check entire environment status
make list-envs                 # Show available environment files
```

### Development Workflow (Kubernetes-native)
```bash
make build                     # Build Docker image
make deploy                    # Deploy to Kind cluster as Main Pod
make logs                      # View Main Pod logs
kubectl get pods -w           # Monitor worker pod lifecycle
```

### Development Tools
```bash
make test                      # Run Go tests with race detection
make fmt                       # Format Go code
make lint                      # Run linter (golangci-lint or go vet)
make proto                     # Generate Protocol Buffer code
```

### Environment Management
```bash
make redis-cli                 # Access Redis CLI (ENV_FILE required)
make k8s-status                # Check Kubernetes cluster status
make clean                     # Complete cleanup (Redis + Kind + images)
```

## Code Architecture

### Go Project Structure (Standard Layout)

```
ottoscaler/
├── cmd/app/                 # Main application entry point
├── internal/                # Private packages (not importable)
│   ├── config/              # Configuration management
│   ├── grpc/                # gRPC server and client implementations
│   ├── k8s/                 # Kubernetes API client  
│   └── worker/              # Worker pod lifecycle management
├── pkg/proto/v1/            # Generated Protocol Buffer code
├── proto/                   # Protocol Buffer definitions
├── k8s/                     # Kubernetes manifests
└── scripts/                 # Build and utility scripts
```

### Core Components

- **Main Pod** (`cmd/app/main.go`): gRPC server that receives scaling commands
- **gRPC Server** (`internal/grpc/`): Handles scaling requests and log forwarding
- **Kubernetes Client** (`internal/k8s/`): Manages pod CRUD operations
- **Worker Manager** (`internal/worker/`): Orchestrates worker pod lifecycle
- **Config** (`internal/config/`): Centralized configuration management

### Execution Model

- **Main Thread**: gRPC server listening for scaling commands
- **Worker Management Goroutines**: Independent creation→monitoring→cleanup for each worker pod
- **Log Streaming**: Forwards worker logs to otto-handler via gRPC


## Key Technologies

### Dependencies
- `k8s.io/client-go` - Kubernetes Go client for cluster interaction
- `google.golang.org/grpc` - gRPC for communication with otto-handler
- `github.com/joho/godotenv` - Environment file loading for multi-user setup

### Environment Variables
```bash
GRPC_PORT=9090                     # gRPC server port
NAMESPACE=default                   # Kubernetes namespace for workers
OTTO_AGENT_IMAGE=busybox:latest    # Worker pod image
OTTO_HANDLER_HOST=                 # Otto-handler address for log streaming
LOG_LEVEL=info                      # Logging level
```

### gRPC Services

#### OttoscalerService
- `ScaleUp`: Creates worker pods for CI/CD tasks
- `ScaleDown`: Terminates worker pods
- `GetWorkerStatus`: Queries worker pod status

#### OttoHandlerLogService
- `ForwardWorkerLogs`: Streams worker logs to otto-handler
- `NotifyWorkerStatus`: Sends worker status changes

## Multi-Developer Resource Allocation

### Developer Assignments

| Developer | Kind Cluster | Namespace | Environment File |
|-----------|-------------|-----------|------------------|
| 한진우 | ottoscaler-hanjinwoo | hanjinwoo-dev | .env.hanjinwoo.local |
| 장준영 | ottoscaler-jangjunyoung | jangjunyoung-dev | .env.jangjunyoung.local |
| 고민지 | ottoscaler-gominji | gominji-dev | .env.gominji.local |
| 이지윤 | ottoscaler-leejiyun | leejiyun-dev | .env.leejiyun.local |
| 김보아 | ottoscaler-kimboa | kimboa-dev | .env.kimboa.local |
| 유호준 | ottoscaler-yoohojun | yoohojun-dev | .env.yoohojun.local |

### Development Workflow

```bash
# Terminal 1: Setup and deploy Main Pod
make setup-user USER=한진우
make build && make deploy
make logs

# Terminal 2: Monitor worker pods  
kubectl get pods -w -n hanjinwoo-dev

# Terminal 3: Test via otto-handler
# Otto-handler sends gRPC requests to create workers
```

## Testing

### Run Tests
```bash
make test           # Run all tests with race detection
go test ./...       # Alternative direct command
```

### Monitor Worker Pods
```bash
kubectl get pods -l managed-by=ottoscaler  # List worker pods
kubectl logs -l app=ottoscaler -f          # Follow main pod logs
```

### Debugging
```bash
kubectl describe pod <pod-name>            # Detailed pod info
kubectl logs <pod-name> --tail=20          # Worker pod logs
```

## Code Standards

### Go Programming Principles

Go 언어로 안정적이고 유지보수성 높은 프로그램을 작성하려면 다음 원칙들을 일관되게 준수해야 합니다.

#### 1. gofmt로 코드 포맷 일관성 유지
- 전체 코드베이스에 `gofmt`를 적용해 들여쓰기, 공백, 줄바꿈 규칙을 자동으로 맞춥니다.
- CI/CD 파이프라인에서 `make fmt` 실행을 필수화합니다.

#### 2. 명확하고 간결한 네이밍
- 식별자는 짧되 의미가 분명해야 합니다.
- 패키지 이름은 소문자 단수형으로, 함수·변수·상수명은 `CamelCase` 스타일로 짓습니다.
- 예: `workerManager`, `ScaleUp`, `GetWorkerStatus`

#### 3. 에러 처리 일관성
- 함수는 오류를 반환값으로 명시적으로 처리합니다.
- `if err != nil { return … }` 패턴을 일관되게 사용합니다.
- `panic`과 `recover`는 예외 상황(초기화 실패 등)에만 제한적으로 씁니다.

#### 4. 인터페이스 최소 원칙
- 필요한 메서드만 정의한 작은 인터페이스를 설계합니다.
- 구현체 측에서 인터페이스를 선언하지 않고, 의존하는 쪽에서 선언하도록 합니다.

#### 5. 구성(composition) 우선, 상속 배제
- 구조체 임베딩으로 기능을 확장하고 재사용합니다.
- Go는 상속을 제공하지 않으므로, "has-a" 관계로 모듈화합니다.

#### 6. 컨텍스트(Context) 활용
- `context.Context`를 모든 API에 첫 매개변수로 전달해 취소·타임아웃·데드라인·값 전파를 일원화합니다.
- 모든 gRPC 메서드와 장시간 실행되는 함수에 context를 전달합니다.

#### 7. 패키지 경계 명확히
- 각 패키지는 단일 책임 원칙을 따릅니다.
- 순환 의존성을 피하고, 내부(`internal` 또는 소문자 시작) 패키지와 외부 API를 구분합니다.
- `internal/` packages는 외부에서 importable하지 않습니다.

#### 8. 병행성 패턴 준수
- 고루틴은 가볍지만 무분별한 사용을 자제하고, `sync.Mutex`·`sync.WaitGroup` 등 동기화 수단을 활용합니다.
- 채널로 소통할 때는 버퍼 크기, 닫기(close) 시점, 셀렉션(`select`) 구조를 명확히 관리합니다.

#### 9. 테스트와 문서화
- 모든 공개 API에는 단위 테스트(`*_test.go`)와 예제 코드(`ExampleXxx`)를 작성합니다.
- `go doc`으로 문서화가 가능하도록 주석을 함수 및 패키지 선언 바로 위에 위치시킵니다.

#### 10. 정적 분석 도구 사용
- `go vet`, `golangci-lint`, `staticcheck` 등을 CI에 연동해 코드 품질과 잠재적 버그를 사전 차단합니다.
- `make lint` 명령어로 정적 분석을 실행합니다.

### Go Conventions
- Use standard Go project layout (`internal/` for private, `pkg/` for public)
- Follow Go naming conventions (PascalCase for exported, camelCase for unexported)
- All long-running operations must use `context.Context`
- Proper error handling with wrapped errors and logging
- Use `log.Printf` for structured logging

### gRPC Patterns
- Bidirectional streaming for log forwarding
- Proper status codes and error messages
- Reconnection logic with exponential backoff
- Request validation before processing


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

## Current Implementation Status

### Completed
- ✅ gRPC server implementation for scaling commands
- ✅ Worker pod creation and management
- ✅ Multi-developer environment support
- ✅ Configuration management system
- ✅ Basic worker lifecycle management

### In Progress
- 🔄 Log streaming from workers to otto-handler
- 🔄 Worker status monitoring and notifications
- 🔄 Scale-down functionality

### TODO
- ⏳ Worker pod log collection and forwarding
- ⏳ Retry mechanisms for failed workers
- ⏳ Metrics and observability
- ⏳ Resource quota management

## Important Development Notes

### Execution Environment
- **Main Pod runs INSIDE Kubernetes cluster** - not externally
- Development cycle: code change → `make build && make deploy` → test in cluster
- ServiceAccount-based RBAC provides necessary pod management permissions
- Each developer works in their own Kubernetes namespace

### Multi-Developer Isolation
- Complete resource isolation per developer (Kind cluster, namespace)
- Environment files auto-generated with developer-specific configuration
- No Redis dependency - direct gRPC communication with otto-handler

### Integration Points
- **gRPC Communication**: Otto-handler → Ottoscaler for scaling commands
- **Log Streaming**: Ottoscaler → Otto-handler for worker logs
- **Status Updates**: Real-time worker status notifications
- **Shared Infrastructure**: Kind clusters for local development

### Testing Strategy
- Unit tests for individual components
- Integration tests with Kind cluster
- gRPC client testing from otto-handler
- Worker pod lifecycle verification