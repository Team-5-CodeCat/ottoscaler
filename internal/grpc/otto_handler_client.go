// Package grpc provides Otto-handler gRPC client implementation.
//
// Otto-handler gRPC 클라이언트를 구현합니다.
// Mock과 실제 구현을 모두 지원하여 개발과 테스트를 용이하게 합니다.
package grpc

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	pb "github.com/Team-5-CodeCat/ottoscaler/pkg/proto/v1"
)

// OttoHandlerClient manages connection to Otto-handler for log forwarding.
//
// OttoHandlerClient는 Otto-handler로 로그를 전달하기 위한 gRPC 클라이언트입니다.
type OttoHandlerClient struct {
	// Connection management
	address     string
	conn        *grpc.ClientConn
	client      pb.OttoHandlerLogServiceClient
	mockMode    bool
	isConnected bool
	mu          sync.RWMutex

	// Streaming management
	activeStreams map[string]*LogStream
	streamMu      sync.RWMutex

	// Configuration
	maxRetries     int
	retryDelay     time.Duration
	connectTimeout time.Duration
	streamTimeout  time.Duration
}

// LogStream represents an active log streaming session.
//
// LogStream은 활성 로그 스트리밍 세션을 나타냅니다.
type LogStream struct {
	TaskID     string
	WorkerID   string
	Stream     pb.OttoHandlerLogService_ForwardWorkerLogsClient
	Context    context.Context
	Cancel     context.CancelFunc
	LogCount   int64
	ErrorCount int64
	CreatedAt  time.Time
	LastActive time.Time
}

// NewOttoHandlerClient creates a new Otto-handler gRPC client.
//
// NewOttoHandlerClient는 새로운 Otto-handler gRPC 클라이언트를 생성합니다.
func NewOttoHandlerClient(address string, mockMode bool) *OttoHandlerClient {
	return &OttoHandlerClient{
		address:        address,
		mockMode:       mockMode,
		activeStreams:  make(map[string]*LogStream),
		maxRetries:     3,
		retryDelay:     5 * time.Second,
		connectTimeout: 10 * time.Second,
		streamTimeout:  30 * time.Minute,
	}
}

// Connect establishes connection to Otto-handler.
//
// Connect는 Otto-handler와의 연결을 설정합니다.
func (c *OttoHandlerClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isConnected {
		return nil
	}

	if c.mockMode {
		log.Printf("🔌 [MOCK] Otto-handler 연결 중: %s", c.address)
		c.isConnected = true
		return nil
	}

	// Real connection
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()), // TODO: Add TLS in production
	}

	conn, err := grpc.NewClient(c.address, opts...)
	if err != nil {
		return fmt.Errorf("failed to connect to Otto-handler at %s: %w", c.address, err)
	}

	c.conn = conn
	c.client = pb.NewOttoHandlerLogServiceClient(conn)
	c.isConnected = true

	log.Printf("✅ Otto-handler 연결 성공: %s", c.address)
	return nil
}

// Disconnect closes the connection to Otto-handler.
//
// Disconnect는 Otto-handler와의 연결을 종료합니다.
func (c *OttoHandlerClient) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.isConnected {
		return nil
	}

	// Close all active streams
	c.closeAllStreams()

	if c.mockMode {
		log.Printf("🔌 [MOCK] Otto-handler 연결 해제")
		c.isConnected = false
		return nil
	}

	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			return fmt.Errorf("failed to close connection: %w", err)
		}
	}

	c.isConnected = false
	log.Printf("🔌 Otto-handler 연결 해제")
	return nil
}

