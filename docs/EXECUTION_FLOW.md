# Ottoscaler 실행 흐름

## 🔄 런타임 동작

### Main 스레드 vs 고루틴

**Main 스레드 (의도적 블로킹):**
```go
func main() {
    // ... 초기화 ...
    
    // 백그라운드 처리 시작
    go handleScaleEvents(ctx, eventChan, workerManager)
    
    // Main 스레드는 종료 시그널 대기 (의도적 블로킹)
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    <-sigChan  // 🛑 시그널을 받을 때까지 여기서 블로킹
    
    // 우아한 종료
    cancel()
    wg.Wait()
}
```

**왜 Main 스레드가 블로킹되나?**
- **목적**: 종료 시그널 대기 (Kubernetes의 SIGTERM)
- **설계**: Main 스레드는 시그널 핸들러 역할만 수행
- **영향**: 처리에 영향 없음 - 모든 작업은 고루틴에서 수행

## 🚀 동시성 처리 아키텍처

```
┌─────────────────────────────────────────────────────────────────┐
│                     Ottoscaler Main Pod                         │
│                                                                 │
│  Main Thread              고루틴 1              고루틴 2        │
│  ┌──────────┐             ┌──────────────┐      ┌─────────────┐ │
│  │ 시그널   │             │ 이벤트       │      │ Redis       │ │
│  │ 핸들러   │             │ 처리         │      │ 리스너      │ │
│  │          │             │              │      │             │ │
│  │<-sigChan │◄──────────┐ │ for {        │◄─────┤ XReadGroup  │ │
│  │          │           │ │   <-eventCh  │      │ (2초 블록)  │ │
│  │ [BLOCK]  │           │ │   scale_up   │      │             │ │
│  └──────────┘           │ │   scale_down │      └─────────────┘ │
│                         │ │ }            │                     │
│                         │ └──────────────┘                     │
│                         │                                      │
│  고루틴 3,4,5...        │                                      │
│  ┌─────────────────┐    │                                      │
│  │ Worker Pod 1    │    │                                      │
│  │ 생성→대기→삭제  │◄───┘                                      │
│  └─────────────────┘                                           │
│  ┌─────────────────┐                                           │
│  │ Worker Pod 2    │                                           │
│  │ 생성→대기→삭제  │                                           │
│  └─────────────────┘                                           │
│  ┌─────────────────┐                                           │
│  │ Worker Pod N    │                                           │
│  │ 생성→대기→삭제  │                                           │
│  └─────────────────┘                                           │
└─────────────────────────────────────────────────────────────────┘
```

## ⚡ 이벤트 처리 흐름

### 1. Redis 이벤트 도착
```bash
# 외부 트리거
docker exec ottoscaler-redis redis-cli XADD otto:scale:events '*' type scale_up pod_count 3
```

### 2. Redis 리스너 (타임아웃 포함 비블로킹)
```go
// 고루틴 2: Redis 리스너
go func() {
    for {
        // 2초마다 폴링 (블로킹 타임아웃, 무한 대기 아님)
        streams, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
            Block: time.Second * 2,  // ⏰ 2초 타임아웃
        })
        
        if err == redis.Nil {
            continue  // 메시지 없음, 다시 폴링
        }
        
        // 파싱 후 이벤트 채널로 전송
        event := parseScaleEvent(message)
        eventChan <- event  // 📨 고루틴 1로 전송
    }
}()
```

### 3. 이벤트 핸들러 처리
```go
// 고루틴 1: 이벤트 처리
func handleScaleEvents(ctx, eventChan, workerManager) {
    for {
        select {
        case <-ctx.Done():
            return  // 우아한 종료
            
        case event := <-eventChan:  // 📨 Redis 리스너에서 수신
            switch event.Type {
            case "scale_up":
                // 🚀 다중 Worker 고루틴 실행
                handleScaleUp(ctx, workerManager, event.PodCount, event.Metadata)
            }
        }
    }
}
```

