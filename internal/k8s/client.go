// Package k8s provides Kubernetes API integration for pod management.
package k8s

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// Client provides Kubernetes API integration for pod management.
//
// Client는 Kubernetes API 클라이언트입니다.
// Ottoscaler Main Pod에서 Worker Pod들을 생성, 관리, 정리하는
// 모든 Kubernetes 작업을 담당합니다.
type Client struct {
	clientset *kubernetes.Clientset
	namespace string
}

// NewClient는 새로운 Kubernetes 클라이언트를 생성합니다.
//
// 인증 방식 우선순위:
//  1. In-cluster config (Pod 내부에서 실행 시)
//  2. Kubeconfig 파일 (로컬 개발 시)
//
// Parameters:
//   - namespace: 작업할 Kubernetes 네임스페이스
//
// Returns:
//   - *Client: 초기화된 클라이언트
//   - error: 초기화 실패 시 에러
func NewClient(namespace string) (*Client, error) {
	config, err := getKubernetesConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get Kubernetes config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes clientset: %w", err)
	}

	if namespace == "" {
		namespace = "default"
	}

	log.Printf("☸️ Kubernetes 클라이언트 초기화 완료 (네임스페이스: %s)", namespace)

	return &Client{
		clientset: clientset,
		namespace: namespace,
	}, nil
}

// getKubernetesConfig는 실행 환경에 맞는 Kubernetes 설정을 반환합니다
func getKubernetesConfig() (*rest.Config, error) {
	// 1순위: In-cluster config (Pod 내부에서 실행 시)
	if config, err := rest.InClusterConfig(); err == nil {
		log.Println("🏠 클러스터 내부 Kubernetes 설정 사용 (ServiceAccount)")
		return config, nil
	}

	// 2순위: Kubeconfig 파일 (로컬 개발 시)
	kubeconfigPath := getKubeconfigPath()
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build config from kubeconfig: %w", err)
	}

	log.Printf("🔧 kubeconfig 파일 사용: %s", kubeconfigPath)
	return config, nil
}

// getKubeconfigPath는 kubeconfig 파일 경로를 반환합니다
func getKubeconfigPath() string {
	// KUBECONFIG 환경변수 확인
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		return kubeconfig
	}

	// 기본 위치 ($HOME/.kube/config)
	if home := homedir.HomeDir(); home != "" {
		return filepath.Join(home, ".kube", "config")
	}

	return ""
}

// CreatePod는 새로운 Pod를 생성합니다
func (c *Client) CreatePod(ctx context.Context, pod *v1.Pod) (*v1.Pod, error) {
	createdPod, err := c.clientset.CoreV1().Pods(c.namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create pod %s: %w", pod.Name, err)
	}

	log.Printf("🆕 Pod 생성 완료: %s", createdPod.Name)
	return createdPod, nil
}

// DeletePod는 Pod를 삭제합니다
func (c *Client) DeletePod(ctx context.Context, name string) error {
	err := c.clientset.CoreV1().Pods(c.namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete pod %s: %w", name, err)
	}

	log.Printf("🗑️ Pod 삭제 완료: %s", name)
	return nil
}

// GetPod는 Pod 정보를 조회합니다
func (c *Client) GetPod(ctx context.Context, name string) (*v1.Pod, error) {
	pod, err := c.clientset.CoreV1().Pods(c.namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get pod %s: %w", name, err)
	}
	return pod, nil
}

// ListPods는 라벨 셀렉터에 맞는 Pod 목록을 조회합니다
func (c *Client) ListPods(ctx context.Context, labelSelector string) (*v1.PodList, error) {
	listOptions := metav1.ListOptions{}
	if labelSelector != "" {
		listOptions.LabelSelector = labelSelector
	}

	pods, err := c.clientset.CoreV1().Pods(c.namespace).List(ctx, listOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to list pods with selector '%s': %w", labelSelector, err)
	}

	return pods, nil
}

