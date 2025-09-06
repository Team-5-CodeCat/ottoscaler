# Ottoscaler

Kubernetes 네이티브 오토스케일러 - Redis Streams 기반 동적 Pod 관리 시스템입니다.

## 🎯 프로젝트 개요

Ottoscaler는 **Kubernetes 클러스터 내에서 Main Pod로 실행**되는 Go 애플리케이션입니다. Redis Streams 이벤트를 기반으로 Otto agent Worker Pods를 동적으로 스케일링하고 관리합니다.

### 핵심 아키텍처
```
┌─── External Redis ─────┐    ┌─── Kubernetes Cluster ──────────────────────┐
│                        │    │                                              │
│  Redis Streams         │    │  ┌─────────────┐    ┌──────────────────────┐ │
│  otto:scale:events ────┼────┼─▶│ Ottoscaler  │───▶│ Otto Agent Pods      │ │
│                        │    │  │ (Main Pod)  │    │ (Dynamic Workers)    │ │
│                        │    │  └─────────────┘    └──────────────────────┘ │
└────────────────────────┘    └──────────────────────────────────────────────┘
```

### 핵심 구조
- **Main Pod (Ottoscaler)**: Kubernetes 내에서 상시 실행되는 컨트롤러 Pod
- **Worker Pods**: Main Pod가 동적으로 생성/관리하는 Otto agent 작업 실행 Pod
- **Redis Streams**: 클러스터 외부의 스케일링 이벤트 전달 메커니즘
- **Pod 오케스트레이션**: Main Pod가 Worker Pod들의 전체 라이프사이클 관리

## 🛠️ 기술 스택

- **Runtime**: Go 1.24 with standard project layout
- **Kubernetes**: client-go for cluster API interaction  
- **Redis**: go-redis/v9 for Streams message consumption
- **Development**: Kind (Kubernetes in Docker) for local clusters
- **Container**: Multi-stage Docker build with Alpine base
- **Orchestration**: ServiceAccount-based RBAC for pod management

## 🚀 빠른 시작 (Kubernetes 환경 개발)

### 자동 개발환경 설정 (권장)

```bash
# 1. 개발환경 자동 설정 (Kind 클러스터 + Redis)
make setup-user USER=한진우
# 또는: ./scripts/setup-user-env.sh 한진우

# 2. Main Pod 배포
make build && make deploy

# 3. 테스트 이벤트 전송
ENV_FILE=".env.jinwoo.local" make test-event

# 4. Worker Pod 상태 모니터링
kubectl get pods -w
```

### 개발자별 리소스 할당

| 개발자 | Redis | Kind 클러스터 | 네임스페이스 | 환경파일 |
|--------|-------|---------------|-------------|----------|
| 한진우 | 6379 | ottoscaler-jinwoo | jinwoo-dev | .env.jinwoo.local |
| 장준영 | 6380 | ottoscaler-junyoung | junyoung-dev | .env.junyoung.local |
| 고민지 | 6381 | ottoscaler-minji | minji-dev | .env.minji.local |
| 이지윤 | 6382 | ottoscaler-jiyoon | jiyoon-dev | .env.jiyoon.local |
| 김보아 | 6383 | ottoscaler-boa | boa-dev | .env.boa.local |
| 유호준 | 6384 | ottoscaler-hojun | hojun-dev | .env.hojun.local |

## 📋 주요 명령어

### 🚀 Kubernetes 환경 개발

```bash
# 환경 설정 (최초 1회)
make setup-user USER=한진우

# Main Pod 개발 사이클
make build                    # Docker 이미지 빌드
make deploy                   # Kind 클러스터에 배포
make logs                     # Main Pod 로그 확인

# 테스트 및 디버깅
ENV_FILE=".env.jinwoo.local" make test-event  # Redis 이벤트 전송
make status                   # 전체 환경 상태 확인
kubectl get pods -w          # Worker Pod 라이프사이클 모니터링
```

### 개발 도구
```bash
make test         # Go 테스트 실행
make fmt          # 코드 포맷팅
make lint         # 린트 검사
make proto        # Protocol Buffer 코드 생성 (TODO)
```

### 환경 관리
```bash
make clean        # 모든 리소스 정리 (Redis + Kind + 이미지)
make redis-cli    # Redis CLI 접속
make k8s-status   # Kubernetes 클러스터 상태 확인
```

## 🏗️ 프로젝트 구조 (Go 표준 레이아웃)

