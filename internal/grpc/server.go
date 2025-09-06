// Package grpc provides gRPC server implementation for Ottoscaler.
//
// This package implements the OttoscalerService defined in the proto file,
// handling scale up/down requests from otto-handler and managing worker pod lifecycle.
package grpc

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Team-5-CodeCat/ottoscaler/internal/config"
	"github.com/Team-5-CodeCat/ottoscaler/internal/k8s"
	"github.com/Team-5-CodeCat/ottoscaler/internal/pipeline"
	"github.com/Team-5-CodeCat/ottoscaler/internal/worker"
	pb "github.com/Team-5-CodeCat/ottoscaler/pkg/proto/v1"
)

// Server implements the OttoscalerService gRPC server.
//
// Server는 Otto-handler로부터 스케일링 요청을 받아 처리하는 gRPC 서버입니다.
// 모든 gRPC 메서드는 context를 첫 번째 매개변수로 받아 취소와 타임아웃을 지원합니다.
type Server struct {
	pb.UnimplementedOttoscalerServiceServer

	config          *config.Config
	workerManager   *worker.Manager
	k8sClient       *k8s.Client
	logStreamServer *LogStreamingServer
	
	// Pipeline 실행 관리
	pipelineExecutors map[string]*pipeline.Executor
	pipelineMu        sync.RWMutex
}

// NewServer creates a new gRPC server instance.
//
// NewServer는 새로운 gRPC 서버 인스턴스를 생성합니다.
//
// Parameters:
//   - cfg: 서버 설정
//   - workerManager: Worker Pod 관리자
//   - k8sClient: Kubernetes API 클라이언트
//
// Returns:
//   - *Server: 초기화된 서버 인스턴스
func NewServer(cfg *config.Config, workerManager *worker.Manager, k8sClient *k8s.Client) *Server {
	if cfg == nil || workerManager == nil || k8sClient == nil {
		panic("NewServer: nil parameters are not allowed")
	}

	log.Printf("🚀 Ottoscaler gRPC 서버 초기화 중 (포트: %d)", cfg.GRPC.Port)

	// Initialize log streaming server with Otto-handler configuration
	ottoHandlerAddress := cfg.GRPC.OttoHandlerHost
	mockMode := cfg.GRPC.MockMode

	if mockMode {
		log.Printf("🎭 MOCK 모드로 실행 중 - Otto-handler 로그가 시뮬레이션됩니다")
	} else {
		log.Printf("🔗 Otto-handler 연결 중: %s", ottoHandlerAddress)
	}

	logStreamServer := NewLogStreamingServer(k8sClient, ottoHandlerAddress, mockMode)

	return &Server{
		config:            cfg,
		workerManager:     workerManager,
		k8sClient:         k8sClient,
		logStreamServer:   logStreamServer,
		pipelineExecutors: make(map[string]*pipeline.Executor),
	}
}

// ScaleUp handles scale up requests from otto-handler.
//
// ScaleUp은 otto-handler로부터 스케일 업 요청을 처리합니다.
// 요청된 수만큼 Worker Pod를 생성하고 결과를 반환합니다.
func (s *Server) ScaleUp(ctx context.Context, req *pb.ScaleRequest) (*pb.ScaleResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	startTime := time.Now()
	log.Printf("📈 ScaleUp 요청 수신: task_id=%s, worker_count=%d, repository=%s",
		req.TaskId, req.WorkerCount, req.Repository)

	// Validate request
	if req.TaskId == "" {
		return nil, status.Error(codes.InvalidArgument, "task_id is required")
	}
	if req.WorkerCount <= 0 {
		return nil, status.Error(codes.InvalidArgument, "worker_count must be positive")
	}

	// Create worker configurations
	workerConfigs := s.createWorkerConfigs(req)

	// Extract worker names for response
	workerPodNames := make([]string, len(workerConfigs))
	for i, config := range workerConfigs {
		workerPodNames[i] = config.Name
	}

	// Run workers using worker manager (in background to not block gRPC response)
	go func() {
		workerCtx := context.Background() // Use independent context for worker execution
		if err := s.workerManager.RunMultipleWorkers(workerCtx, workerConfigs); err != nil {
			log.Printf("❌ 태스크 %s의 Worker 실행 오류: %v", req.TaskId, err)
		}
	}()

	response := &pb.ScaleResponse{
		Status:         pb.ScaleResponse_SUCCESS,
		Message:        fmt.Sprintf("Successfully started %d workers for task %s", req.WorkerCount, req.TaskId),
		ProcessedCount: req.WorkerCount,
		WorkerPodNames: workerPodNames,
		StartedAt:      startTime.Format(time.RFC3339),
		CompletedAt:    time.Now().Format(time.RFC3339),
	}

	log.Printf("✅ ScaleUp 완료: task_id=%s, 처리된 수=%d, 소요 시간=%v",
		req.TaskId, response.ProcessedCount, time.Since(startTime))

	return response, nil
}

