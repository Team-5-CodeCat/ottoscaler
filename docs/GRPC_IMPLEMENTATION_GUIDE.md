# gRPC 구현 가이드 - Ottoscaler 프로젝트

## 📚 **gRPC 기본 개념 (초보자용)**

### gRPC란?

- **Google Remote Procedure Call**의 줄임말
- HTTP/2 기반의 고성능 RPC 프레임워크
- Protocol Buffers를 사용한 언어 간 통신
- 스트리밍과 양방향 통신 지원

### 왜 gRPC를 사용하나?

1. **높은 성능**: HTTP/2 + Binary 직렬화
2. **타입 안전성**: Protocol Buffers의 강타입 시스템
3. **스트리밍**: 실시간 데이터 전송에 최적
4. **다언어 지원**: Go, TypeScript, Python 등

## 🏗️ **Ottoscaler gRPC 아키텍처**

```
┌─────────────────┐     gRPC Stream     ┌─────────────────┐
│   Worker Pod    │────────────────────▶│   NestJS Server │
│                 │                     │                 │
│ 1. 로그 수집     │◀──────────────────── │ 1. 로그 처리       │
│ 2. gRPC Client  │     LogResponse     │ 2. gRPC Server  │
│ 3. 스트림 전송     │                     │ 3. 웹 인터페이스   │
└─────────────────┘                     └─────────────────┘
        ▲
        │ Worker 생성
        │
┌─────────────────┐
│   Main Pod      │
│  (Ottoscaler)   │
│                 │
│ - Redis 이벤트   │
│ - Worker 관리    │
│ - 라이프사이클    │
└─────────────────┘
```

## 📋 **단계별 구현 계획**

### Phase 1: Protocol Buffer 설정 ✅

- [x] `.proto` 파일 작성 완료
- [x] 상세 한국어 주석 추가
- [ ] Protocol Buffer 코드 생성

### Phase 2: 기본 gRPC 서버 구현

```go
// Worker Pod에서 사용할 gRPC 클라이언트
type LogStreamClient struct {
    conn   *grpc.ClientConn
    client pb.LogStreamingServiceClient
    stream pb.LogStreamingService_StreamLogsClient
}
```

### Phase 3: 로그 수집 시스템

```go
// Worker Pod 내부에서 stdout/stderr 캐치
func (w *Worker) collectLogs() {
    // cmd.Stdout을 파이프로 연결
    // 실시간으로 로그 라인 읽기
    // gRPC 스트림으로 전송
}
```

### Phase 4: NestJS 통합

```typescript
// NestJS에서 gRPC 서버 구현
@GrpcService()
export class LogStreamingService {
  @GrpcStreamMethod("LogStreamingService", "StreamLogs")
  streamLogs(stream: Observable<LogEntry>): Observable<LogResponse> {
    // 실시간 로그 처리
  }
}
```

## 🔧 **개발 단계별 체크리스트**

### 1단계: 환경 설정

- [ ] `protoc` 설치 확인: `brew install protobuf`
- [ ] Go gRPC 플러그인: `go install google.golang.org/protobuf/cmd/protoc-gen-go@latest`
- [ ] Protocol Buffer 코드 생성: `make proto`
- [ ] 생성된 코드 확인: `pkg/proto/v1/` 디렉토리

### 2단계: 간단한 예제 작성

```bash
# 테스트용 gRPC 서버 생성
mkdir examples/grpc-test
cd examples/grpc-test
```

```go
// 기본 gRPC 서버 예제
func main() {
    lis, err := net.Listen("tcp", ":50051")
    if err != nil {
        log.Fatal(err)
    }

    s := grpc.NewServer()
    pb.RegisterLogStreamingServiceServer(s, &logServer{})

    log.Println("gRPC server listening on :50051")
    if err := s.Serve(lis); err != nil {
        log.Fatal(err)
    }
}
```

### 3단계: 스트리밍 구현

```go
// 양방향 스트리밍 구현
func (s *logServer) StreamLogs(stream pb.LogStreamingService_StreamLogsServer) error {
    for {
        // 클라이언트에서 로그 받기
        logEntry, err := stream.Recv()
        if err == io.EOF {
            return nil
        }
        if err != nil {
            return err
        }

        // 로그 처리
        log.Printf("Received log: %s", logEntry.Message)

        // 응답 전송
        response := &pb.LogResponse{
            Status: pb.LogResponse_ACK,
            Message: "Log received successfully",
        }

        if err := stream.Send(response); err != nil {
            return err
        }
    }
}
```