// WatchPod는 특정 Pod의 상태 변화를 모니터링합니다.
//
// Pod가 완료(Succeeded) 또는 실패(Failed) 상태가 될 때까지
// 실시간으로 상태 변화를 추적합니다.
func (c *Client) WatchPod(ctx context.Context, name string) error {
	watch, err := c.clientset.CoreV1().Pods(c.namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", name),
	})
	if err != nil {
		return fmt.Errorf("failed to watch pod %s: %w", name, err)
	}
	defer watch.Stop()

	log.Printf("👀 Pod 모니터링 중: %s", name)

	for event := range watch.ResultChan() {
		pod, ok := event.Object.(*v1.Pod)
		if !ok {
			continue
		}

		log.Printf("📊 Pod %s 상태: %s", pod.Name, pod.Status.Phase)

		// 터미널 상태 감지
		if pod.Status.Phase == v1.PodSucceeded || pod.Status.Phase == v1.PodFailed {
			log.Printf("🏁 Pod %s 완료 (상태: %s)", pod.Name, pod.Status.Phase)
			break
		}
	}

	return nil
}

// LogEntry represents a single log line from a pod
//
// LogEntry는 Pod에서 수집된 단일 로그 라인을 나타냅니다.
type LogEntry struct {
	PodName   string    `json:"pod_name"`
	Container string    `json:"container"`
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
	Source    string    `json:"source"` // "stdout" or "stderr"
}

// LogStreamOptions contains options for log streaming
//
// LogStreamOptions는 로그 스트리밍 설정을 담습니다.
type LogStreamOptions struct {
	Follow     bool   // 실시간 로그 스트리밍 여부
	TailLines  *int64 // 마지막 N줄만 가져오기
	SinceTime  *metav1.Time
	Container  string // 특정 컨테이너 지정
	Timestamps bool   // 타임스탬프 포함 여부
	Previous   bool   // 이전 컨테이너 로그 포함
}

// StreamPodLogs는 Pod의 로그를 실시간으로 스트리밍합니다.
//
// StreamPodLogs는 지정된 Pod의 stdout/stderr 로그를 실시간으로 수집하여
// 채널로 전송합니다. 로그가 발생할 때마다 LogEntry로 파싱하여 전달합니다.
//
// Context 취소 시 스트리밍이 중단되고 채널이 닫힙니다.
func (c *Client) StreamPodLogs(ctx context.Context, podName string, options LogStreamOptions) (<-chan LogEntry, <-chan error) {
	logChan := make(chan LogEntry, 100)
	errChan := make(chan error, 1)

	go func() {
		defer close(logChan)
		defer close(errChan)

		// 기본 옵션 설정
		podLogOpts := &v1.PodLogOptions{
			Follow:     options.Follow,
			Timestamps: options.Timestamps,
		}

		if options.TailLines != nil {
			podLogOpts.TailLines = options.TailLines
		}

		if options.SinceTime != nil {
			podLogOpts.SinceTime = options.SinceTime
		}

		if options.Container != "" {
			podLogOpts.Container = options.Container
		}

		if options.Previous {
			podLogOpts.Previous = options.Previous
		}

		// 로그 스트림 요청
		logRequest := c.clientset.CoreV1().Pods(c.namespace).GetLogs(podName, podLogOpts)
		stream, err := logRequest.Stream(ctx)
		if err != nil {
			errChan <- fmt.Errorf("failed to get log stream for pod %s: %w", podName, err)
			return
		}
		defer stream.Close()

		log.Printf("📜 Pod 로그 스트리밍 시작: %s", podName)

		// 로그 라인별로 파싱
		scanner := bufio.NewScanner(stream)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}

			logEntry := LogEntry{
				PodName:   podName,
				Container: options.Container,
				Timestamp: time.Now(),
				Message:   line,
				Source:    "stdout", // Kubernetes logs API는 stdout/stderr 구분이 어려움
			}

			// 타임스탬프가 포함된 경우 파싱
			if options.Timestamps {
				if parsed, msg := c.parseTimestampedLog(line); parsed != nil {
					logEntry.Timestamp = *parsed
					logEntry.Message = msg
				}
			}

			select {
			case logChan <- logEntry:
			case <-ctx.Done():
				log.Printf("📜 Pod 로그 스트리밍 취소됨: %s", podName)
				return
			}
		}

		if err := scanner.Err(); err != nil {
			errChan <- fmt.Errorf("error reading log stream for pod %s: %w", podName, err)
			return
		}

		log.Printf("📜 Pod 로그 스트리밍 완료: %s", podName)
	}()

	return logChan, errChan
}

