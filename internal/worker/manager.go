// Package worker provides Worker Pod lifecycle management for Ottoscaler.
//
// 이 패키지는 Otto agent Worker Pod들의 전체 라이프사이클을 관리합니다.
// Kubernetes API를 통해 Worker Pod를 동적으로 생성, 모니터링, 정리하는
// 모든 기능을 제공합니다.
//
// 주요 기능:
//   - 동시 다중 Worker Pod 생성 및 관리 (RunMultipleWorkers)
//   - Pod 상태 모니터링 (2초 간격 폴링)
//   - 작업 완료 후 자동 정리 (CleanupPod)
//   - 에러 복구 및 재시도 로직
//   - 배치 단위 Worker 관리
//   - 활성 Pod 목록 조회 (scale_down 준비)
//
// 사용 예시:
//
//	k8sClient, _ := k8s.NewClient("default")
//	manager := worker.NewManager(k8sClient, "default")
//
//	config := worker.WorkerConfig{
//		Name:    "otto-agent-1",
//		Image:   "busybox:latest",
//		Command: []string{"sh", "-c"},
//		Args:    []string{"echo hello"},
//		Labels:  map[string]string{"app": "otto-agent"},
//	}
//
//	err := manager.CreateAndWaitForWorker(ctx, config)
//
// Worker Manager는 다음과 같은 패턴으로 Pod를 관리합니다:
//   1. 생성 (CreateWorkerPod)
//   2. 모니터링 (WaitForPodCompletion)
//   3. 정리 (CleanupPod)
package worker

import (
	"context"
	"fmt"
	"log"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Team-5-CodeCat/ottoscaler/internal/k8s"
)

const (
	// PodMonitoringInterval은 Pod 상태 확인 간격입니다
	PodMonitoringInterval = 2 * time.Second
	// DefaultWorkerImage는 기본 Worker 이미지입니다
	DefaultWorkerImage = "busybox:latest"
	// WorkerContainerName은 Worker 컨테이너 이름입니다
	WorkerContainerName = "worker"
)

// Manager manages the lifecycle of Otto agent Worker Pods.
//
// Manager는 Worker Pod들의 라이프사이클을 관리하는 컨트롤러입니다.
//
// 책임:
//   - Worker Pod 생성 및 설정
//   - 동시 실행되는 다중 Worker 관리
//   - Pod 완료 상태 모니터링
//   - 자동 정리 및 리소스 해제
type Manager struct {
	k8sClient *k8s.Client
	namespace string
}

// WorkerConfig contains configuration for creating a Worker Pod.
//
// WorkerConfig는 Worker Pod 생성에 필요한 설정을 담는 구조체입니다.
//
// 필드 설명:
//   - Name: Pod 이름 (Kubernetes 네이밍 규칙 준수)
//   - Image: 컨테이너 이미지
//   - Command: 실행할 명령어
//   - Args: 명령어 인자
//   - Labels: Pod에 적용할 라벨 (관리 및 식별용)
//   - Resources: CPU/메모리 리소스 제한 (선택적)
type WorkerConfig struct {
	Name      string            `json:"name"`      // Pod 이름
	Image     string            `json:"image"`     // 컨테이너 이미지
	Command   []string          `json:"command"`   // 실행 명령어
	Args      []string          `json:"args"`      // 명령어 인자
	Labels    map[string]string `json:"labels"`    // Pod 라벨
	Resources *ResourceConfig   `json:"resources"` // 리소스 설정 (선택적)
}

// ResourceConfig defines resource limits for Worker Pods.
//
// ResourceConfig는 Worker Pod의 리소스 제한을 정의합니다.
type ResourceConfig struct {
	CPURequest    string `json:"cpu_request"`    // CPU 요청량 (예: "100m")
	MemoryRequest string `json:"memory_request"` // 메모리 요청량 (예: "128Mi")
	CPULimit      string `json:"cpu_limit"`      // CPU 제한 (예: "500m")
	MemoryLimit   string `json:"memory_limit"`   // 메모리 제한 (예: "256Mi")
}

