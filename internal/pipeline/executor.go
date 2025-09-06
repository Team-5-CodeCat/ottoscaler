// Package pipeline provides Pipeline execution functionality for Ottoscaler.
//
// 이 패키지는 CI/CD Pipeline을 실행하고 관리하는 기능을 제공합니다.
// 복잡한 build, test, deploy 워크플로우를 Stage별로 관리하고
// 의존성에 따라 순차/병렬 실행을 조율합니다.
package pipeline

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/Team-5-CodeCat/ottoscaler/internal/worker"
	pb "github.com/Team-5-CodeCat/ottoscaler/pkg/proto/v1"
)

// Executor는 Pipeline 실행을 관리하는 구조체입니다.
//
// Pipeline의 Stage들을 분석하여 의존성 그래프를 구성하고,
// 병렬 실행 가능한 Stage들을 동시에 실행하며,
// 각 Stage의 진행 상황을 실시간으로 추적합니다.
type Executor struct {
	workerManager *worker.Manager
	namespace     string

	// Pipeline 실행 상태
	pipeline       *pb.PipelineRequest
	stages         map[string]*StageInfo
	stageOrder     [][]string // 실행 순서 (각 레벨은 병렬 실행 가능)
	progressStream chan *pb.PipelineProgress
	
	// 동기화
	mu             sync.RWMutex
	cancelFunc     context.CancelFunc
	
	// 메트릭
	startTime      time.Time
	endTime        time.Time
}

// StageInfo는 개별 Stage의 실행 정보를 담습니다.
type StageInfo struct {
	Stage          *pb.PipelineStage
	Status         pb.StageStatus
	WorkerPodNames []string
	StartTime      time.Time
	EndTime        time.Time
	Error          error
	RetryCount     int32
	Metrics        *pb.StageMetrics
}

// NewExecutor는 새로운 Pipeline Executor를 생성합니다.
func NewExecutor(workerManager *worker.Manager, namespace string) *Executor {
	return &Executor{
		workerManager:  workerManager,
		namespace:      namespace,
		stages:         make(map[string]*StageInfo),
		progressStream: make(chan *pb.PipelineProgress, 100),
	}
}

// Execute는 Pipeline을 실행합니다.
//
// Pipeline의 Stage들을 분석하여 실행 순서를 결정하고,
// 각 Stage를 적절한 시점에 실행하며,
// 실시간으로 진행 상황을 progressStream으로 전송합니다.
func (e *Executor) Execute(ctx context.Context, req *pb.PipelineRequest) (<-chan *pb.PipelineProgress, error) {
	log.Printf("🚀 Pipeline 실행 시작: %s (%s)", req.PipelineId, req.Name)
	
	// Context with cancellation
	execCtx, cancel := context.WithCancel(ctx)
	e.cancelFunc = cancel
	
	// Initialize
	e.pipeline = req
	e.startTime = time.Now()
	
	// Parse stages and build execution order
	if err := e.parseStages(); err != nil {
		return nil, fmt.Errorf("pipeline 파싱 실패: %w", err)
	}
	
	// Start execution in background
	go e.executePipeline(execCtx)
	
	return e.progressStream, nil
}

// parseStages는 Stage들을 파싱하고 실행 순서를 결정합니다.
func (e *Executor) parseStages() error {
	// Initialize stage info
	for _, stage := range e.pipeline.Stages {
		e.stages[stage.StageId] = &StageInfo{
			Stage:  stage,
			Status: pb.StageStatus_STAGE_PENDING,
		}
	}
	
	// Build execution order based on dependencies
	order, err := e.buildExecutionOrder()
	if err != nil {
		return fmt.Errorf("실행 순서 구성 실패: %w", err)
	}
	
	e.stageOrder = order
	
	log.Printf("📋 Pipeline 실행 순서 결정:")
	for level, stageIDs := range e.stageOrder {
		log.Printf("  Level %d (병렬 실행): %v", level+1, stageIDs)
	}
	
	return nil
}