// StartLogStream starts a new log streaming session for a worker.
//
// StartLogStream은 Worker를 위한 새로운 로그 스트리밍 세션을 시작합니다.
func (c *OttoHandlerClient) StartLogStream(ctx context.Context, workerID string, taskID string) error {
	c.streamMu.Lock()
	defer c.streamMu.Unlock()

	// Check if stream already exists
	if _, exists := c.activeStreams[workerID]; exists {
		return fmt.Errorf("log stream already exists for worker %s", workerID)
	}

	// Create stream context with timeout
	streamCtx, cancel := context.WithTimeout(ctx, c.streamTimeout)

	if c.mockMode {
		// Mock stream
		log.Printf("📡 [MOCK] Worker %s의 로그 스트림 시작 (task: %s)", workerID, taskID)

		stream := &LogStream{
			TaskID:     taskID,
			WorkerID:   workerID,
			Context:    streamCtx,
			Cancel:     cancel,
			CreatedAt:  time.Now(),
			LastActive: time.Now(),
		}

		c.activeStreams[workerID] = stream
		return nil
	}

	// Real stream
	stream, err := c.client.ForwardWorkerLogs(streamCtx)
	if err != nil {
		cancel()
		return fmt.Errorf("failed to create log stream: %w", err)
	}

	logStream := &LogStream{
		TaskID:     taskID,
		WorkerID:   workerID,
		Stream:     stream,
		Context:    streamCtx,
		Cancel:     cancel,
		CreatedAt:  time.Now(),
		LastActive: time.Now(),
	}

	c.activeStreams[workerID] = logStream

	// Start response handler
	go c.handleStreamResponses(logStream)

	log.Printf("📡 Worker %s의 로그 스트림 시작 (task: %s)", workerID, taskID)
	return nil
}