### 4. Worker Pod 관리 (동시성)
```go
func handleScaleUp(ctx, workerManager, podCount, metadata) {
    configs := createWorkerConfigs(podCount)
    
    // 🎯 모든 Worker 동시 시작
    return workerManager.RunMultipleWorkers(ctx, configs)
}

func RunMultipleWorkers(ctx, configs) {
    results := make(chan error, len(configs))
    
    // 각 Worker를 별도 고루틴에서 실행
    for _, config := range configs {
        go func(cfg WorkerConfig) {
            // 고루틴 3,4,5...: 독립적인 Worker 관리
            results <- CreateAndWaitForWorker(ctx, cfg)
        }(config)
    }
    
    // 모든 Worker 완료 대기
    for i := 0; i < len(configs); i++ {
        <-results  // 완료 시그널 수집
    }
}
```

### 5. 개별 Worker 라이프사이클
```go
// 고루틴 3,4,5...: Worker별 처리
func CreateAndWaitForWorker(ctx, config) error {
    // 1. Kubernetes Pod 생성
    pod, err := m.k8sClient.CreatePod(ctx, podSpec)
    
    // 2. 완료 모니터링 (2초 간격 폴링)
    ticker := time.NewTicker(2 * time.Second)
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-ticker.C:
            pod, err := m.k8sClient.GetPod(ctx, config.Name)
            
            switch pod.Status.Phase {
            case v1.PodSucceeded:
                // 3. 완료된 Pod 정리
                m.k8sClient.DeletePod(ctx, config.Name)
                return nil
            case v1.PodFailed:
                return fmt.Errorf("pod failed")
            case v1.PodRunning, v1.PodPending:
                // 모니터링 계속
                continue
            }
        }
    }
}
```

## 📊 블로킹 vs 비블로킹 요약

| 컴포넌트 | 블로킹 타입 | 목적 | 영향 |
|-----------|---------------|---------|--------|
| **Main 스레드** | 의도적 블로킹 | 종료 시그널 대기 | ✅ 영향 없음 - 시그널 처리만 |
| **Redis 리스너** | 타임아웃 블로킹 (2초) | 새 이벤트 폴링 | ✅ 비블로킹 - 폴링 계속 |
| **이벤트 핸들러** | 이벤트 드리븐 블로킹 | 처리할 이벤트 대기 | ✅ 비블로킹 - 사용 가능 시 처리 |
| **Worker 모니터링** | 폴링 블로킹 (2초) | Pod 완료 모니터링 | ✅ 독립적 - 각 Worker 별도 |

## 🎯 주요 통찰

### Pod는 블로킹되지 않음
- **Main 스레드는 의도적으로 블로킹** - 시그널 처리만 담당
- **모든 처리는 별도 고루틴에서 동시 수행**
- **다중 Worker가 간섭 없이 동시 관리됨**
- **Redis 이벤트는 사용 가능 시 즉시 처리**

### 확장성 특징
- **이벤트 처리**: 다중 동시 스케일 이벤트 처리 가능
- **Worker 관리**: 무제한 동시 Worker Pod (리소스 허용 범위)
- **리소스 사용**: 효율적인 고루틴 기반 동시성 모델
- **응답 시간**: 거의 즉시 이벤트 처리 (Kubernetes API 제한)

### 장애 모드
- **Worker Pod 실패**: 다른 Worker나 메인 처리에 영향 없음
- **Redis 연결 끊김**: 에러 로깅과 함께 자동 재시도
- **Kubernetes API 문제**: 개별 Worker 실패, 시스템은 계속 동작
- **우아한 종료**: 종료 전 모든 Worker 완료

## 🔧 모니터링 포인트

### 상태 지표
```bash
# Main Pod 상태 확인
kubectl get pods -l app=ottoscaler

# Worker Pod 모니터링
kubectl get pods -l managed-by=ottoscaler

# 처리 로그 확인
kubectl logs -l app=ottoscaler -f

# Redis 연결 상태
kubectl exec ottoscaler-pod -- ping redis
```

### 성능 메트릭
- 이벤트 처리 지연시간
- Worker Pod 생성 시간
- 동시 Worker 수
- Redis 폴링 빈도
- Kubernetes API 응답 시간