// WorkerStatus represents the execution result of a Worker Pod.
//
// WorkerStatus는 Worker Pod의 실행 결과를 나타냅니다.
type WorkerStatus struct {
	Name      string        `json:"name"`       // Pod 이름
	Status    string        `json:"status"`     // 최종 상태
	StartTime time.Time     `json:"start_time"` // 시작 시간
	EndTime   time.Time     `json:"end_time"`   // 종료 시간
	Duration  time.Duration `json:"duration"`   // 실행 시간
	Error     error         `json:"error"`      // 에러 (실패 시)
}

// NewManager는 새로운 Worker Manager를 생성합니다.
//
// Parameters:
//   - k8sClient: Kubernetes API 클라이언트
//   - namespace: Worker Pod를 생성할 네임스페이스
//
// Returns:
//   - *Manager: 초기화된 매니저
func NewManager(k8sClient *k8s.Client, namespace string) *Manager {
	if namespace == "" {
		namespace = "default"
	}

	log.Printf("👷 Worker Manager initialized for namespace: %s", namespace)

	return &Manager{
		k8sClient: k8sClient,
		namespace: namespace,
	}
}

// CreateWorkerPod는 WorkerConfig를 바탕으로 새로운 Worker Pod를 생성합니다.
//
// 생성되는 Pod의 특징:
//   - RestartPolicy: Never (일회성 작업)
//   - 관리 라벨 자동 추가
//   - 리소스 제한 적용 (설정된 경우)
func (m *Manager) CreateWorkerPod(ctx context.Context, config WorkerConfig) (*v1.Pod, error) {
	// Pod 스펙 생성
	podSpec := m.buildPodSpec(config)

	log.Printf("🚀 Creating worker pod: %s (image: %s)", config.Name, config.Image)

	createdPod, err := m.k8sClient.CreatePod(ctx, podSpec)
	if err != nil {
		return nil, fmt.Errorf("failed to create worker pod %s: %w", config.Name, err)
	}

	return createdPod, nil
}

// buildPodSpec은 WorkerConfig로부터 Pod 스펙을 구성합니다
func (m *Manager) buildPodSpec(config WorkerConfig) *v1.Pod {
	// 기본 라벨 설정
	labels := make(map[string]string)
	for k, v := range config.Labels {
		labels[k] = v
	}

	// 필수 관리 라벨 추가
	if labels["managed-by"] == "" {
		labels["managed-by"] = "ottoscaler"
	}
	if labels["app"] == "" {
		labels["app"] = "otto-agent"
	}

	// 컨테이너 스펙 생성
	container := v1.Container{
		Name:    WorkerContainerName,
		Image:   config.Image,
		Command: config.Command,
		Args:    config.Args,
	}

	// 리소스 제한 적용
	if config.Resources != nil {
		container.Resources = m.buildResourceRequirements(config.Resources)
	}

	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.Name,
			Namespace: m.namespace,
			Labels:    labels,
			Annotations: map[string]string{
				"ottoscaler.io/created-at": time.Now().Format(time.RFC3339),
			},
		},
		Spec: v1.PodSpec{
			RestartPolicy: v1.RestartPolicyNever,
			Containers:    []v1.Container{container},
		},
	}
}

// buildResourceRequirements는 ResourceConfig를 Kubernetes ResourceRequirements로 변환합니다
func (m *Manager) buildResourceRequirements(config *ResourceConfig) v1.ResourceRequirements {
	// TODO: 실제 리소스 파싱 구현
	// 현재는 기본값 반환
	return v1.ResourceRequirements{}
}

