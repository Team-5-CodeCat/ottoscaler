# Ottoscaler

gRPC 기반 Kubernetes Worker Pod 오케스트레이터 - Otto CI/CD 플랫폼의 동적 워커 관리 시스템

## 🎯 프로젝트 개요

Ottoscaler는 Kubernetes 클러스터 내에서 Main Pod로 실행되는 Go 애플리케이션입니다. Otto-handler로부터 gRPC 요청을 받아 Worker Pod를 동적으로 생성하고 관리합니다.

### 핵심 아키텍처

```
Otto-handler (NestJS) → gRPC → Ottoscaler (Main Pod) → Worker Pods
```

- **Main Pod**: Kubernetes 내에서 상시 실행되는 gRPC 서버
- **gRPC Server**: otto-handler의 스케일링 명령 수신 (포트 9090)
- **Worker Management**: Otto Agent Pod 동적 생성/모니터링/정리
- **Log Streaming**: Worker 로그를 otto-handler로 실시간 전달 (구현 중)

## 🚀 빠른 시작

### 1. 환경 구성 및 배포

```bash
# Kind 클러스터 및 환경 설정 (최초 1회)
make setup-user USER=한진우

# Docker 이미지 빌드
make build

# Kind 클러스터에 배포
make deploy

# 로그 확인
make logs
```

### 2. 테스트 실행

```bash
# 테스트 바이너리 빌드
go build -o test-scaling ./cmd/test-scaling

# 포트 포워딩 (별도 터미널에서 실행)
kubectl port-forward deployment/ottoscaler 9090:9090

# Worker Pod 생성 테스트
./test-scaling -action scale-up -workers 3 -task my-task-123

# Worker 상태 조회
./test-scaling -action status

# Scale down 테스트
./test-scaling -action scale-down -workers 0
```

### 3. 동작 확인

```bash
# Worker Pod 모니터링
kubectl get pods -w

# Ottoscaler 로그 확인
kubectl logs -l app=ottoscaler -f

# 생성된 Worker Pod 확인
kubectl get pods -l managed-by=ottoscaler
```

## 📋 테스트 도구

### test-scaling: 스케일링 테스트

`test-scaling`은 otto-handler 역할을 대신하여 Ottoscaler의 스케일링 API를 테스트하는 클라이언트입니다.

### 사용법

```bash
# 도움말 보기
./test-scaling -h

# Scale up 예제
./test-scaling -action scale-up -workers 5 -task build-123

# 상태 조회
./test-scaling -action status

# Scale up 후 상태 모니터링
./test-scaling -action scale-up -workers 3 -watch
```

### 옵션

- `-action`: 수행할 작업 (`scale-up`, `scale-down`, `status`)
- `-workers`: 생성/관리할 Worker 수
- `-task`: 작업 ID (자동 생성 가능)
- `-server`: Ottoscaler 서버 주소 (기본값: `localhost:9090`)
- `-watch`: 스케일링 후 상태 모니터링
- `-timeout`: 요청 타임아웃 (기본값: 30초)

### test-pipeline: Pipeline 실행 테스트

`test-pipeline`은 CI/CD Pipeline 실행을 테스트하는 도구입니다.

```bash
# 간단한 순차 Pipeline (build → test → deploy)
./test-pipeline -type simple

# 복잡한 CI/CD Pipeline (병렬 테스트 포함)
./test-pipeline -type full

# 병렬 실행 테스트 (동시 실행 Stage)
./test-pipeline -type parallel
```

**옵션:**
- `-server`: Ottoscaler 서버 주소 (기본값: localhost:9090)
- `-type`: Pipeline 유형 (simple, full, parallel)
- `-id`: Pipeline ID (자동 생성)
- `-repo`: Git 저장소 URL
- `-sha`: Commit SHA
- `-timeout`: 실행 타임아웃 (기본값: 10분)

## 🏗️ 프로젝트 구조

```
ottoscaler/
├── cmd/
│   ├── app/                 # Main Pod 애플리케이션
│   └── test-scaling/         # 테스트 클라이언트
├── internal/
│   ├── config/              # 설정 관리
│   ├── grpc/                # gRPC 서버 구현
│   ├── k8s/                 # Kubernetes 클라이언트
│   └── worker/              # Worker Pod 관리
├── pkg/proto/v1/            # Protocol Buffer 생성 코드
├── proto/                   # Protocol Buffer 정의
├── k8s/                     # Kubernetes 매니페스트
└── scripts/                 # 유틸리티 스크립트
```

## 🔧 주요 명령어

### 개발 도구

```bash
make test          # Go 테스트 실행
make fmt           # 코드 포맷팅
make lint          # 린트 검사
make proto         # Protocol Buffer 코드 생성
```

### 환경 관리

```bash
make status        # 전체 환경 상태 확인
make k8s-status    # Kubernetes 클러스터 상태
make clean         # 모든 리소스 정리
```

## 📊 테스트 시나리오

### 현재 구현된 기능

- ✅ **Pipeline 실행**: CI/CD Pipeline 관리
  - DAG 기반 의존성 해결
  - 병렬 Stage 실행 지원
  - 실시간 진행 상황 스트리밍
  - Stage별 재시도 정책

