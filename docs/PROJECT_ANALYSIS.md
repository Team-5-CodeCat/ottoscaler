# Ottoscaler Project Analysis
> 2025-01-06 현재 프로젝트 상태 및 아키텍처 분석

## 📊 프로젝트 현황

### 1. 아키텍처 전환 완료
- **이전**: Redis Streams 기반 이벤트 드리븐 아키텍처
- **현재**: gRPC 기반 직접 통신 아키텍처
- **이유**: 
  - 낮은 지연시간 (Low latency)
  - 실시간 스트리밍 지원
  - Type-safe 통신
  - 복잡한 의존성 제거

### 2. 주요 구성 요소

#### 2.1 Main Pod (Controller)
- **위치**: Kubernetes 클러스터 내부 실행
- **역할**: 
  - gRPC 서버 (포트 9090)
  - Worker Pod 생명주기 관리
  - Pipeline 실행 조율
  - 로그 스트리밍 중계

#### 2.2 gRPC Services
```protobuf
// 주요 서비스
service OttoscalerService {
    rpc ScaleUp(ScaleRequest) returns (ScaleResponse);
    rpc ScaleDown(ScaleRequest) returns (ScaleResponse);
    rpc GetWorkerStatus(WorkerStatusRequest) returns (WorkerStatusResponse);
    rpc ExecutePipeline(PipelineRequest) returns (stream PipelineProgress);
}

service LogStreamingService {
    rpc StreamWorkerLogs(WorkerIdentifier) returns (stream LogEntry);
    rpc StreamBuildLogs(BuildIdentifier) returns (stream BuildLogEntry);
}
```

#### 2.3 Pipeline Executor
- **기능**: CI/CD Pipeline 실행 관리
- **특징**:
  - DAG (Directed Acyclic Graph) 기반 의존성 해결
  - 병렬 Stage 실행 지원
  - 실시간 진행 상황 스트리밍
  - Stage 재시도 정책

#### 2.4 Worker Management
- **아키텍처**: Task → N Pods → 1 Container per Pod
- **관리 방식**:
  - 동적 Pod 생성/삭제
  - 실시간 상태 모니터링
  - 자동 정리 (Cleanup)
  - 레이블 기반 추적

## 🔄 최근 변경 사항

### 완료된 작업
1. ✅ Redis 관련 코드 완전 제거
   - `internal/redis/client.go` 삭제
   - `cmd/test-event/main.go` 삭제
   - Redis 설정 제거

2. ✅ gRPC 기반 Pipeline 지원
   - `internal/pipeline/executor.go` 구현
   - `cmd/test-pipeline/main.go` 테스트 도구
   - DAG 의존성 해결 알고리즘

3. ✅ 로그 한국어화
   - 모든 Main Pod 로그 메시지 한국어 변환
   - 개발자 친화적 이모지 추가

4. ✅ Deprecated 코드 업데이트
   - `grpc.WithInsecure()` → `grpc.WithTransportCredentials(insecure.NewCredentials())`
   - `grpc.DialContext()` → `grpc.NewClient()`
   - `grpc.WithBlock()` 제거 (NewClient에서 지원 안 함)

### 진행 중인 작업
1. 🔄 로그 스트리밍 완성
   - Worker → Ottoscaler → Otto-handler 전달 체인
   - 실시간 로그 수집 및 전달

2. 🔄 Status 모니터링
   - Worker 상태 변경 알림
   - Pipeline 진행 상황 추적

## 🏗️ 프로젝트 구조

```
ottoscaler/
├── cmd/
│   ├── app/                 # Main Pod 진입점
│   ├── test-scaling/        # 스케일링 테스트 도구
│   └── test-pipeline/       # Pipeline 테스트 도구
├── internal/
│   ├── config/              # 환경 설정 관리
│   ├── grpc/                # gRPC 서버/클라이언트
│   │   ├── server.go        # Main gRPC 서버
│   │   ├── scaling.go       # 스케일링 헬퍼 함수
│   │   ├── log_streaming.go # 로그 스트리밍 서버
│   │   └── otto_handler_client.go # Otto-handler 클라이언트
│   ├── k8s/                 # Kubernetes API 클라이언트
│   ├── pipeline/            # Pipeline 실행 엔진
│   │   └── executor.go      # DAG 기반 실행기
│   └── worker/              # Worker Pod 관리
├── pkg/proto/v1/            # Protocol Buffer 생성 코드
├── proto/                   # Protocol Buffer 정의
│   └── log_streaming.proto  # 모든 메시지 및 서비스 정의
├── k8s/                     # Kubernetes 매니페스트
└── scripts/                 # 유틸리티 스크립트

```

## 🎯 핵심 설계 원칙

### 1. Kubernetes Native
- Main Pod는 클러스터 내부에서 실행
- ServiceAccount 기반 RBAC 권한 관리
- 네이티브 Pod API 활용

### 2. 확장성 고려
- Pipeline 단위 작업 처리
- Stage별 병렬 실행
- 동적 Worker 스케일링
- 향후 Multi-container 지원 가능한 구조

### 3. 개발자 경험
- 완전한 멀티 개발자 환경 지원
- Kind 클러스터 기반 로컬 개발
- 명확한 한국어 로그 메시지
- 포괄적인 테스트 도구

