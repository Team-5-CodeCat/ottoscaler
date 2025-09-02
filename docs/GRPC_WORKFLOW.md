# gRPC 구현 워크플로우 - 일반적인 개발 프로세스

## 🔄 **일반적인 gRPC 개발 워크플로우**

### 1단계: 요구사항 분석 및 설계
```
📋 비즈니스 요구사항 → 📐 API 설계 → 🔧 기술 스택 선택
```

#### 핵심 질문들
- **무엇을 전송**하는가? (데이터 구조)
- **언제 전송**하는가? (트리거 조건)
- **어떻게 전송**하는가? (단방향/양방향/스트리밍)
- **얼마나 빠르게** 전송해야 하는가? (지연시간 요구사항)
- **얼마나 많은 데이터**를 처리하는가? (처리량 요구사항)

### 2단계: Protocol Buffer 스키마 정의
```proto
// 1. 서비스 정의
service MyService {
    rpc UnaryCall(Request) returns (Response);
    rpc ServerStream(Request) returns (stream Response);
    rpc ClientStream(stream Request) returns (Response);
    rpc BidirectionalStream(stream Request) returns (stream Response);
}

// 2. 메시지 구조 정의
message Request {
    string id = 1;
    int32 value = 2;
}

message Response {
    bool success = 1;
    string message = 2;
}
```

### 3단계: 코드 생성 및 환경 설정
```bash
# Protocol Buffer 컴파일러 설치
# macOS
brew install protobuf

# 언어별 플러그인 설치
# Go
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# TypeScript/JavaScript
npm install -g protoc-gen-ts

# 코드 생성
protoc --go_out=. --go-grpc_out=. *.proto
```

### 4단계: 서버 구현
```go
// gRPC 서버 구현 패턴
type server struct {
    pb.UnimplementedMyServiceServer
}

func (s *server) UnaryCall(ctx context.Context, req *pb.Request) (*pb.Response, error) {
    // 비즈니스 로직 구현
    return &pb.Response{
        Success: true,
        Message: "Processed: " + req.Id,
    }, nil
}

func main() {
    lis, err := net.Listen("tcp", ":50051")
    if err != nil {
        log.Fatal(err)
    }
    
    s := grpc.NewServer()
    pb.RegisterMyServiceServer(s, &server{})
    
    log.Println("gRPC server listening on :50051")
    s.Serve(lis)
}
```

### 5단계: 클라이언트 구현
```go
// gRPC 클라이언트 구현 패턴
func main() {
    conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
    if err != nil {
        log.Fatal(err)
    }
    defer conn.Close()
    
    client := pb.NewMyServiceClient(conn)
    
    // 단방향 호출
    response, err := client.UnaryCall(context.Background(), &pb.Request{
        Id:    "test-123",
        Value: 42,
    })
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("Response: %s", response.Message)
}
```

### 6단계: 테스트 및 검증
```go
// 단위 테스트
func TestUnaryCall(t *testing.T) {
    s := &server{}
    
    req := &pb.Request{Id: "test", Value: 123}
    resp, err := s.UnaryCall(context.Background(), req)
    
    assert.NoError(t, err)
    assert.True(t, resp.Success)
    assert.Equal(t, "Processed: test", resp.Message)
}

// 통합 테스트 - 실제 네트워크 호출
func TestIntegration(t *testing.T) {
    // gRPC 서버 시작
    // 클라이언트로 실제 호출
    // 결과 검증
}
```

### 7단계: 배포 및 모니터링
```yaml
# Kubernetes 배포
apiVersion: apps/v1
kind: Deployment
metadata:
  name: grpc-server
spec:
  replicas: 3
  selector:
    matchLabels:
      app: grpc-server
  template:
    spec:
      containers:
      - name: grpc-server
        image: myapp/grpc-server:latest
        ports:
        - containerPort: 50051
          name: grpc
---
apiVersion: v1
kind: Service
metadata:
  name: grpc-service
spec:
  selector:
    app: grpc-server
  ports:
  - port: 50051
    targetPort: 50051
    name: grpc
```

## 🎯 **각 RPC 타입별 구현 패턴**

### 1. Unary RPC (단방향 호출)
**사용 케이스**: 사용자 인증, 설정 조회, 단순 CRUD

```go
// 서버
func (s *server) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.User, error) {
    user := s.userService.GetByID(req.UserId)
    return &pb.User{
        Id:    user.ID,
        Name:  user.Name,
        Email: user.Email,
    }, nil
}

// 클라이언트
user, err := client.GetUser(ctx, &pb.GetUserRequest{UserId: "123"})
```

### 2. Server Streaming (서버 스트리밍)
**사용 케이스**: 실시간 알림, 대용량 데이터 조회, 진행률 업데이트