// WaitForPodCompletion은 Pod가 완료될 때까지 대기합니다.
//
// 모니터링 방식:
//   - 2초 간격으로 Pod 상태 폴링
//   - Succeeded: 정상 완료
//   - Failed: 실패
//   - Running/Pending: 계속 대기
//
// Context 취소 시 즉시 반환합니다.
func (m *Manager) WaitForPodCompletion(ctx context.Context, podName string) error {
	log.Printf("⏳ Waiting for pod %s to complete...", podName)

	ticker := time.NewTicker(PodMonitoringInterval)
	defer ticker.Stop()

	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			log.Printf("⏰ Pod monitoring cancelled for %s after %v", podName, time.Since(startTime))
			return ctx.Err()

		case <-ticker.C:
			pod, err := m.k8sClient.GetPod(ctx, podName)
			if err != nil {
				log.Printf("⚠️ Error getting pod %s: %v", podName, err)
				continue
			}

			// 상태별 처리
			switch pod.Status.Phase {
			case v1.PodSucceeded:
				duration := time.Since(startTime)
				log.Printf("✅ Pod %s completed successfully in %v", podName, duration)
				return nil

			case v1.PodFailed:
				duration := time.Since(startTime)
				reason := m.getPodFailureReason(pod)
				log.Printf("❌ Pod %s failed after %v: %s", podName, duration, reason)
				return fmt.Errorf("pod %s failed: %s", podName, reason)

			case v1.PodRunning:
				log.Printf("🏃 Pod %s is running... (elapsed: %v)", podName, time.Since(startTime))

			case v1.PodPending:
				log.Printf("⏸️ Pod %s is pending... (elapsed: %v)", podName, time.Since(startTime))

			default:
				log.Printf("❓ Pod %s in unknown state: %s", podName, pod.Status.Phase)
			}
		}
	}
}

// getPodFailureReason은 Pod 실패 원인을 분석하여 반환합니다
func (m *Manager) getPodFailureReason(pod *v1.Pod) string {
	// 컨테이너 상태 확인
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.State.Terminated != nil {
			terminated := containerStatus.State.Terminated
			if terminated.ExitCode != 0 {
				return fmt.Sprintf("container exited with code %d: %s",
					terminated.ExitCode, terminated.Reason)
			}
		}

		if containerStatus.State.Waiting != nil {
			waiting := containerStatus.State.Waiting
			return fmt.Sprintf("container waiting: %s - %s",
				waiting.Reason, waiting.Message)
		}
	}

	// Pod 조건 확인
	for _, condition := range pod.Status.Conditions {
		if condition.Type == v1.PodReady && condition.Status == v1.ConditionFalse {
			return fmt.Sprintf("pod not ready: %s", condition.Message)
		}
	}

	return "unknown failure reason"
}

// CleanupPod는 완료된 Pod를 정리합니다
func (m *Manager) CleanupPod(ctx context.Context, podName string) error {
	log.Printf("🧹 Cleaning up pod: %s", podName)

	if err := m.k8sClient.DeletePod(ctx, podName); err != nil {
		return fmt.Errorf("failed to cleanup pod %s: %w", podName, err)
	}

	return nil
}

// CreateAndWaitForWorker는 Worker Pod를 생성하고 완료까지 대기한 후 정리합니다.
//
// 전체 프로세스:
//  1. Pod 생성
//  2. 완료 대기 (모니터링)
//  3. 자동 정리
//
// 에러 발생 시에도 정리를 시도합니다.
func (m *Manager) CreateAndWaitForWorker(ctx context.Context, config WorkerConfig) error {
	startTime := time.Now()

	// 1. Worker Pod 생성
	_, err := m.CreateWorkerPod(ctx, config)
	if err != nil {
		return fmt.Errorf("worker creation failed: %w", err)
	}

	// 2. 완료 대기
	err = m.WaitForPodCompletion(ctx, config.Name)

	// 3. 정리 (성공/실패 관계없이 수행)
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if cleanupErr := m.CleanupPod(cleanupCtx, config.Name); cleanupErr != nil {
		log.Printf("⚠️ Warning: failed to cleanup pod %s: %v", config.Name, cleanupErr)
	}

	totalDuration := time.Since(startTime)

	if err != nil {
		log.Printf("❌ Worker %s failed after %v: %v", config.Name, totalDuration, err)
		return fmt.Errorf("worker %s failed: %w", config.Name, err)
	}

	log.Printf("✅ Worker %s completed successfully in %v", config.Name, totalDuration)
	return nil
}

