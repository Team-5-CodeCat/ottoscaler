# Ottoscaler 🚀

**Kubernetes 네이티브 오토스케일러 - Redis Streams 기반 동적 Pod 관리 시스템**

Otto agent pods를 현재 이벤트에 따라 스케일하는 Kubernetes 기반 자동 스케일링 애플리케이션입니다. Go로 작성되었으며 Redis Streams를 메시지 큐로 사용하여 이벤트를 소비하고, Kubernetes 클라이언트를 통해 동작합니다.

## 🏗️ 시스템 아키텍처

```
External Redis → Kubernetes Main Pod (Ottoscaler) → Otto Agent Pods
```

- **Main Pod**: 지속적으로 실행되는 이벤트 드리븐 코디네이터
- **Event Processing**: Redis Streams에서 scale_up/scale_down 이벤트 수신
- **Worker Management**: Otto Agent pods의 동적 생성, 모니터링, 정리
- **Concurrent Processing**: 여러 worker pods를 동시에 병렬 관리

## 📁 프로젝트 구조

Ottoscaler는 Go 커뮤니티의 표준 프로젝트 레이아웃을 따릅니다:

```
ottoscaler/
├── cmd/                    # 메인 애플리케이션들
│   └── app/               # Main Pod 애플리케이션 엔트리포인트
├── internal/              # 내부 패키지 (외부 노출 금지)
│   ├── redis/             # Redis Streams 클라이언트
│   ├── k8s/               # Kubernetes API 클라이언트
│   ├── worker/            # Worker Pod 라이프사이클 관리
│   └── app/               # 애플리케이션별 내부 로직
├── pkg/                   # 외부 사용 가능한 라이브러리 코드
│   └── proto/v1/          # 생성된 Protocol Buffer 코드
├── proto/                 # Protocol Buffer 정의 파일
├── k8s/                   # Kubernetes 매니페스트
├── docs/                  # 문서
├── scripts/               # 빌드 및 유틸리티 스크립트
├── configs/               # 설정 파일들
├── examples/              # 예제 코드 (향후)
├── go.mod                 # Go 모듈 정의
├── go.sum                 # Go 모듈 체크섬
├── Makefile               # 빌드 및 개발 명령어
├── Dockerfile             # 프로덕션 이미지
├── dev.Dockerfile         # 개발 환경 이미지
├── README.md              # 프로젝트 소개 및 가이드
└── CLAUDE.md              # Claude Code를 위한 프로젝트 지침
```

### 핵심 컴포넌트

- `cmd/app/main.go`: 메인 애플리케이션 (이벤트 드리븐 서비스)
- `internal/redis/client.go`: Redis Streams 클라이언트 (2초 간격 폴링)
- `internal/k8s/client.go`: Kubernetes 클라이언트 (Pod CRUD 작업)
- `internal/worker/manager.go`: Worker Pod 라이프사이클 관리자

### 실행 모델

- **메인 스레드**: 종료 시그널 대기 (graceful shutdown)
- **이벤트 처리 고루틴**: Redis 이벤트 수신 및 Worker 생성 코디네이션
- **Redis 리스닝 고루틴**: 2초마다 Redis Streams 폴링 (블로킹 타임아웃)
- **Worker 관리 고루틴들**: 각 Worker Pod를 독립적으로 생성→모니터링→정리

## ⚡ gRPC 로그 스트리밍 기능

**현재 상태**: 프로토콜 설계 완료, 구현 단계별 진행 중

- **목적**: Worker Pod의 실시간 로그를 NestJS 서버로 직접 전송
- **아키텍처**: Worker Pod → gRPC Stream → NestJS Server (Main Pod는 Worker 생성만 담당)
- **프로토콜**: `proto/log_streaming.proto` (상세 한국어 주석 포함)

### 구현 원칙
1. **단계적 접근**: Protocol Buffer → 기본 예제 → 실제 통합
2. **초보자 친화적**: 상세 주석, 예제 코드, 체크리스트 제공
3. **테스트 우선**: 각 단계마다 로컬 테스트 후 Kubernetes 통합