// ForwardLogEntry forwards a single log entry to Otto-handler.
//
// ForwardLogEntry는 단일 로그 엔트리를 Otto-handler로 전달합니다.
func (c *OttoHandlerClient) ForwardLogEntry(ctx context.Context, entry *pb.WorkerLogEntry) error {
	c.streamMu.RLock()
	stream, exists := c.activeStreams[entry.WorkerId]
	c.streamMu.RUnlock()

	if !exists {
		return fmt.Errorf("no active stream for worker %s", entry.WorkerId)
	}

	// Update activity
	stream.LastActive = time.Now()
	stream.LogCount++

	if c.mockMode {
		// Mock forwarding with simulated delay
		select {
		case <-time.After(10 * time.Millisecond):
			log.Printf("📤 [MOCK] 로그 전달 [%s|%s] %s: %s",
				entry.WorkerId, entry.TaskId, entry.Level, entry.Message)
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// Real forwarding
	if err := stream.Stream.Send(entry); err != nil {
		stream.ErrorCount++
		return fmt.Errorf("failed to send log entry: %w", err)
	}

	return nil
}

// handleStreamResponses handles responses from Otto-handler.
//
// handleStreamResponses는 Otto-handler로부터의 응답을 처리합니다.
func (c *OttoHandlerClient) handleStreamResponses(stream *LogStream) {
	defer func() {
		c.streamMu.Lock()
		delete(c.activeStreams, stream.WorkerID)
		c.streamMu.Unlock()
		stream.Cancel()
		log.Printf("📡 Worker %s의 로그 스트림 종료", stream.WorkerID)
	}()

	for {
		select {
		case <-stream.Context.Done():
			return
		default:
			resp, err := stream.Stream.Recv()
			if err == io.EOF {
				log.Printf("📡 Otto-handler가 Worker %s의 스트림을 종료함", stream.WorkerID)
				return
			}
			if err != nil {
				if status.Code(err) != codes.Canceled {
					log.Printf("❌ Worker %s의 응답 수신 오류: %v", stream.WorkerID, err)
				}
				return
			}

			// Process response
			c.processResponse(stream, resp)
		}
	}
}

// processResponse processes a response from Otto-handler.
//
// processResponse는 Otto-handler로부터의 응답을 처리합니다.
func (c *OttoHandlerClient) processResponse(stream *LogStream, resp *pb.LogForwardResponse) {
	switch resp.Status {
	case pb.LogForwardResponse_ACK:
		// Success - no action needed
		if resp.ThrottleMs > 0 {
			// Apply throttling if requested
			time.Sleep(time.Duration(resp.ThrottleMs) * time.Millisecond)
		}

	case pb.LogForwardResponse_RETRY:
		log.Printf("⚠️ Otto-handler가 Worker %s의 재시도를 요청: %s", stream.WorkerID, resp.Message)
		stream.ErrorCount++

	case pb.LogForwardResponse_DROP:
		log.Printf("❌ Otto-handler가 Worker %s의 로그를 폐기: %s", stream.WorkerID, resp.Message)
		stream.ErrorCount++
	}
}

// NotifyWorkerStatus notifies Otto-handler about worker status changes.
//
// NotifyWorkerStatus는 Worker 상태 변경을 Otto-handler에 알립니다.
func (c *OttoHandlerClient) NotifyWorkerStatus(ctx context.Context, notification *pb.WorkerStatusNotification) error {
	if c.mockMode {
		log.Printf("📢 [MOCK] Worker 상태 알림: %s -> %s",
			notification.WorkerId, notification.Status.String())
		return nil
	}

	c.mu.RLock()
	if !c.isConnected {
		c.mu.RUnlock()
		return fmt.Errorf("not connected to Otto-handler")
	}
	client := c.client
	c.mu.RUnlock()

	resp, err := client.NotifyWorkerStatus(ctx, notification)
	if err != nil {
		return fmt.Errorf("failed to notify worker status: %w", err)
	}

	if resp.Status != pb.WorkerStatusAck_RECEIVED {
		log.Printf("⚠️ Worker 상태 알림이 확인되지 않음: %s", resp.Message)
	}

	return nil
}

// CloseLogStream closes the log stream for a specific worker.
//
// CloseLogStream은 특정 Worker의 로그 스트림을 종료합니다.
func (c *OttoHandlerClient) CloseLogStream(workerID string) error {
	c.streamMu.Lock()
	defer c.streamMu.Unlock()

	stream, exists := c.activeStreams[workerID]
	if !exists {
		return nil
	}

	if c.mockMode {
		log.Printf("📡 [MOCK] Worker %s의 로그 스트림 종료 (전송된 로그: %d개, 오류: %d개)",
			workerID, stream.LogCount, stream.ErrorCount)
	} else {
		if stream.Stream != nil {
			if err := stream.Stream.CloseSend(); err != nil {
				log.Printf("⚠️ Worker %s의 스트림 종료 오류: %v", workerID, err)
			}
		}
		log.Printf("📡 Worker %s의 로그 스트림 종료 (전송된 로그: %d개, 오류: %d개)",
			workerID, stream.LogCount, stream.ErrorCount)
	}

	stream.Cancel()
	delete(c.activeStreams, workerID)
	return nil
}

// closeAllStreams closes all active log streams.
//
// closeAllStreams는 모든 활성 로그 스트림을 종료합니다.
func (c *OttoHandlerClient) closeAllStreams() {
	c.streamMu.Lock()
	defer c.streamMu.Unlock()

	for workerID, stream := range c.activeStreams {
		if stream.Stream != nil && !c.mockMode {
			stream.Stream.CloseSend()
		}
		stream.Cancel()
		log.Printf("📡 Worker %s의 로그 스트림 종료", workerID)
	}

	c.activeStreams = make(map[string]*LogStream)
}

// IsConnected returns whether the client is connected.
//
// IsConnected는 클라이언트가 연결되어 있는지 반환합니다.
func (c *OttoHandlerClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.mockMode {
		return c.isConnected
	}

	if !c.isConnected || c.conn == nil {
		return false
	}

	// Check actual connection state
	return c.conn.GetState() == connectivity.Ready
}

// GetActiveStreamCount returns the number of active log streams.
//
// GetActiveStreamCount는 활성 로그 스트림 수를 반환합니다.
func (c *OttoHandlerClient) GetActiveStreamCount() int {
	c.streamMu.RLock()
	defer c.streamMu.RUnlock()
	return len(c.activeStreams)
}

// GetStreamStats returns statistics for a specific worker's stream.
//
// GetStreamStats는 특정 Worker 스트림의 통계를 반환합니다.
func (c *OttoHandlerClient) GetStreamStats(workerID string) (logCount, errorCount int64, exists bool) {
	c.streamMu.RLock()
	defer c.streamMu.RUnlock()

	stream, exists := c.activeStreams[workerID]
	if !exists {
		return 0, 0, false
	}

	return stream.LogCount, stream.ErrorCount, true
}