// ScaleDown handles scale down requests from otto-handler.
//
// ScaleDown은 otto-handler로부터 스케일 다운 요청을 처리합니다.
// 지정된 Worker Pod들을 종료하고 결과를 반환합니다.
func (s *Server) ScaleDown(ctx context.Context, req *pb.ScaleRequest) (*pb.ScaleResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	startTime := time.Now()
	log.Printf("📉 ScaleDown 요청 수신: task_id=%s, 목표 수=%d",
		req.TaskId, req.WorkerCount)

	// Validate request
	if req.TaskId == "" {
		return nil, status.Error(codes.InvalidArgument, "task_id is required")
	}

	// TODO: Implement actual worker termination logic
	response := &pb.ScaleResponse{
		Status:         pb.ScaleResponse_SUCCESS,
		Message:        fmt.Sprintf("Successfully processed scale down request for task %s", req.TaskId),
		ProcessedCount: 0,          // TODO: Fill with actual terminated count
		WorkerPodNames: []string{}, // TODO: Fill with actual terminated pod names
		StartedAt:      startTime.Format(time.RFC3339),
		CompletedAt:    time.Now().Format(time.RFC3339),
	}

	log.Printf("✅ ScaleDown 완료: task_id=%s, 처리된 수=%d, 소요 시간=%v",
		req.TaskId, response.ProcessedCount, time.Since(startTime))

	return response, nil
}

// GetWorkerStatus handles worker status requests from otto-handler.
//
// GetWorkerStatus는 otto-handler로부터 Worker 상태 조회 요청을 처리합니다.
// 현재 활성 상태인 Worker Pod들의 상태 정보를 반환합니다.
func (s *Server) GetWorkerStatus(ctx context.Context, req *pb.WorkerStatusRequest) (*pb.WorkerStatusResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	log.Printf("📊 GetWorkerStatus 요청 수신: task_id=%s, 상태 필터=%s",
		req.TaskId, req.StatusFilter)

	// Get worker statuses from Kubernetes
	workerStatuses, err := s.convertWorkerStatusesToPB(ctx, req.TaskId)
	if err != nil {
		log.Printf("❌ Worker 상태 조회 실패: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to retrieve worker statuses: %v", err)
	}

	// Calculate status counts
	running, pending, succeeded, failed := calculateStatusCounts(workerStatuses)

	response := &pb.WorkerStatusResponse{
		TotalCount:     int32(len(workerStatuses)),
		RunningCount:   running,
		PendingCount:   pending,
		SucceededCount: succeeded,
		FailedCount:    failed,
		Workers:        workerStatuses,
	}

	log.Printf("✅ GetWorkerStatus 완료: 총=%d, 실행 중=%d, 대기 중=%d",
		response.TotalCount, response.RunningCount, response.PendingCount)

	return response, nil
}