// RunMultipleWorkers는 여러 Worker Pod를 동시에 실행하고 모든 완료를 대기합니다.
//
// 특징:
//   - 모든 Worker를 동시에 시작 (병렬 처리)
//   - 각 Worker는 독립적으로 실행 (하나 실패해도 다른 Worker 계속 실행)
//   - 모든 Worker 완료 후 전체 결과 반환
//   - 부분 실패 시에도 상세한 에러 정보 제공
func (m *Manager) RunMultipleWorkers(ctx context.Context, configs []WorkerConfig) error {
	if len(configs) == 0 {
		return fmt.Errorf("no worker configurations provided")
	}

	log.Printf("🚀 Starting batch of %d worker pods", len(configs))
	startTime := time.Now()

	// 결과 수집용 채널
	results := make(chan WorkerStatus, len(configs))

	// 모든 Worker를 동시에 시작
	for _, config := range configs {
		go func(cfg WorkerConfig) {
			workerStartTime := time.Now()
			status := WorkerStatus{
				Name:      cfg.Name,
				StartTime: workerStartTime,
			}

			// Worker 실행
			err := m.CreateAndWaitForWorker(ctx, cfg)

			// 결과 기록
			status.EndTime = time.Now()
			status.Duration = status.EndTime.Sub(status.StartTime)
			status.Error = err

			if err != nil {
				status.Status = "failed"
			} else {
				status.Status = "succeeded"
			}

			results <- status
		}(config)
	}

	// 모든 Worker 완료 대기 및 결과 수집
	var (
		successCount = 0
		failureCount = 0
		errors       []error
		statuses     = make([]WorkerStatus, 0, len(configs))
	)

	for i := 0; i < len(configs); i++ {
		status := <-results
		statuses = append(statuses, status)

		if status.Error != nil {
			failureCount++
			errors = append(errors, fmt.Errorf("worker %s: %w", status.Name, status.Error))
		} else {
			successCount++
		}
	}

	totalDuration := time.Since(startTime)

	// 결과 로깅
	log.Printf("📊 Batch completed in %v: %d succeeded, %d failed",
		totalDuration, successCount, failureCount)

	// 개별 Worker 상태 로깅
	for _, status := range statuses {
		if status.Error != nil {
			log.Printf("  ❌ %s: failed in %v", status.Name, status.Duration)
		} else {
			log.Printf("  ✅ %s: succeeded in %v", status.Name, status.Duration)
		}
	}

	// 실패가 있는 경우 에러 반환
	if failureCount > 0 {
		return fmt.Errorf("batch partially failed: %d/%d workers failed: %v",
			failureCount, len(configs), errors)
	}

	log.Printf("🎉 All %d workers completed successfully!", successCount)
	return nil
}

// ListActivePods는 현재 활성 상태인 Worker Pod 목록을 반환합니다.
//
// 활성 상태 정의:
//   - Pending: 시작 대기 중
//   - Running: 실행 중
//
// 이 메서드는 scale_down 기능 구현 시 사용됩니다.
func (m *Manager) ListActivePods(ctx context.Context) ([]*v1.Pod, error) {
	// managed-by=ottoscaler 라벨로 필터링
	podList, err := m.k8sClient.ListPods(ctx, "managed-by=ottoscaler")
	if err != nil {
		return nil, fmt.Errorf("failed to list worker pods: %w", err)
	}

	activePods := make([]*v1.Pod, 0)

	for i := range podList.Items {
		pod := &podList.Items[i]

		// 활성 상태인 Pod만 포함
		if pod.Status.Phase == v1.PodPending || pod.Status.Phase == v1.PodRunning {
			activePods = append(activePods, pod)
		}
	}

	log.Printf("📋 Found %d active worker pods", len(activePods))
	return activePods, nil
}

// TerminatePods는 지정된 수만큼 Worker Pod를 종료합니다.
//
// 종료 전략:
//   - 가장 오래된 Pod부터 종료 (FIFO)
//   - Graceful termination 시도
//   - 강제 종료는 수행하지 않음
//
// TODO: scale_down 기능과 함께 구현 예정
func (m *Manager) TerminatePods(ctx context.Context, count int) error {
	// TODO: scale_down 기능 구현 시 추가
	return fmt.Errorf("pod termination not implemented yet")
}
