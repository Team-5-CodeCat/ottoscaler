// Package grpc provides log streaming implementation for gRPC server.
//
// This file implements the LogStreamingService which receives real-time logs
// from worker pods and streams them to otto-handler via gRPC.
package grpc

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Team-5-CodeCat/ottoscaler/internal/k8s"
	pb "github.com/Team-5-CodeCat/ottoscaler/pkg/proto/v1"
)

// LogStreamingServer implements the LogStreamingService gRPC server.
//
// LogStreamingServer는 Worker Pod들의 로그를 수집하고
// Otto-handler로 실시간 스트리밍하는 gRPC 서버입니다.
type LogStreamingServer struct {
	pb.UnimplementedLogStreamingServiceServer

	k8sClient         *k8s.Client
	ottoHandlerClient *OttoHandlerClient

	// Active streaming sessions
	sessions map[string]*StreamingSession
	mu       sync.RWMutex

	// Configuration
	maxSessionsPerWorker int
	streamTimeout        time.Duration
	maxRetries           int
	retryDelay           time.Duration
}

// StreamingSession represents an active log streaming session
//
// StreamingSession은 활성 로그 스트리밍 세션을 나타냅니다.
type StreamingSession struct {
	SessionID  string
	WorkerID   string
	TaskID     string
	CreatedAt  time.Time
	LastActive time.Time
	LogCount   int64
	ErrorCount int64
	IsActive   bool

	// Connection management
	currentConnections int
	maxConnections     int
	retryCount         int

	// Streaming channels
	logStream   chan *pb.LogEntry
	errorStream chan error
	stopChan    chan struct{}
	mu          sync.RWMutex
}

// NewLogStreamingServer creates a new log streaming server instance.
//
// NewLogStreamingServer는 새로운 로그 스트리밍 서버 인스턴스를 생성합니다.
func NewLogStreamingServer(k8sClient *k8s.Client, ottoHandlerAddress string, mockMode bool) *LogStreamingServer {
	// Create Otto-handler client
	ottoHandlerClient := NewOttoHandlerClient(ottoHandlerAddress, mockMode)

	// Connect to Otto-handler if not in mock mode
	if !mockMode || mockMode { // Always try to connect for now
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := ottoHandlerClient.Connect(ctx); err != nil {
			log.Printf("⚠️ Otto-handler 연결 실패: %v", err)
			log.Printf("📡 나중에 연결을 다시 시도합니다...")
		}
	}

	return &LogStreamingServer{
		k8sClient:            k8sClient,
		ottoHandlerClient:    ottoHandlerClient,
		sessions:             make(map[string]*StreamingSession),
		maxSessionsPerWorker: 5,                // 한 Worker당 최대 5개 동시 스트림
		streamTimeout:        30 * time.Minute, // 30분 세션 타임아웃
		maxRetries:           3,                // 최대 3회 재시도
		retryDelay:           5 * time.Second,  // 5초 재시도 지연
	}
}