```go
// 서버
func (s *server) StreamNotifications(req *pb.StreamRequest, stream pb.MyService_StreamNotificationsServer) error {
    for {
        notification := s.getNextNotification(req.UserId)
        if notification == nil {
            break
        }
        
        if err := stream.Send(notification); err != nil {
            return err
        }
        
        time.Sleep(1 * time.Second)
    }
    return nil
}

// 클라이언트
stream, err := client.StreamNotifications(ctx, &pb.StreamRequest{UserId: "123"})
for {
    notification, err := stream.Recv()
    if err == io.EOF {
        break
    }
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("Received: %s", notification.Message)
}
```

### 3. Client Streaming (클라이언트 스트리밍)
**사용 케이스**: 파일 업로드, 배치 데이터 전송, 실시간 메트릭 수집

```go
// 서버
func (s *server) UploadFile(stream pb.MyService_UploadFileServer) error {
    var fileData []byte
    
    for {
        chunk, err := stream.Recv()
        if err == io.EOF {
            // 파일 업로드 완료
            result := s.saveFile(fileData)
            return stream.SendAndClose(&pb.UploadResponse{
                Success: true,
                FileId:  result.ID,
            })
        }
        if err != nil {
            return err
        }
        
        fileData = append(fileData, chunk.Data...)
    }
}

// 클라이언트
stream, err := client.UploadFile(ctx)
file, _ := os.Open("large-file.zip")
buffer := make([]byte, 1024)

for {
    n, err := file.Read(buffer)
    if err == io.EOF {
        break
    }
    
    stream.Send(&pb.FileChunk{Data: buffer[:n]})
}

response, err := stream.CloseAndRecv()
```

### 4. Bidirectional Streaming (양방향 스트리밍)
**사용 케이스**: 채팅, 실시간 게임, 로그 스트리밍

```go
// 서버
func (s *server) Chat(stream pb.MyService_ChatServer) error {
    go func() {
        // 메시지 수신 고루틴
        for {
            msg, err := stream.Recv()
            if err != nil {
                return
            }
            // 메시지 처리 및 브로드캐스트
            s.broadcastMessage(msg)
        }
    }()
    
    // 메시지 전송 고루틴
    for msg := range s.messageChan {
        if err := stream.Send(msg); err != nil {
            return err
        }
    }
    
    return nil
}

// 클라이언트
stream, err := client.Chat(ctx)

go func() {
    // 메시지 송신 고루틴
    for {
        var input string
        fmt.Scanln(&input)
        stream.Send(&pb.ChatMessage{
            User:    "user123",
            Content: input,
        })
    }
}()

// 메시지 수신 고루틴
for {
    msg, err := stream.Recv()
    if err != nil {
        break
    }
    fmt.Printf("[%s]: %s\n", msg.User, msg.Content)
}
```

## 🛡️ **에러 처리 및 모범 사례**

### 에러 처리 패턴
```go
import "google.golang.org/grpc/codes"
import "google.golang.org/grpc/status"

// 서버에서 에러 반환
func (s *server) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.User, error) {
    if req.UserId == "" {
        return nil, status.Error(codes.InvalidArgument, "user_id is required")
    }
    
    user, err := s.userService.GetByID(req.UserId)
    if err != nil {
        if errors.Is(err, ErrUserNotFound) {
            return nil, status.Error(codes.NotFound, "user not found")
        }
        return nil, status.Error(codes.Internal, "internal error")
    }
    
    return user, nil
}

// 클라이언트에서 에러 처리
user, err := client.GetUser(ctx, req)
if err != nil {
    st, ok := status.FromError(err)
    if ok {
        switch st.Code() {
        case codes.InvalidArgument:
            log.Println("Invalid request")
        case codes.NotFound:
            log.Println("User not found")
        case codes.Internal:
            log.Println("Server error")
        }
    }
}
```

### 연결 관리
```go
// Connection Pool 사용
type ClientPool struct {
    conns []*grpc.ClientConn
    mu    sync.RWMutex
}

func (p *ClientPool) GetConnection() *grpc.ClientConn {
    p.mu.RLock()
    defer p.mu.RUnlock()
    
    // Round-robin 방식으로 연결 선택
    return p.conns[rand.Intn(len(p.conns))]
}

// 재연결 로직
func (c *Client) ensureConnection() error {
    if c.conn.GetState() == connectivity.Ready {
        return nil
    }
    
    // 재연결 시도
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    return c.conn.Connect(ctx)
}
```

### 모니터링 및 로깅
```go
import "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"

// OpenTelemetry 통합
s := grpc.NewServer(
    grpc.UnaryInterceptor(otelgrpc.UnaryServerInterceptor()),
    grpc.StreamInterceptor(otelgrpc.StreamServerInterceptor()),
)

// 커스텀 인터셉터
func loggingInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
    start := time.Now()
    
    log.Printf("gRPC call: %s started", info.FullMethod)
    resp, err := handler(ctx, req)
    
    duration := time.Since(start)
    log.Printf("gRPC call: %s completed in %v", info.FullMethod, duration)
    
    return resp, err
}
```

이런 패턴들을 이해하고 적용하면 안정적이고 확장 가능한 gRPC 시스템을 구축할 수 있습니다! 🚀