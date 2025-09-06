// Package grpc provides scaling logic for gRPC server implementation.
//
// This file contains the actual scaling implementation that bridges
// gRPC requests with the worker manager.
package grpc

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"

	"github.com/Team-5-CodeCat/ottoscaler/internal/worker"
	pb "github.com/Team-5-CodeCat/ottoscaler/pkg/proto/v1"
)

// createWorkerConfigs creates worker configurations based on scale request.
//
// createWorkerConfigs는 스케일 요청을 기반으로 Worker 설정을 생성합니다.
func (s *Server) createWorkerConfigs(req *pb.ScaleRequest) []worker.WorkerConfig {
	configs := make([]worker.WorkerConfig, req.WorkerCount)

	for i := int32(0); i < req.WorkerCount; i++ {
		workerID := fmt.Sprintf("otto-agent-%s-%d",
			sanitizeTaskID(req.TaskId), i+1)

		// Build worker labels
		workerLabels := s.config.GetWorkerLabels(map[string]string{
			"app":          "otto-agent",
			"task-id":      req.TaskId,
			"repository":   sanitizeLabel(req.Repository),
			"commit-sha":   sanitizeLabel(req.CommitSha),
			"worker-index": strconv.Itoa(int(i + 1)),
		})

		// Create worker configuration
		configs[i] = worker.WorkerConfig{
			Name:    workerID,
			Image:   s.config.Worker.Image,
			Command: s.buildWorkerCommand(req),
			Args:    s.buildWorkerArgs(req),
			Labels:  workerLabels,
			Resources: &worker.ResourceConfig{
				CPULimit:    s.config.Worker.CPULimit,
				MemoryLimit: s.config.Worker.MemoryLimit,
			},
		}
	}

	return configs
}

// buildWorkerCommand builds the command for worker pod based on request.
//
// buildWorkerCommand는 요청을 기반으로 Worker Pod 명령을 구성합니다.
func (s *Server) buildWorkerCommand(req *pb.ScaleRequest) []string {
	// For now, use a simple shell command
	// TODO: Replace with actual Otto agent image and commands
	return []string{"sh", "-c"}
}

// buildWorkerArgs builds the arguments for worker pod based on request.
//
// buildWorkerArgs는 요청을 기반으로 Worker Pod 인자를 구성합니다.
func (s *Server) buildWorkerArgs(req *pb.ScaleRequest) []string {
	// Build script that simulates CI/CD work
	script := fmt.Sprintf(`
echo "🚀 Otto Agent Worker started"
echo "Task ID: %s"
echo "Repository: %s"
echo "Commit SHA: %s"
echo "Triggered by: %s"
echo "Reason: %s"

# Simulate CI/CD work
echo "📁 Cloning repository..."
sleep 2
echo "🔨 Building project..."
sleep 5
echo "🧪 Running tests..."
sleep 3
echo "✅ CI/CD job completed successfully"
`, req.TaskId, req.Repository, req.CommitSha, req.TriggeredBy, req.Reason)

	return []string{script}
}

// convertWorkerStatusesToPB converts internal worker statuses to protobuf format.
//
// convertWorkerStatusesToPB는 내부 Worker 상태를 protobuf 형식으로 변환합니다.
func (s *Server) convertWorkerStatusesToPB(ctx context.Context, taskID string) ([]*pb.WorkerPodStatus, error) {
	// Get active pods from worker manager
	activePods, err := s.workerManager.ListActivePods(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list active pods: %w", err)
	}

	var pbStatuses []*pb.WorkerPodStatus

	for _, pod := range activePods {
		// Filter by task ID if specified
		if taskID != "" {
			if podTaskID, exists := pod.Labels["task-id"]; !exists || podTaskID != taskID {
				continue
			}
		}

		pbStatus := &pb.WorkerPodStatus{
			PodName:   pod.Name,
			TaskId:    pod.Labels["task-id"],
			Status:    string(pod.Status.Phase),
			CreatedAt: formatTime(pod.CreationTimestamp.Time),
			NodeName:  pod.Spec.NodeName,
			PodIp:     pod.Status.PodIP,
			Labels:    pod.Labels,
		}

		// Set start time
		if pod.Status.StartTime != nil {
			pbStatus.StartedAt = formatTime(pod.Status.StartTime.Time)
		}

		// Set completion time and error message for terminated pods
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.State.Terminated != nil {
				terminated := containerStatus.State.Terminated
				pbStatus.CompletedAt = formatTime(terminated.FinishedAt.Time)

				if terminated.ExitCode != 0 {
					pbStatus.ErrorMessage = fmt.Sprintf("Container exited with code %d: %s",
						terminated.ExitCode, terminated.Reason)
				}
			}
		}

		pbStatuses = append(pbStatuses, pbStatus)
	}

	return pbStatuses, nil
}

// calculateStatusCounts calculates status counts from worker pod statuses.
//
// calculateStatusCounts는 Worker Pod 상태에서 상태별 개수를 계산합니다.
func calculateStatusCounts(statuses []*pb.WorkerPodStatus) (running, pending, succeeded, failed int32) {
	for _, status := range statuses {
		switch status.Status {
		case string(v1.PodRunning):
			running++
		case string(v1.PodPending):
			pending++
		case string(v1.PodSucceeded):
			succeeded++
		case string(v1.PodFailed):
			failed++
		}
	}
	return
}

// sanitizeTaskID sanitizes task ID for use in pod names.
//
// sanitizeTaskID는 Task ID를 Pod 이름에 사용할 수 있도록 정리합니다.
func sanitizeTaskID(taskID string) string {
	// Replace invalid characters with hyphens
	sanitized := strings.ReplaceAll(taskID, "_", "-")
	sanitized = strings.ReplaceAll(sanitized, ".", "-")
	sanitized = strings.ToLower(sanitized)

	// Kubernetes pod names must be max 63 characters
	if len(sanitized) > 50 {
		sanitized = sanitized[:50]
	}

	return sanitized
}

// sanitizeLabel sanitizes label values for Kubernetes labels.
//
// sanitizeLabel은 Kubernetes 라벨에 사용할 수 있도록 값을 정리합니다.
func sanitizeLabel(value string) string {
	if len(value) > 63 {
		value = value[:63]
	}

	// Replace invalid characters
	sanitized := strings.ReplaceAll(value, "/", "-")
	sanitized = strings.ReplaceAll(sanitized, ":", "-")
	sanitized = strings.ReplaceAll(sanitized, "@", "-")

	return sanitized
}

// formatTime formats time for protobuf string fields.
//
// formatTime은 protobuf 문자열 필드를 위해 시간을 포맷합니다.
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}