### 4단계: Worker Pod 통합

```go
// Worker Pod에서 gRPC 클라이언트 사용
func (w *Worker) startLogStreaming() error {
    // 1. gRPC 연결 설정
    conn, err := grpc.Dial("nestjs-server:50051", grpc.WithInsecure())
    if err != nil {
        return err
    }
    defer conn.Close()

    client := pb.NewLogStreamingServiceClient(conn)

    // 2. 스트림 시작
    stream, err := client.StreamLogs(context.Background())
    if err != nil {
        return err
    }

    // 3. 로그 전송 루프
    for logLine := range w.logChan {
        logEntry := &pb.LogEntry{
            WorkerId:  w.ID,
            TaskId:    w.TaskID,
            Timestamp: time.Now().Format(time.RFC3339),
            Level:     "INFO",
            Source:    "stdout",
            Message:   logLine,
        }

        if err := stream.Send(logEntry); err != nil {
            log.Printf("Failed to send log: %v", err)
        }

        // 응답 받기
        response, err := stream.Recv()
        if err != nil {
            log.Printf("Failed to receive response: %v", err)
            continue
        }

        // 응답에 따른 처리
        switch response.Status {
        case pb.LogResponse_ACK:
            // 성공, 다음 로그 전송
        case pb.LogResponse_RETRY:
            // 재시도 로직
            time.Sleep(1 * time.Second)
        case pb.LogResponse_DROP:
            // 로그 포기
            log.Printf("Log dropped: %s", response.Message)
        }
    }

    return nil
}
```

## 🚀 **실제 개발 순서**

### 1. Protocol Buffer 코드 생성부터 시작

```bash
# protoc가 설치되어 있는지 확인
protoc --version

# Protocol Buffer 코드 생성
make proto

# 생성된 파일 확인
ls -la pkg/proto/v1/
```

### 2. 간단한 테스트 서버 만들기

```bash
# 테스트 환경 디렉토리 생성
mkdir -p examples/grpc-hello
cd examples/grpc-hello
```

### 3. 단계적 기능 추가

1. **Unary RPC** → 단순 요청/응답부터
2. **Server Streaming** → 서버에서 다중 응답
3. **Client Streaming** → 클라이언트에서 다중 전송
4. **Bidirectional Streaming** → 양방향 스트리밍

### 4. Worker Pod 통합 전 로컬 테스트

```bash
# 터미널 1: gRPC 서버 실행
go run examples/grpc-hello/server.go

# 터미널 2: gRPC 클라이언트 테스트
go run examples/grpc-hello/client.go
```

## ⚠️ **주의사항 및 팁**

### 개발 시 주의할 점

1. **연결 관리**: gRPC 연결은 장시간 유지되므로 적절한 타임아웃 설정
2. **에러 처리**: 네트워크 단절, 서버 재시작 등에 대한 재연결 로직
3. **백프레셔**: 로그 생성 속도 > 전송 속도일 때 버퍼링 전략
4. **성능**: 대용량 로그 전송 시 배치 처리 고려

### 디버깅 팁

```bash
# gRPC 연결 상태 확인
grpcurl -plaintext localhost:50051 list

# gRPC 메서드 테스트
grpcurl -plaintext -d '{"worker_id":"test"}' \
  localhost:50051 ottoscaler.v1.LogStreamingService/RegisterWorker
```

### 성능 최적화

1. **Connection Pooling**: 다중 Worker에서 연결 재사용
2. **Compression**: gRPC 압축 활성화
3. **Keep-Alive**: 연결 유지 설정
4. **Load Balancing**: 다중 NestJS 서버 환경 대비

## 📖 **추가 학습 자료**

### 공식 문서

- [gRPC Go Tutorial](https://grpc.io/docs/languages/go/quickstart/)
- [Protocol Buffers Guide](https://developers.google.com/protocol-buffers)

### 실습 예제

- [gRPC Go Examples](https://github.com/grpc/grpc-go/tree/master/examples)
- [Streaming Examples](https://github.com/grpc/grpc-go/blob/master/examples/route_guide)

이 가이드를 따라하면서 단계적으로 gRPC 시스템을 구축해 나가시면 됩니다! 🚀