- ✅ **ScaleUp/ScaleDown**: Worker Pod 관리
  - gRPC 요청 기반 동적 생성
  - 지정된 수만큼 Worker Pod 생성
  - 자동 생명주기 관리

- ✅ **gRPC 서버**: 완전한 API 구현
  - ExecutePipeline 스트리밍 RPC
  - ScaleUp/ScaleDown 동기 RPC
  - GetWorkerStatus 상태 조회
  - Mock 모드 지원

### 구현 중인 기능

- 🔄 **Log Forwarding**: Worker → Otto-handler 로그 전달
- 🔄 **Status Notifications**: 실시간 상태 변경 알림
- 🔄 **Metrics Collection**: Prometheus 메트릭 수집

## 🛠️ 기술 스택

- **Runtime**: Go 1.24
- **Kubernetes**: client-go for API interaction
- **gRPC**: google.golang.org/grpc v1.68.1
- **Protocol Buffers**: google.golang.org/protobuf v1.36.1
- **Development**: Kind for local Kubernetes
- **Container**: Multi-stage Docker build

## 🏗️ 아키텍처

```
Otto-handler (NestJS)
        |
   gRPC (9090)
        v
Ottoscaler (Main Pod)
        |
   Kubernetes API
        v
    Worker Pods
```

### 핵심 컴포넌트

1. **gRPC Server**: 스케일링 및 Pipeline 실행 요청 처리
2. **Pipeline Executor**: DAG 의존성 해결 및 Stage 병렬 실행
3. **Worker Manager**: Pod 생명주기 관리 및 모니터링
4. **Log Streaming**: 실시간 로그 수집 및 전달

## 🤝 통합 포인트

### Otto-handler와의 연동

1. **Scaling Commands**: otto-handler → Ottoscaler
   - ScaleUp/ScaleDown 요청
   - Worker 상태 조회

2. **Log Forwarding**: Ottoscaler → otto-handler
   - Worker Pod 로그 스트리밍
   - 상태 변경 알림

## 👥 멀티 개발자 환경

각 개발자는 독립된 Kind 클러스터와 네임스페이스를 사용합니다:

| 개발자 | Kind 클러스터 | 네임스페이스 |
|--------|--------------|-------------|
| 한진우 | ottoscaler-hanjinwoo | hanjinwoo-dev |
| 장준영 | ottoscaler-jangjunyoung | jangjunyoung-dev |
| 고민지 | ottoscaler-gominji | gominji-dev |
| 이지윤 | ottoscaler-leejiyun | leejiyun-dev |
| 김보아 | ottoscaler-kimboa | kimboa-dev |
| 유호준 | ottoscaler-yoohojun | yoohojun-dev |

### 개발자별 환경 설정

```bash
# 개발자별 환경 자동 구성
make setup-user USER=한진우

# 환경 상태 확인
make status
```

## 📝 환경 변수

```bash
GRPC_PORT=9090                  # gRPC 서버 포트
NAMESPACE=default                # Worker Pod 네임스페이스
OTTO_AGENT_IMAGE=busybox:latest # Worker Pod 이미지
LOG_LEVEL=info                   # 로깅 레벨
```

## 🔍 디버깅

### Pod 상태 확인

```bash
# Ottoscaler Main Pod 로그
kubectl logs -l app=ottoscaler -f

# Worker Pod 목록
kubectl get pods -l managed-by=ottoscaler

# Pod 상세 정보
kubectl describe pod <pod-name>
```

### 일반적인 문제 해결

**Image Pull 에러**:
```bash
# Docker 이미지를 Kind 클러스터에 로드
make build
kind load docker-image ottoscaler:latest --name ottoscaler-hanjinwoo
```

**포트 포워딩 실패**:
```bash
# 기존 포트 포워딩 프로세스 종료
pkill -f "port-forward.*9090"

# 다시 시작
kubectl port-forward deployment/ottoscaler 9090:9090
```

## 🚦 프로젝트 상태

### 완료된 기능
- ✅ gRPC 서버 구현
- ✅ Worker Pod 생성 및 관리
- ✅ 멀티 개발자 환경 지원
- ✅ 기본 Worker 생명주기 관리

### 진행 중
- 🔄 Worker 로그 스트리밍
- 🔄 상태 모니터링 개선
- 🔄 Scale-down 기능

### 예정
- ⏳ 로그 수집 및 전달
- ⏳ 실패 시 재시도 메커니즘
- ⏳ 메트릭스 및 모니터링
- ⏳ 리소스 쿼터 관리

## 📚 추가 문서

- [CLAUDE.md](./CLAUDE.md) - AI 어시스턴트를 위한 프로젝트 가이드
- [DEVELOPMENT.md](./DEVELOPMENT.md) - 상세 개발 환경 설정

## 📄 라이선스

MIT License

---

**Ottoscaler는 Otto CI/CD 플랫폼의 핵심 컴포넌트로서 효율적인 Worker Pod 오케스트레이션을 제공합니다.**