## 🚀 개발 환경 시작하기

### 크로스 플랫폼 개발 환경 특징

**모든 개발자가 동일한 환경에서 작업할 수 있도록 컨테이너 기반 개발 환경을 제공합니다.**

- **완전한 크로스 플랫폼**: Windows, macOS, Linux에서 100% 동일한 경험
- **Docker Desktop Kubernetes 통합**: 로컬 K8s 클러스터와 완전 연동
- **모든 개발 도구 포함**: Go, protoc, golangci-lint, kubectl, Starship 프롬프트
- **볼륨 마운트**: 호스트 소스코드 + kubeconfig + Docker 소켓 접근
- **Go 모듈 캐싱**: 영구 볼륨으로 빠른 의존성 관리

### Quick Start

```bash
# 1. 레포지토리 클론
git clone <repository-url>
cd ottoscaler

# 2. 개발 환경 시작 (Redis + 개발 컨테이너)
make dev-start

# 3. 개발 컨테이너 접속
make dev-shell

# ✨ 이제 컨테이너 안에서 모든 개발 작업을 수행합니다!
# 🎯 TIP: 코드 수정은 로컬 IDE에서, 실행은 컨테이너에서!

# 4. (컨테이너 내부에서) 애플리케이션 실행
go run ./cmd/app

# 5. 테스트 및 개발 작업
make test-event      # Redis에 테스트 이벤트 전송
kubectl get pods     # Worker Pod 상태 확인
make proto           # Protocol Buffer 코드 생성
make test            # 테스트 실행
make fmt             # 코드 포맷팅
make lint            # 코드 린트
```

## 💡 개발 워크플로우

**핵심 포인트**: 코드 편집은 호스트(로컬)에서, 실행은 컨테이너에서!

1. **컨테이너 시작**: `make dev-start` (한 번만)
2. **컨테이너 접속**: `make dev-shell` 
3. **코드 편집**: VS Code, IntelliJ 등 로컬 IDE 사용 → **실시간으로 컨테이너에 반영됨**
4. **개발 작업**: 컨테이너 내부에서 모든 `make` 명령어 실행
5. **반복**: 코드 수정 → 컨테이너에서 테스트 → 반복

```bash
# 예시 개발 세션
make dev-shell                    # 컨테이너 접속

# (컨테이너 내부에서 - Starship 프롬프트 표시)
🐹 go run ./cmd/app              # 애플리케이션 실행
🐹 make test-event               # 이벤트 테스트
🐹 kubectl get pods -w          # Pod 상태 실시간 모니터링
🐹 make proto                    # gRPC 코드 생성
🐹 make test                     # 테스트 실행
🐹 make fmt && make lint         # 코드 품질 검사
```

## 📖 Make 명령어 가이드

### 📦 개발 환경 관리
- `make dev-build` - 개발 환경 이미지 빌드
- `make dev-start` - 개발 환경 시작 (Redis + Dev Container)
- `make dev-shell` - 개발 컨테이너에 접속 (여기서 모든 개발 작업 수행)
- `make dev-stop` - 개발 환경 중지
- `make dev-clean` - 개발 환경 완전 정리

### 🔧 개발 도구 (호스트/컨테이너 어디서든 사용 가능!)
- `make proto` - Protocol Buffer 코드 생성
- `make test` - 테스트 실행
- `make fmt` - 코드 포맷팅
- `make lint` - 코드 린트

### 🎯 테스트 & 디버깅 (호스트/컨테이너 어디서든 사용 가능!)
- `make test-event` - Redis에 테스트 이벤트 전송
- `make redis-cli` - Redis CLI 접속

### 🏭 프로덕션
- `make build` - 프로덕션 이미지 빌드
- `make deploy` - Kubernetes 배포
- `make logs` - 배포된 Pod 로그 조회

### 🧹 정리
- `make clean` - 모든 리소스 정리

## 🎭 자주 묻는 질문

**Q: 코드를 수정했는데 컨테이너에 반영이 안 돼요!**
A: 볼륨 마운트로 실시간 반영됩니다. 파일 저장 후 컨테이너에서 `ls -la` 확인해보세요.

