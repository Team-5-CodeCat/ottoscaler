# Ottoscaler 아키텍처

## 개요

Ottoscaler는 Redis Streams 이벤트를 기반으로 Otto agent Pod들을 동적으로 스케일링하는 Kubernetes 기반 자동 스케일링 애플리케이션입니다. 스케일링 이벤트를 지속적으로 수신하고 Worker Pod들을 관리하는 Main Pod 역할을 합니다.

## 시스템 아키텍처

```
┌─── External Redis ─────┐    ┌─── Kubernetes Cluster ──────────────────────┐
│                        │    │                                              │
│  Redis Streams         │    │  ┌─────────────┐    ┌──────────────────────┐ │
│  otto:scale:events ────┼────┼─▶│ Ottoscaler  │───▶│ Otto Agent Pods      │ │
│                        │    │  │ (Main Pod)  │    │ (Dynamic Workers)    │ │
│  XADD otto:scale:events│    │  │ - Always On │    │ - Created on demand  │ │
│  type=scale_up         │    │  │ - Event     │    │ - Auto cleanup       │ │
│  pod_count=3           │    │  │   Consumer  │    │                      │ │
│                        │    │  └─────────────┘    └──────────────────────┘ │
└────────────────────────┘    └──────────────────────────────────────────────┘
```

## 핵심 컴포넌트

### 1. Main Pod (Ottoscaler)
- **위치**: `cmd/app/main.go`
- **역할**: 이벤트 드리븐 코디네이터
- **라이프사이클**: 장기 실행 데몬 프로세스
- **책임**:
  - Redis Streams에서 스케일링 이벤트 수신
  - Kubernetes 및 Redis 클라이언트 초기화 및 관리
  - Worker Pod 생성 및 정리 조율
  - 우아한 종료 시그널 처리

### 2. Redis 클라이언트 (`internal/redis/client.go`)
- **목적**: Redis Streams 통합
- **주요 기능**:
  - Consumer Group 관리 (`ottoscaler` 그룹)
  - 2초마다 이벤트 폴링 (타임아웃 포함 블로킹)
  - 자동 메시지 확인 응답
  - 이벤트 파싱 및 검증

### 3. Kubernetes 클라이언트 (`internal/k8s/client.go`)
- **목적**: Kubernetes 클러스터 상호작용
- **기능**:
  - 클러스터 내부 및 kubeconfig 기반 인증
  - Pod CRUD 작업
  - Pod 상태 모니터링
  - 네임스페이스 범위 작업

### 4. Worker 관리자 (`internal/worker/manager.go`)
- **목적**: Otto Agent Pod 라이프사이클 관리
- **기능**:
  - 동시 Worker Pod 생성
  - Pod 완료 모니터링 (2초 간격 폴링)
  - 완료 후 자동 정리
  - 에러 처리 및 복구

## 실행 흐름

### 1. 시작 시퀀스
```go
main() {
    // 1. 환경 설정 읽기
    redisAddr := getEnv("REDIS_HOST", "host.docker.internal") + ":6379"
    streamName := getEnv("REDIS_STREAM", "otto:scale:events")
    
    // 2. 클라이언트 초기화
    k8sClient := k8s.NewClient("default")
    redisClient := redis.NewClient(redisAddr)
    workerManager := worker.NewManager(k8sClient)
    
    // 3. 연결 테스트
    redisClient.Ping() // Redis 사용 불가 시 빠른 실패
    
    // 4. 이벤트 처리 시작
    eventChan := redisClient.ListenForScaleEvents()
    go handleScaleEvents(ctx, eventChan, workerManager)
    
    // 5. 종료 시그널 대기
    <-sigChan // SIGTERM/SIGINT까지 블로킹
}
```

### 2. 이벤트 처리 루프
```go
handleScaleEvents() {
    for {
        select {
        case <-ctx.Done():
            return // 우아한 종료
            
        case event := <-eventChan:
            switch event.Type {
            case "scale_up":
                handleScaleUp(event.PodCount, event.Metadata)
            case "scale_down":
                handleScaleDown(event.PodCount) // TODO: 미구현
            }
        }
    }
}
```