```
ottoscaler/
├── cmd/                    # 메인 애플리케이션들
│   ├── app/               # Main Pod 애플리케이션 엔트리포인트
│   └── test-event/        # Redis 테스트 이벤트 전송 도구
├── internal/              # 내부 패키지 (외부 노출 금지)
│   ├── redis/             # Redis Streams 클라이언트
│   ├── k8s/               # Kubernetes API 클라이언트
│   ├── worker/            # Worker Pod 라이프사이클 관리
│   └── app/               # 애플리케이션별 내부 로직
├── pkg/                   # 외부 사용 가능한 라이브러리 코드
│   └── proto/v1/          # 생성된 Protocol Buffer 코드
├── proto/                 # Protocol Buffer 정의 파일
├── k8s/                   # Kubernetes 매니페스트
├── scripts/               # 빌드 및 유틸리티 스크립트
└── docs/                  # 문서
```

### 핵심 컴포넌트
- **Main Pod** (`cmd/app/main.go`): 이벤트 드리븐 컨트롤러 (Kubernetes 내부 실행)
- **Redis Client** (`internal/redis/client.go`): Consumer Group 관리, 2초 간격 폴링
- **Kubernetes Client** (`internal/k8s/client.go`): 클러스터 내부 인증, Pod CRUD 작업
- **Worker Manager** (`internal/worker/manager.go`): 동시 Worker Pod 생성/완료/정리

## 🔧 개발 환경

### Kubernetes 환경에서 Main Pod 개발

**핵심 철학**: **실제 Kubernetes 환경에서 Main Pod로 개발**
- Kind로 로컬 K8s 클러스터 구성
- Main Pod로 배포하여 실제 동작 확인  
- 코드 수정 → 이미지 재빌드 → Pod 재배포

**개발 워크플로우**:
```bash
# VS Code에서 코드 수정
# ↓
make build && make deploy    # 이미지 빌드 + Main Pod 재배포
# ↓ 
kubectl get pods -w         # Worker Pod 생성/관리 확인
```

### 공유 리소스

**Redis 컨테이너**: `redis-{개발자영문명}`
- otto-handler와 공유 사용
- 자동 생성/재사용 로직 적용

**Kind 클러스터**: `ottoscaler-{개발자영문명}`
- 개발자별 독립 Kubernetes 환경
- ServiceAccount 기반 RBAC 설정

### 환경 설정

자동 생성되는 `.env.jinwoo.local` 파일 예시:
```env
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_STREAM=otto:scale:events
REDIS_CONSUMER_GROUP=ottoscaler-jinwoo
NAMESPACE=jinwoo-dev
OTTO_AGENT_IMAGE=busybox:latest
KIND_CLUSTER_NAME=ottoscaler-jinwoo
```

## 🎯 실행 모델

### 동시성 구조
- **메인 스레드**: 종료 시그널 대기 (graceful shutdown)
- **이벤트 처리 고루틴**: Redis 이벤트 수신 및 Worker 생성 코디네이션
- **Redis 리스닝 고루틴**: 2초마다 Redis Streams 폴링 (블로킹 타임아웃)
- **Worker 관리 고루틴들**: 각 Worker Pod를 독립적으로 생성→모니터링→정리

### Redis 이벤트 처리 플로우
```bash
# 이벤트 전송
XADD otto:scale:events * type scale_up pod_count 3 task_id task-123

# Main Pod에서 처리
📨 Received scaling event: scale_up (PodCount: 3)
🚀 Creating worker pod: otto-agent-XXX-1
🚀 Creating worker pod: otto-agent-XXX-2  
🚀 Creating worker pod: otto-agent-XXX-3
⏳ Monitoring pod completion...
✅ All 3 workers completed successfully!
🧹 Cleaning up pods...
```

## 🔗 연동 프로젝트

### Otto Handler (NestJS 백엔드)
- **Redis 공유**: 스케일링 이벤트 수신
- **포트**: 6379-6384 공유
- **통신**: Redis Streams → gRPC 스트리밍 (예정)

### 향후 gRPC 통신 (TODO)
- **양방향 스트리밍**: Worker Pod ↔ NestJS 서버
- **로그 전송**: 실시간 빌드/테스트 로그 스트리밍
- **Protocol Buffer**: `proto/log_streaming.proto` 기반

## 📊 Kubernetes 배포

### RBAC 및 ServiceAccount
- **ServiceAccount**: `ottoscaler` (네임스페이스별)
- **ClusterRole**: Pod 관리 권한 (생성/조회/삭제)
- **Deployment**: 단일 레플리카 Main Pod
- **Labels**: Worker Pod에 `managed-by=ottoscaler` 라벨 적용

### 리소스 관리
- Main Pod가 클러스터 내부에서 지속 실행
- Worker Pod 온디맨드 생성/삭제
- 자동 정리 및 실패 시 재시도 로직
- 개발자별 네임스페이스 격리

## 🧪 테스팅

### 테스트 전략
```bash
go test ./...                    # 전체 테스트 실행
go test -race ./...              # 레이스 컨디션 검사
go test -cover ./...             # 커버리지 측정
```