// ExecutePipeline handles pipeline execution requests from otto-handler.
//
// ExecutePipeline은 otto-handler로부터 Pipeline 실행 요청을 처리합니다.
// 전체 Pipeline을 받아 Stage별로 분석하고 의존성에 따라 실행합니다.
func (s *Server) ExecutePipeline(req *pb.PipelineRequest, stream pb.OttoscalerService_ExecutePipelineServer) error {
	if req == nil {
		return status.Error(codes.InvalidArgument, "request cannot be nil")
	}
	
	// Validate request
	if req.PipelineId == "" {
		return status.Error(codes.InvalidArgument, "pipeline_id is required")
	}
	if len(req.Stages) == 0 {
		return status.Error(codes.InvalidArgument, "at least one stage is required")
	}
	
	log.Printf("🚀 ExecutePipeline 요청 수신: pipeline_id=%s, name=%s, stages=%d",
		req.PipelineId, req.Name, len(req.Stages))
	
	// Check if pipeline is already running
	s.pipelineMu.RLock()
	if _, exists := s.pipelineExecutors[req.PipelineId]; exists {
		s.pipelineMu.RUnlock()
		return status.Error(codes.AlreadyExists, 
			fmt.Sprintf("pipeline %s is already running", req.PipelineId))
	}
	s.pipelineMu.RUnlock()
	
	// Create new executor
	executor := pipeline.NewExecutor(s.workerManager, s.config.Kubernetes.Namespace)
	
	// Store executor
	s.pipelineMu.Lock()
	s.pipelineExecutors[req.PipelineId] = executor
	s.pipelineMu.Unlock()
	
	// Cleanup executor when done
	defer func() {
		s.pipelineMu.Lock()
		delete(s.pipelineExecutors, req.PipelineId)
		s.pipelineMu.Unlock()
		log.Printf("🧹 Pipeline executor 정리: %s", req.PipelineId)
	}()
	
	// Start pipeline execution
	ctx := stream.Context()
	progressChan, err := executor.Execute(ctx, req)
	if err != nil {
		log.Printf("❌ Pipeline 실행 시작 실패: %v", err)
		return status.Error(codes.Internal, 
			fmt.Sprintf("failed to start pipeline: %v", err))
	}
	
	// Stream progress updates
	for progress := range progressChan {
		if err := stream.Send(progress); err != nil {
			log.Printf("❌ Progress 전송 실패: %v", err)
			executor.Cancel()
			return err
		}
	}
	
	log.Printf("✅ Pipeline 스트리밍 완료: %s", req.PipelineId)
	return nil
}

// Start starts the gRPC server and begins listening for requests.
//
// Start는 gRPC 서버를 시작하고 요청 대기를 시작합니다.
// Context가 취소될 때까지 블로킹됩니다.
func (s *Server) Start(ctx context.Context) error {
	addr := s.config.GetGRPCAddr()

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterOttoscalerServiceServer(grpcServer, s)
	pb.RegisterLogStreamingServiceServer(grpcServer, s.logStreamServer)

	log.Printf("🎯 gRPC 서버 시작 (주소: %s)", addr)

	// Start periodic cleanup for log streaming server
	cleanupTicker := time.NewTicker(5 * time.Minute)
	defer cleanupTicker.Stop()

	go func() {
		for {
			select {
			case <-cleanupTicker.C:
				s.logStreamServer.CleanupInactiveSessions()
			case <-ctx.Done():
				return
			}
		}
	}()

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			errChan <- fmt.Errorf("gRPC server failed: %w", err)
		}
	}()

	// Wait for context cancellation or server error
	select {
	case <-ctx.Done():
		log.Println("🛑 Shutting down gRPC server...")

		// Stop all active log collections
		log.Printf("📜 활성 로그 스트리밍 세션: %d개", s.logStreamServer.GetActiveSessionsCount())

		grpcServer.GracefulStop()
		log.Println("✅ gRPC server stopped gracefully")
		return ctx.Err()
	case err := <-errChan:
		return err
	}
}