### 3. Worker Pod 관리
```go
handleScaleUp(podCount, metadata) {
    // Worker 설정 생성
    configs := []WorkerConfig{
        {
            Name: "otto-agent-1-{timestamp}",
            Image: "busybox:latest",
            Command: ["sh", "-c"],
            Args: ["echo 'Starting...'; sleep 30; echo 'Done!'"],
            Labels: {"managed-by": "ottoscaler"}
        }
        // ... 더 많은 Worker들
    }
    
    // 모든 Worker 동시 실행
    workerManager.RunMultipleWorkers(configs)
}
```

## 동시성 모델

### 고루틴 구조
```
Main Thread
├── 시그널 핸들러 (SIGTERM/SIGINT 블로킹 대기)
│
├── 이벤트 핸들러 고루틴
│   └── Redis 스케일 이벤트 처리
│
├── Redis 리스너 고루틴  
│   └── 2초마다 Redis Streams 폴링
│
└── Worker 관리 고루틴들 (Worker별)
    ├── Worker 1: 생성 → 모니터링 → 정리
    ├── Worker 2: 생성 → 모니터링 → 정리
    └── Worker N: 생성 → 모니터링 → 정리
```

### 블로킹 vs 비블로킹 작업

**블로킹 작업** (의도적):
- Main 스레드의 종료 시그널 대기
- 2초 타임아웃을 가진 Redis XReadGroup
- Worker 완료 모니터링 (2초 간격 폴링)

**비블로킹 작업**:
- 이벤트 채널 통신
- 동시 Worker Pod 관리
- Kubernetes API 호출 (컨텍스트 취소 포함)

### 주요 특징

1. **이벤트 드리븐**: Redis 이벤트 수신 시에만 동작
2. **동시 처리**: 여러 Worker Pod를 동시에 관리  
3. **자동 정리**: Worker들은 완료 후 자동 삭제
4. **우아한 종료**: SIGTERM/SIGINT 시 적절한 정리
5. **장애 내성**: 개별 Worker 실패 시에도 계속 운영

## 설정

### 환경 변수
```bash
REDIS_HOST=host.docker.internal          # Redis 서버 주소
REDIS_PORT=6379                          # Redis 서버 포트
REDIS_PASSWORD=                          # Redis 비밀번호 (선택사항)
REDIS_STREAM=otto:scale:events           # Redis 스트림 이름
REDIS_CONSUMER_GROUP=ottoscaler          # Consumer Group 이름
REDIS_CONSUMER=ottoscaler-1              # Consumer 인스턴스 이름
OTTO_AGENT_IMAGE=busybox:latest          # Worker Pod 이미지
```

### Redis 이벤트 형식
```bash
XADD otto:scale:events * type scale_up pod_count 3 task_id task-123
```

### 스케일 이벤트 예시
```json
{
  "EventID": "1756659802903-0",
  "Type": "scale_up",
  "PodCount": 3,
  "Timestamp": "2025-08-31T17:03:22Z",
  "Metadata": {
    "task_id": "task-123"
  }
}
```

## 배포

### Kubernetes 리소스
- **ServiceAccount**: `ottoscaler` 
- **ClusterRole**: Pod 관리 권한
- **Deployment**: 단일 복제본 Main Pod
- **ConfigMap/Secrets**: 환경 설정

### 개발 명령어
```bash
# 인프라 관리
make redis           # Redis 컨테이너 시작
make k8s-deploy      # Kubernetes 배포
make test-event      # 테스트 스케일 이벤트 전송

# 모니터링
make k8s-logs        # ottoscaler 로그 확인
kubectl get pods -l managed-by=ottoscaler  # Worker Pod 확인
```

## 제한사항 및 TODO

1. **Scale Down**: 현재 미구현
2. **Worker 지속성**: 장기 실행 Worker들의 상태 관리 없음
3. **리소스 제한**: Worker Pod들에 CPU/메모리 제약 없음
4. **모니터링**: 제한적인 관찰가능성과 메트릭
5. **에러 복구**: 기본적인 에러 처리만 있고, 재시도 메커니즘 없음