// buildExecutionOrder는 의존성을 분석하여 실행 순서를 구성합니다.
//
// DAG (Directed Acyclic Graph) 분석을 통해
// 각 레벨에서 병렬 실행 가능한 Stage들을 그룹화합니다.
func (e *Executor) buildExecutionOrder() ([][]string, error) {
	// 의존성 맵 구성
	dependencies := make(map[string][]string)
	inDegree := make(map[string]int)
	
	for stageID, info := range e.stages {
		dependencies[stageID] = info.Stage.DependsOn
		inDegree[stageID] = len(info.Stage.DependsOn)
	}
	
	// Topological sort with level grouping
	var order [][]string
	remaining := len(e.stages)
	
	for remaining > 0 {
		// Find stages with no dependencies (in-degree = 0)
		currentLevel := []string{}
		
		for stageID, degree := range inDegree {
			if degree == 0 {
				currentLevel = append(currentLevel, stageID)
			}
		}
		
		if len(currentLevel) == 0 {
			return nil, fmt.Errorf("순환 의존성 감지됨")
		}
		
		// Add current level to order
		order = append(order, currentLevel)
		
		// Remove processed stages from dependency graph
		for _, stageID := range currentLevel {
			delete(inDegree, stageID)
			remaining--
			
			// Decrease in-degree for dependent stages
			for otherID, deps := range dependencies {
				for _, dep := range deps {
					if dep == stageID {
						inDegree[otherID]--
					}
				}
			}
		}
	}
	
	return order, nil
}

// executePipeline은 Pipeline을 실제로 실행합니다.
func (e *Executor) executePipeline(ctx context.Context) {
	defer close(e.progressStream)
	
	// Send initial progress
	e.sendProgress("", pb.StageStatus_STAGE_PENDING, 
		fmt.Sprintf("Pipeline %s 시작", e.pipeline.Name), 0)
	
	// Execute stages level by level
	for levelIdx, stageIDs := range e.stageOrder {
		log.Printf("🎯 Level %d 실행 시작: %v", levelIdx+1, stageIDs)
		
		// Execute stages in parallel within same level
		if err := e.executeLevel(ctx, stageIDs); err != nil {
			log.Printf("❌ Level %d 실행 실패: %v", levelIdx+1, err)
			e.handlePipelineFailure(ctx, err)
			return
		}
		
		log.Printf("✅ Level %d 완료", levelIdx+1)
	}
	
	// Pipeline completed successfully
	e.endTime = time.Now()
	duration := e.endTime.Sub(e.startTime)
	
	e.sendProgress("", pb.StageStatus_STAGE_COMPLETED,
		fmt.Sprintf("Pipeline 완료 (소요 시간: %v)", duration), 100)
	
	log.Printf("🎉 Pipeline %s 성공적으로 완료!", e.pipeline.PipelineId)
}

// executeLevel은 같은 레벨의 Stage들을 병렬로 실행합니다.
func (e *Executor) executeLevel(ctx context.Context, stageIDs []string) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(stageIDs))
	
	for _, stageID := range stageIDs {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			
			if err := e.executeStage(ctx, id); err != nil {
				log.Printf("❌ Stage %s 실행 실패: %v", id, err)
				errChan <- fmt.Errorf("stage %s: %w", id, err)
			}
		}(stageID)
	}
	
	// Wait for all stages to complete
	wg.Wait()
	close(errChan)
	
	// Check for errors
	for err := range errChan {
		return err // Return first error
	}
	
	return nil
}