### 통합 테스트 시나리오
1. **Redis 연결 테스트**: `make test-event`
2. **Worker Pod 생성 테스트**: 스케일링 이벤트 전송 후 확인
3. **부하 테스트**: 다중 Worker Pod 동시 생성
4. **실패 복구 테스트**: Pod 실패 시 정리 로직 확인

## 💡 개발 시나리오

### 일반적인 개발 플로우

```bash
# Terminal 1: Main Pod 배포 및 모니터링
make setup-user USER=한진우
make build && make deploy
make logs

# Terminal 2: Worker Pod 상태 모니터링  
kubectl get pods -w -n jinwoo-dev

# Terminal 3: 테스트 이벤트 전송
ENV_FILE=".env.jinwoo.local" make test-event

# VS Code: 코드 수정 후
make build && make deploy        # 재배포
```

### 서버 앱 통합 개발

각 개발자는 독립적인 개발 환경을 가집니다:
```
개발자A 서버앱 → Redis A → Ottoscaler A → Kind Cluster A → Worker Pods A
개발자B 서버앱 → Redis B → Ottoscaler B → Kind Cluster B → Worker Pods B
```

## 🛠️ 문제 해결

### 일반적인 문제

**Redis 연결 실패**:
```bash
ENV_FILE=".env.jinwoo.local" make redis-cli
redis-cli> PING                # Redis 연결 테스트
```

**Kind 클러스터 문제**:
```bash
make k8s-status                 # 클러스터 상태 확인
kubectl get pods -A            # 모든 네임스페이스 Pod 확인
kind get clusters              # Kind 클러스터 목록
```

**Worker Pod 디버깅**:
```bash
kubectl get pods -l managed-by=ottoscaler -n jinwoo-dev
kubectl logs -l managed-by=ottoscaler -n jinwoo-dev --tail=20
kubectl describe pod <pod-name> -n jinwoo-dev
```

### 개발 환경 재설정
```bash
make clean                     # 모든 리소스 정리
make setup-user USER=한진우     # 환경 재생성
```

## 🔒 보안 고려사항

### Kubernetes 보안
- **최소 권한 원칙**: RBAC에서 Pod 관리에 필요한 권한만 부여
- **네임스페이스 격리**: 개발자별 완전한 리소스 분리
- **ServiceAccount**: 전용 계정으로 클러스터 인증

### 컨테이너 보안
- **정적 바이너리**: CGO_ENABLED=0으로 보안 강화
- **최소 이미지**: Alpine 기반 최소한의 의존성
- **루트 권한 없음**: 비특권 컨테이너 실행

## 🚦 현재 제약사항

- **Scale Down**: 아직 구현되지 않음 (Worker Pod 생성만 지원)
- **리소스 제한**: Worker Pod에 CPU/메모리 제약 설정 필요
- **장기 실행 Job**: 현재는 단기 작업만 가정
- **모니터링**: 제한적인 메트릭스 및 관찰 가능성

## 🎛️ 환경별 설정

### 개발 환경 (현재)
- **Redis**: Docker 컨테이너로 로컬 실행
- **Kubernetes**: Kind로 로컬 클러스터 구성
- **이미지**: `imagePullPolicy: Never`로 로컬 빌드 사용

### 프로덕션 환경 (향후)
- **Redis**: 외부 Redis 클러스터 연결
- **Kubernetes**: EKS 클러스터 내부 배포
- **이미지**: ECR 레지스트리에서 이미지 pull

## 📚 추가 자료

- [Kubernetes Go Client 문서](https://pkg.go.dev/k8s.io/client-go)
- [Redis Go Client 문서](https://pkg.go.dev/github.com/redis/go-redis/v9)
- [Kind 사용 가이드](https://kind.sigs.k8s.io/docs/)
- [Go 표준 프로젝트 레이아웃](https://github.com/golang-standards/project-layout)
- [프로젝트 CLAUDE.md](../CLAUDE.md) - AI 어시스턴트 가이드

## 🤔 자주 묻는 질문

**Q: 다른 개발자가 내 환경에 영향을 주나요?**  
A: 아니요! Redis, Kind 클러스터, 네임스페이스 모두 완전히 격리됩니다.

**Q: Main Pod와 Worker Pod의 차이점은 무엇인가요?**  
A: Main Pod는 컨트롤러로 지속 실행되고, Worker Pod는 작업 수행 후 자동 삭제됩니다.

**Q: 개발 시 매번 재배포해야 하나요?**  
A: 네. 실제 Kubernetes 환경에서 개발하므로 코드 변경 시 `make build && make deploy` 필요합니다.

**Q: 환경을 재설정하고 싶어요.**  
A: `make clean && make setup-user USER=한진우`로 깨끗하게 재시작할 수 있습니다.

---

**Ottoscaler는 Kubernetes 네이티브 환경에서 효율적인 동적 Pod 오케스트레이션을 제공합니다.**