// RegisterWorker handles worker registration requests.
//
// RegisterWorker는 Worker Pod 등록 요청을 처리합니다.
// Worker Pod가 시작될 때 호출되어 로그 스트리밍 세션을 설정합니다.
func (s *LogStreamingServer) RegisterWorker(ctx context.Context, req *pb.WorkerRegistration) (*pb.RegistrationResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	if req.WorkerId == "" {
		return nil, status.Error(codes.InvalidArgument, "worker_id is required")
	}

	if req.TaskId == "" {
		return nil, status.Error(codes.InvalidArgument, "task_id is required")
	}

	log.Printf("📋 Worker 등록 요청: worker_id=%s, task_id=%s", req.WorkerId, req.TaskId)

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if worker is already registered
	for _, session := range s.sessions {
		if session.WorkerID == req.WorkerId && session.IsActive {
			log.Printf("⚠️ Worker %s가 이미 등록됨", req.WorkerId)
			return &pb.RegistrationResponse{
				Status:    pb.RegistrationResponse_ALREADY_REGISTERED,
				Message:   fmt.Sprintf("Worker %s is already registered", req.WorkerId),
				SessionId: session.SessionID,
				Config:    s.getDefaultLoggingConfig(),
			}, nil
		}
	}

	// Check session limits
	activeSessionsCount := s.countActiveSessionsForWorker(req.WorkerId)
	if activeSessionsCount >= s.maxSessionsPerWorker {
		log.Printf("⚠️ Worker %s의 세션이 너무 많음: %d개", req.WorkerId, activeSessionsCount)
		return &pb.RegistrationResponse{
			Status:  pb.RegistrationResponse_SERVER_FULL,
			Message: fmt.Sprintf("Maximum sessions reached for worker %s", req.WorkerId),
		}, nil
	}

	// Create new session
	sessionID := fmt.Sprintf("%s-%d", req.WorkerId, time.Now().UnixNano())
	session := &StreamingSession{
		SessionID:          sessionID,
		WorkerID:           req.WorkerId,
		TaskID:             req.TaskId,
		CreatedAt:          time.Now(),
		LastActive:         time.Now(),
		IsActive:           true,
		currentConnections: 0,
		maxConnections:     3, // 최대 3개 동시 연결
		retryCount:         0,
		logStream:          make(chan *pb.LogEntry, 1000),
		errorStream:        make(chan error, 10),
		stopChan:           make(chan struct{}),
	}

	s.sessions[sessionID] = session

	log.Printf("✅ Worker 등록 완료: worker_id=%s, session_id=%s", req.WorkerId, sessionID)

	return &pb.RegistrationResponse{
		Status:    pb.RegistrationResponse_SUCCESS,
		Message:   fmt.Sprintf("Worker %s registered successfully", req.WorkerId),
		SessionId: sessionID,
		Config:    s.getDefaultLoggingConfig(),
	}, nil
}