// executeStage는 개별 Stage를 실행합니다.
func (e *Executor) executeStage(ctx context.Context, stageID string) error {
	stageInfo := e.stages[stageID]
	stage := stageInfo.Stage
	
	log.Printf("🔨 Stage 실행 시작: %s (%s)", stage.StageId, stage.Name)
	
	// Update status
	e.updateStageStatus(stageID, pb.StageStatus_STAGE_RUNNING)
	stageInfo.StartTime = time.Now()
	
	// Send progress
	e.sendStageProgress(stageID, pb.StageStatus_STAGE_RUNNING, 
		fmt.Sprintf("Stage %s 시작", stage.Name), 0)
	
	// Create worker configurations
	workerConfigs := e.createWorkerConfigs(stage)
	
	// Execute workers
	var err error
	if len(workerConfigs) > 1 {
		// Multiple workers - run in parallel
		err = e.workerManager.RunMultipleWorkers(ctx, workerConfigs)
	} else if len(workerConfigs) == 1 {
		// Single worker
		err = e.workerManager.CreateAndWaitForWorker(ctx, workerConfigs[0])
	}
	
	// Handle result
	stageInfo.EndTime = time.Now()
	
	if err != nil {
		stageInfo.Error = err
		e.updateStageStatus(stageID, pb.StageStatus_STAGE_FAILED)
		
		// Check retry policy
		if e.shouldRetry(stageInfo) {
			log.Printf("🔄 Stage %s 재시도 중...", stageID)
			return e.retryStage(ctx, stageID)
		}
		
		e.sendStageProgress(stageID, pb.StageStatus_STAGE_FAILED,
			fmt.Sprintf("Stage %s 실패: %v", stage.Name, err), 0)
		return err
	}
	
	// Success
	e.updateStageStatus(stageID, pb.StageStatus_STAGE_COMPLETED)
	
	// Calculate metrics
	duration := stageInfo.EndTime.Sub(stageInfo.StartTime)
	stageInfo.Metrics = &pb.StageMetrics{
		DurationSeconds:   int32(duration.Seconds()),
		SuccessfulWorkers: stage.WorkerCount,
		TotalWorkers:      stage.WorkerCount,
	}
	
	e.sendStageProgress(stageID, pb.StageStatus_STAGE_COMPLETED,
		fmt.Sprintf("Stage %s 완료 (소요 시간: %v)", stage.Name, duration), 100)
	
	log.Printf("✅ Stage 완료: %s", stageID)
	return nil
}

// createWorkerConfigs는 Stage를 위한 Worker 설정을 생성합니다.
func (e *Executor) createWorkerConfigs(stage *pb.PipelineStage) []worker.WorkerConfig {
	configs := make([]worker.WorkerConfig, stage.WorkerCount)
	
	// Default image if not specified
	image := stage.Image
	if image == "" {
		image = "busybox:latest" // TODO: Get from config
	}
	
	for i := int32(0); i < stage.WorkerCount; i++ {
		workerID := fmt.Sprintf("otto-%s-%s-%d",
			e.pipeline.PipelineId, stage.StageId, i+1)
		
		configs[i] = worker.WorkerConfig{
			Name:    workerID,
			Image:   image,
			Command: stage.Command,
			Args:    stage.Args,
			Labels: map[string]string{
				"pipeline-id": e.pipeline.PipelineId,
				"stage-id":    stage.StageId,
				"stage-type":  stage.Type,
				"managed-by":  "ottoscaler",
			},
		}
		
		// Store worker pod name
		e.mu.Lock()
		e.stages[stage.StageId].WorkerPodNames = append(
			e.stages[stage.StageId].WorkerPodNames, workerID)
		e.mu.Unlock()
	}
	
	return configs
}

// shouldRetry는 Stage를 재시도해야 하는지 판단합니다.
func (e *Executor) shouldRetry(stageInfo *StageInfo) bool {
	if stageInfo.Stage.RetryPolicy == nil {
		return false
	}
	
	policy := stageInfo.Stage.RetryPolicy
	return stageInfo.RetryCount < policy.MaxAttempts
}