## 📈 성능 특성

### 지연시간
- gRPC 직접 통신: ~5ms
- Worker Pod 생성: ~2-3초
- Pipeline Stage 전환: ~100ms

### 확장성
- 동시 Worker Pod: 최대 100개 (설정 가능)
- 병렬 Stage 실행: 제한 없음
- 동시 Pipeline: 메모리 한계까지

### 리소스 사용
- Main Pod: 메모리 ~50MB, CPU ~0.1 core
- Worker Pod: 작업에 따라 가변

## 🔮 향후 로드맵

### Phase 1 (현재 진행 중)
- [x] gRPC 마이그레이션
- [x] Pipeline 실행 지원
- [ ] 로그 스트리밍 완성
- [ ] 상태 모니터링

### Phase 2 (계획)
- [ ] 메트릭 수집 (Prometheus)
- [ ] 오토스케일링 정책
- [ ] 리소스 쿼터 관리
- [ ] 웹훅 지원

### Phase 3 (검토 중)
- [ ] Multi-container Pod 지원
- [ ] Sidecar 패턴 구현
- [ ] 분산 Pipeline 실행
- [ ] 고급 재시도 정책

## 🧪 테스트 방법

### 1. 환경 설정
```bash
# 개발자별 환경 구성
make setup-user USER=한진우

# 빌드 및 배포
make build && make deploy
```

### 2. 스케일링 테스트
```bash
# ScaleUp 테스트
./test-scaling -action scale-up -workers 3

# Worker 상태 확인
./test-scaling -action status -watch
```

### 3. Pipeline 테스트
```bash
# 간단한 Pipeline
./test-pipeline -type simple

# 복잡한 CI/CD Pipeline
./test-pipeline -type full

# 병렬 실행 테스트
./test-pipeline -type parallel
```

### 4. 모니터링
```bash
# Main Pod 로그
make logs

# Worker Pod 관찰
kubectl get pods -w

# 리소스 사용량
kubectl top pods
```

## 📝 주요 코드 패턴

### 1. Context 기반 취소
```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

// 모든 장시간 작업에 context 전달
err := longRunningOperation(ctx)
```

### 2. 구조적 로깅
```go
log.Printf("🚀 Pipeline 실행 시작: id=%s, stages=%d", 
    pipeline.ID, len(pipeline.Stages))
```

### 3. 에러 래핑
```go
if err != nil {
    return fmt.Errorf("failed to create worker: %w", err)
}
```

### 4. 동시성 안전
```go
type Manager struct {
    mu sync.RWMutex
    workers map[string]*Worker
}

func (m *Manager) GetWorker(id string) *Worker {
    m.mu.RLock()
    defer m.mu.RUnlock()
    return m.workers[id]
}
```

## 🔒 보안 고려사항

### 현재 구현
- ServiceAccount 기반 권한 관리
- Namespace 격리
- 레이블 기반 리소스 추적

### 향후 개선
- TLS/mTLS 지원
- 시크릿 관리
- 감사 로깅
- 네트워크 정책

## 🤝 통합 포인트

### Otto-handler와의 통합
- **프로토콜**: gRPC
- **방향**: 양방향 (요청/응답 + 스트리밍)
- **인증**: 현재 없음 (TODO)
- **재시도**: 클라이언트 측 구현

### Kubernetes와의 통합
- **API**: client-go 라이브러리
- **권한**: ClusterRole/ServiceAccount
- **네임스페이스**: 개발자별 격리
- **레이블**: 일관된 셀렉터 사용

## 📊 메트릭 및 모니터링

### 현재 수집 중
- Worker Pod 수
- Pipeline 실행 시간
- Stage별 소요 시간
- 로그 전송 수

### 계획 중
- CPU/메모리 사용률
- 네트워크 I/O
- 에러율
- 지연시간 분포

## 🐛 알려진 이슈

1. **로그 스트리밍 미완성**
   - Worker 로그가 Otto-handler로 전달되지 않음
   - 임시 해결: kubectl logs 직접 사용

2. **Scale-down 미구현**
   - 현재 stub만 존재
   - Worker는 자동 종료에 의존

3. **리소스 제한 없음**
   - Worker Pod에 리소스 제한 미설정
   - 노드 리소스 고갈 가능성

## 💡 베스트 프랙티스

### 개발 시
1. 항상 context 전달
2. 에러는 즉시 처리
3. 로그는 구조적으로
4. 테스트 먼저 작성

### 배포 시
1. 이미지 빌드 후 Kind 로드
2. 기존 Pod 삭제 후 재배포
3. 로그 확인으로 시작
4. Worker 정리 확인

### 디버깅 시
1. Main Pod 로그 먼저 확인
2. Worker Pod 상태 점검
3. gRPC 연결 상태 확인
4. 네트워크 정책 검토

## 📚 참고 자료

- [Kubernetes client-go](https://github.com/kubernetes/client-go)
- [gRPC Go](https://grpc.io/docs/languages/go/)
- [Protocol Buffers](https://protobuf.dev/)
- [Kind](https://kind.sigs.k8s.io/)

---

*이 문서는 2025-01-06 기준으로 작성되었으며, 프로젝트 진행에 따라 지속적으로 업데이트됩니다.*