// StreamLogs handles bidirectional log streaming.
//
// StreamLogs는 양방향 로그 스트리밍을 처리합니다.
// Worker Pod에서 로그를 받아 Otto-handler로 전달하고
// 처리 결과를 다시 Worker Pod로 전송합니다.
func (s *LogStreamingServer) StreamLogs(stream pb.LogStreamingService_StreamLogsServer) error {
	log.Printf("📡 새 로그 스트리밍 연결 설정됨")

	ctx := stream.Context()
	var currentSession *StreamingSession

	// Response goroutine for sending responses back to client
	responseChan := make(chan *pb.LogResponse, 100)
	defer close(responseChan)

	go func() {
		for {
			select {
			case response, ok := <-responseChan:
				if !ok {
					return
				}
				if err := stream.Send(response); err != nil {
					log.Printf("❌ 로그 응답 전송 실패: %v", err)
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Main streaming loop
	for {
		select {
		case <-ctx.Done():
			log.Printf("📡 로그 스트리밍 연결 종료: %v", ctx.Err())
			if currentSession != nil {
				s.deactivateSession(currentSession.SessionID)
			}
			return ctx.Err()
		default:
			// Receive log entry from client
			logEntry, err := stream.Recv()
			if err == io.EOF {
				log.Printf("📡 클라이언트가 로그 스트림을 닫음")
				if currentSession != nil {
					s.deactivateSession(currentSession.SessionID)
				}
				return nil
			}
			if err != nil {
				log.Printf("❌ 로그 엔트리 수신 오류: %v", err)
				if currentSession != nil {
					s.deactivateSession(currentSession.SessionID)
				}
				return err
			}

			// Validate log entry
			if logEntry == nil {
				response := &pb.LogResponse{
					Status:  pb.LogResponse_DROP,
					Message: "log entry cannot be nil",
				}
				select {
				case responseChan <- response:
				case <-ctx.Done():
					return ctx.Err()
				}
				continue
			}

			// Find or validate session
			if currentSession == nil {
				session := s.findSessionByWorker(logEntry.WorkerId)
				if session == nil {
					log.Printf("⚠️ Worker %s의 세션을 찾을 수 없음", logEntry.WorkerId)
					response := &pb.LogResponse{
						Status:  pb.LogResponse_DROP,
						Message: fmt.Sprintf("Worker %s not registered", logEntry.WorkerId),
					}
					select {
					case responseChan <- response:
					case <-ctx.Done():
						return ctx.Err()
					}
					continue
				}
				currentSession = session
				log.Printf("📡 세션과 스트림 연결: %s", session.SessionID)
			}

			// Process log entry
			if err := s.processLogEntry(ctx, logEntry, currentSession); err != nil {
				log.Printf("❌ 로그 엔트리 처리 오류: %v", err)
				response := &pb.LogResponse{
					Status:  pb.LogResponse_RETRY,
					Message: fmt.Sprintf("Processing error: %v", err),
				}
				select {
				case responseChan <- response:
				case <-ctx.Done():
					return ctx.Err()
				}
				continue
			}

			// Send success response
			currentSession.LogCount++
			response := &pb.LogResponse{
				Status:   pb.LogResponse_ACK,
				Message:  "Log received successfully",
				Sequence: currentSession.LogCount,
			}
			select {
			case responseChan <- response:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}

// processLogEntry processes a single log entry
//
// processLogEntry는 단일 로그 엔트리를 처리합니다.
func (s *LogStreamingServer) processLogEntry(ctx context.Context, logEntry *pb.LogEntry, session *StreamingSession) error {
	// Update session activity
	session.mu.Lock()
	session.LastActive = time.Now()
	session.mu.Unlock()

	// Validate log entry fields
	if logEntry.WorkerId == "" {
		session.mu.Lock()
		session.ErrorCount++
		session.mu.Unlock()
		return fmt.Errorf("worker_id is required")
	}
	if logEntry.TaskId == "" {
		session.mu.Lock()
		session.ErrorCount++
		session.mu.Unlock()
		return fmt.Errorf("task_id is required")
	}
	if logEntry.Message == "" {
		session.mu.Lock()
		session.ErrorCount++
		session.mu.Unlock()
		return fmt.Errorf("message is required")
	}

	// Add timestamp if missing
	if logEntry.Timestamp == "" {
		logEntry.Timestamp = time.Now().Format(time.RFC3339)
	}

	// Set default level if missing
	if logEntry.Level == "" {
		logEntry.Level = "INFO"
	}

	// Set default source if missing
	if logEntry.Source == "" {
		logEntry.Source = "stdout"
	}

	// Attempt to forward to Otto-handler with retry logic
	if err := s.forwardLogEntryWithRetry(ctx, logEntry, session); err != nil {
		session.mu.Lock()
		session.ErrorCount++
		session.mu.Unlock()
		log.Printf("⚠️ 재시도 후에도 로그 엔트리 전달 실패: %v", err)
		return err
	}

	return nil
}

// forwardLogEntryWithRetry forwards log entry with retry logic
//
// forwardLogEntryWithRetry는 재시도 로직과 함께 로그 엔트리를 전달합니다.
func (s *LogStreamingServer) forwardLogEntryWithRetry(ctx context.Context, logEntry *pb.LogEntry, session *StreamingSession) error {
	var lastErr error

	for attempt := 0; attempt <= s.maxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry
			select {
			case <-time.After(s.retryDelay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		// Attempt to forward the log entry
		if err := s.forwardLogEntry(ctx, logEntry, session); err != nil {
			lastErr = err
			log.Printf("⚠️ 로그 전달 %d차 시도 실패: %v", attempt+1, err)
			continue
		}

		// Success
		if attempt > 0 {
			log.Printf("✅ 로그 전달 %d차 시도에서 성공", attempt+1)
		}
		return nil
	}

	return fmt.Errorf("log forwarding failed after %d attempts: %w", s.maxRetries+1, lastErr)
}

// forwardLogEntry forwards a single log entry to Otto-handler.
//
// forwardLogEntry는 단일 로그 엔트리를 Otto-handler로 전달합니다.
func (s *LogStreamingServer) forwardLogEntry(ctx context.Context, logEntry *pb.LogEntry, session *StreamingSession) error {
	// Convert LogEntry to WorkerLogEntry for Otto-handler
	workerLogEntry := &pb.WorkerLogEntry{
		WorkerId:  logEntry.WorkerId,
		TaskId:    logEntry.TaskId,
		Timestamp: logEntry.Timestamp,
		Level:     logEntry.Level,
		Source:    logEntry.Source,
		Message:   logEntry.Message,
		PodMetadata: &pb.WorkerMetadata{
			PodName:   logEntry.WorkerId,
			Namespace: "default", // TODO: Get from actual context
			CreatedAt: session.CreatedAt.Format(time.RFC3339),
		},
		Metadata: logEntry.Metadata,
	}

	// Ensure log stream is started for this worker
	if s.ottoHandlerClient.GetActiveStreamCount() == 0 ||
		!s.isStreamActive(logEntry.WorkerId) {
		// Start new log stream
		if err := s.ottoHandlerClient.StartLogStream(ctx, logEntry.WorkerId, logEntry.TaskId); err != nil {
			log.Printf("⚠️ Worker %s의 로그 스트림 시작 실패: %v", logEntry.WorkerId, err)
			return err
		}
		log.Printf("📡 Worker %s의 Otto-handler 로그 스트림 시작", logEntry.WorkerId)
	}

	// Forward the log entry
	if err := s.ottoHandlerClient.ForwardLogEntry(ctx, workerLogEntry); err != nil {
		log.Printf("⚠️ Otto-handler로 로그 전달 실패: %v", err)
		return err
	}

	return nil
}

// isStreamActive checks if a log stream is active for a worker.
//
// isStreamActive는 Worker의 로그 스트림이 활성 상태인지 확인합니다.
func (s *LogStreamingServer) isStreamActive(workerID string) bool {
	_, _, exists := s.ottoHandlerClient.GetStreamStats(workerID)
	return exists
}

// getDefaultLoggingConfig returns default logging configuration
//
// getDefaultLoggingConfig는 기본 로깅 설정을 반환합니다.
func (s *LogStreamingServer) getDefaultLoggingConfig() *pb.LoggingConfig {
	return &pb.LoggingConfig{
		RateLimit:       100,  // 초당 최대 100개 로그
		BufferSize:      50,   // 50개 로그 버퍼링
		MaxMessageSize:  1024, // 1KB 최대 메시지 크기
		IncludeMetadata: true, // 메타데이터 포함
	}
}

// countActiveSessionsForWorker counts active sessions for a worker
//
// countActiveSessionsForWorker는 특정 Worker의 활성 세션 수를 계산합니다.
func (s *LogStreamingServer) countActiveSessionsForWorker(workerID string) int {
	count := 0
	for _, session := range s.sessions {
		if session.WorkerID == workerID && session.IsActive {
			count++
		}
	}
	return count
}

// findSessionByWorker finds an active session for a worker
//
// findSessionByWorker는 특정 Worker의 활성 세션을 찾습니다.
func (s *LogStreamingServer) findSessionByWorker(workerID string) *StreamingSession {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, session := range s.sessions {
		if session.WorkerID == workerID && session.IsActive {
			return session
		}
	}
	return nil
}

// deactivateSession deactivates a streaming session
//
// deactivateSession은 스트리밍 세션을 비활성화합니다.
func (s *LogStreamingServer) deactivateSession(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if session, exists := s.sessions[sessionID]; exists {
		session.IsActive = false
		close(session.stopChan)
		log.Printf("🔌 세션 비활성화: %s (처리된 로그: %d개)", sessionID, session.LogCount)
	}
}

// CleanupInactiveSessions removes inactive sessions periodically
//
// CleanupInactiveSessions는 비활성 세션을 주기적으로 정리합니다.
func (s *LogStreamingServer) CleanupInactiveSessions() {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-s.streamTimeout)
	cleaned := 0

	for sessionID, session := range s.sessions {
		if !session.IsActive || session.CreatedAt.Before(cutoff) {
			delete(s.sessions, sessionID)
			cleaned++
		}
	}

	if cleaned > 0 {
		log.Printf("🧹 비활성 세션 %d개 정리됨", cleaned)
	}
}

// GetActiveSessionsCount returns the number of active sessions
//
// GetActiveSessionsCount는 활성 세션 수를 반환합니다.
func (s *LogStreamingServer) GetActiveSessionsCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, session := range s.sessions {
		if session.IsActive {
			count++
		}
	}
	return count
}