// parseTimestampedLog은 타임스탬프가 포함된 로그를 파싱합니다
func (c *Client) parseTimestampedLog(line string) (*time.Time, string) {
	// Kubernetes 로그 형식: 2006-01-02T15:04:05.999999999Z message
	if len(line) < 30 {
		return nil, line
	}

	timestampStr := line[:30]
	message := line[30:]

	// 공백 제거
	if len(message) > 0 && message[0] == ' ' {
		message = message[1:]
	}

	// RFC3339 형식으로 파싱
	timestamp, err := time.Parse(time.RFC3339Nano, timestampStr)
	if err != nil {
		return nil, line
	}

	return &timestamp, message
}

// GetPodLogs는 Pod의 로그를 한 번에 가져옵니다.
//
// GetPodLogs는 스트리밍이 아닌 일회성 로그 수집을 위해 사용합니다.
// 주로 완료된 Pod의 전체 로그를 확인할 때 유용합니다.
func (c *Client) GetPodLogs(ctx context.Context, podName string, options LogStreamOptions) (string, error) {
	podLogOpts := &v1.PodLogOptions{
		Timestamps: options.Timestamps,
	}

	if options.TailLines != nil {
		podLogOpts.TailLines = options.TailLines
	}

	if options.SinceTime != nil {
		podLogOpts.SinceTime = options.SinceTime
	}

	if options.Container != "" {
		podLogOpts.Container = options.Container
	}

	if options.Previous {
		podLogOpts.Previous = options.Previous
	}

	logRequest := c.clientset.CoreV1().Pods(c.namespace).GetLogs(podName, podLogOpts)
	logs, err := logRequest.DoRaw(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get logs for pod %s: %w", podName, err)
	}

	return string(logs), nil
}

// StreamMultiplePodLogs는 여러 Pod의 로그를 동시에 스트리밍합니다.
//
// StreamMultiplePodLogs는 라벨 셀렉터로 선택된 여러 Pod의 로그를
// 하나의 채널로 통합하여 전달합니다. 각 로그 엔트리에는 Pod 이름이 포함되어
// 어느 Pod에서 온 로그인지 구분할 수 있습니다.
func (c *Client) StreamMultiplePodLogs(ctx context.Context, labelSelector string, options LogStreamOptions) (<-chan LogEntry, <-chan error) {
	logChan := make(chan LogEntry, 100)
	errChan := make(chan error, 1)

	go func() {
		defer close(logChan)
		defer close(errChan)

		// Pod 목록 조회
		pods, err := c.ListPods(ctx, labelSelector)
		if err != nil {
			errChan <- fmt.Errorf("failed to list pods with selector %s: %w", labelSelector, err)
			return
		}

		if len(pods.Items) == 0 {
			log.Printf("📜 셀렉터로 Pod를 찾을 수 없음: %s", labelSelector)
			return
		}

		log.Printf("📜 %d개 Pod의 로그 스트리밍 시작", len(pods.Items))

		// 각 Pod에 대해 로그 스트리밍 시작
		for _, pod := range pods.Items {
			go func(podName string) {
				podLogChan, podErrChan := c.StreamPodLogs(ctx, podName, options)

				// Pod 로그를 통합 채널로 전달
				for {
					select {
					case logEntry, ok := <-podLogChan:
						if !ok {
							return
						}
						select {
						case logChan <- logEntry:
						case <-ctx.Done():
							return
						}
					case err, ok := <-podErrChan:
						if !ok {
							return
						}
						if err != nil {
							select {
							case errChan <- err:
							case <-ctx.Done():
								return
							}
						}
					case <-ctx.Done():
						return
					}
				}
			}(pod.Name)
		}

		// Context 완료까지 대기
		<-ctx.Done()
		log.Printf("📜 다중 Pod 로그 스트리밍 취소됨")
	}()

	return logChan, errChan
}