// retryStage는 Stage를 재시도합니다.
func (e *Executor) retryStage(ctx context.Context, stageID string) error {
	stageInfo := e.stages[stageID]
	stageInfo.RetryCount++
	
	// Update status
	e.updateStageStatus(stageID, pb.StageStatus_STAGE_RETRYING)
	e.sendStageProgress(stageID, pb.StageStatus_STAGE_RETRYING,
		fmt.Sprintf("재시도 %d/%d", stageInfo.RetryCount, 
			stageInfo.Stage.RetryPolicy.MaxAttempts), 0)
	
	// Wait before retry
	retryDelay := time.Duration(stageInfo.Stage.RetryPolicy.RetryDelaySeconds) * time.Second
	select {
	case <-time.After(retryDelay):
	case <-ctx.Done():
		return ctx.Err()
	}
	
	// Retry execution
	return e.executeStage(ctx, stageID)
}

// handlePipelineFailure는 Pipeline 실패를 처리합니다.
func (e *Executor) handlePipelineFailure(ctx context.Context, err error) {
	e.endTime = time.Now()
	
	// Mark remaining stages as skipped
	for stageID, info := range e.stages {
		if info.Status == pb.StageStatus_STAGE_PENDING {
			e.updateStageStatus(stageID, pb.StageStatus_STAGE_SKIPPED)
		}
	}
	
	e.sendProgress("", pb.StageStatus_STAGE_FAILED,
		fmt.Sprintf("Pipeline 실패: %v", err), 0)
}

// updateStageStatus는 Stage 상태를 업데이트합니다.
func (e *Executor) updateStageStatus(stageID string, status pb.StageStatus) {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	if info, exists := e.stages[stageID]; exists {
		info.Status = status
	}
}

// sendProgress는 Pipeline 진행 상황을 전송합니다.
func (e *Executor) sendProgress(stageID string, status pb.StageStatus, message string, percentage int32) {
	progress := &pb.PipelineProgress{
		PipelineId:         e.pipeline.PipelineId,
		StageId:            stageID,
		Status:             status,
		Message:            message,
		ProgressPercentage: percentage,
		Timestamp:          time.Now().Format(time.RFC3339),
	}
	
	select {
	case e.progressStream <- progress:
	default:
		log.Printf("⚠️ Progress 채널이 가득 참")
	}
}

// sendStageProgress는 Stage별 진행 상황을 전송합니다.
func (e *Executor) sendStageProgress(stageID string, status pb.StageStatus, message string, percentage int32) {
	e.mu.RLock()
	stageInfo := e.stages[stageID]
	e.mu.RUnlock()
	
	progress := &pb.PipelineProgress{
		PipelineId:         e.pipeline.PipelineId,
		StageId:            stageID,
		Status:             status,
		Message:            message,
		ProgressPercentage: percentage,
		Timestamp:          time.Now().Format(time.RFC3339),
		WorkerPodNames:     stageInfo.WorkerPodNames,
		Metrics:            stageInfo.Metrics,
	}
	
	if !stageInfo.StartTime.IsZero() {
		progress.StartedAt = stageInfo.StartTime.Format(time.RFC3339)
	}
	
	if !stageInfo.EndTime.IsZero() {
		progress.CompletedAt = stageInfo.EndTime.Format(time.RFC3339)
	}
	
	if stageInfo.Error != nil {
		progress.ErrorMessage = stageInfo.Error.Error()
	}
	
	select {
	case e.progressStream <- progress:
	default:
		log.Printf("⚠️ Progress 채널이 가득 참")
	}
}

// Cancel은 실행 중인 Pipeline을 취소합니다.
func (e *Executor) Cancel() {
	if e.cancelFunc != nil {
		log.Printf("🛑 Pipeline %s 취소 요청", e.pipeline.PipelineId)
		e.cancelFunc()
	}
}

// GetStatus는 현재 Pipeline 상태를 반환합니다.
func (e *Executor) GetStatus() map[string]*StageInfo {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	// Deep copy to avoid race conditions
	status := make(map[string]*StageInfo)
	for k, v := range e.stages {
		status[k] = v
	}
	
	return status
}