**Q: 컨테이너를 재시작해야 하나요?**
A: 개발 중에는 거의 필요 없습니다. 코드 수정 → 컨테이너에서 `go run` 만 하시면 됩니다.

**Q: Make 명령어를 어디서 실행해야 하나요?**
A: 이제 **호스트와 컨테이너 양쪽에서 모두 동작**합니다! 편한 곳에서 사용하세요.

**Q: 여러 터미널을 열어야 하나요?**
A: 선택사항입니다. 하나는 애플리케이션 실행용, 하나는 테스트/명령어용으로 나누면 편합니다.

## 🛠️ 개발 환경 세부사항

### 포함된 도구들
- **Go 1.24.6**: 최신 Go 런타임 및 도구체인
- **Protocol Buffers**: protoc + Go/gRPC 플러그인
- **Code Quality**: golangci-lint, goimports, go vet
- **Kubernetes**: kubectl (최신 버전)
- **Database**: Redis CLI 접근
- **Shell Enhancement**: Starship 프롬프트 (Ottoscaler 최적화 설정)
- **Editor**: nano, vim

### 볼륨 마운트 (실시간 코드 동기화)
- `$(PWD):/workspace` - 소스코드 (읽기/쓰기) → **로컬 수정사항 실시간 반영**
- `$(HOME)/.kube:/root/.kube:ro` - Kubernetes 설정 (읽기 전용)
- `/var/run/docker.sock:/var/run/docker.sock` - Docker 소켓 접근
- `ottoscaler-go-cache:/go/pkg/mod` - Go 모듈 캐시 (영구 저장)

### 네트워크
- `--network host` - 호스트 네트워크 사용으로 Redis/K8s 직접 접근

## 실제 개발 시나리오 예시

```bash
# Terminal 1: 호스트
make dev-start                    # 환경 시작 (한 번만)
make dev-shell                    # 컨테이너 접속

# Terminal 2: VS Code에서 코드 수정
# - internal/worker/manager.go 수정
# - 파일 저장 → 자동으로 컨테이너에 반영

# Terminal 1: 컨테이너 내부
🐹 go run ./cmd/app              # 수정된 코드로 애플리케이션 실행
🐹 make test-event               # 이벤트 테스트
🐹 kubectl get pods -w          # Pod 상태 실시간 모니터링

# Terminal 2: 또는 새 터미널에서
make test                        # 호스트에서도 테스트 실행 가능!
make lint                        # 호스트에서도 린트 실행 가능!
```

## 📚 관련 문서

- `docs/ARCHITECTURE.md` - 상세 아키텍처 설계
- `docs/GRPC_STREAMING_REQUIREMENTS.md` - 요구사항 분석 및 아키텍처 설계
- `docs/GRPC_IMPLEMENTATION_GUIDE.md` - 단계별 구현 가이드 (초보자 친화적)
- `docs/GRPC_WORKFLOW.md` - 일반적인 gRPC 개발 워크플로우
- `CLAUDE.md` - Claude Code를 위한 프로젝트 지침

## 🔗 주요 기술 스택

### Core Dependencies
- `k8s.io/client-go` - Kubernetes Go client for cluster interaction
- `k8s.io/api` - Kubernetes API types
- `k8s.io/apimachinery` - Kubernetes API machinery
- `github.com/redis/go-redis/v9` - Redis client for consuming messages from Redis Streams
- `github.com/spf13/cobra` - CLI framework for building command-line applications
- `github.com/spf13/pflag` - POSIX/GNU-style command-line flag parsing

### Integration Points
- **Kubernetes**: For managing and scaling Otto agent pods based on events
- **Redis Streams**: As a message queue system where this application acts as a consumer
- **gRPC Streams**: For real-time streaming of pod stdout/stderr to Node.js+NestJS backend server
- **Cobra CLI**: For command-line interface and subcommand structure

---

**🎉 시작하세요!** `make dev-start && make dev-shell` 명령어로 바로 개발을 시작할 수 있습니다.