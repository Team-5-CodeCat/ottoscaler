// Package k8s provides Kubernetes API integration for pod management.
package k8s

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

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

	log.Printf("☸️ Kubernetes client initialized for namespace: %s", namespace)

	return &Client{
		clientset: clientset,
		namespace: namespace,
	}, nil
}

// getKubernetesConfig는 실행 환경에 맞는 Kubernetes 설정을 반환합니다
func getKubernetesConfig() (*rest.Config, error) {
	// 1순위: In-cluster config (Pod 내부에서 실행 시)
	if config, err := rest.InClusterConfig(); err == nil {
		log.Println("🏠 Using in-cluster Kubernetes config (ServiceAccount)")
		return config, nil
	}

	// 2순위: Kubeconfig 파일 (로컬 개발 시)
	kubeconfigPath := getKubeconfigPath()
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build config from kubeconfig: %w", err)
	}

	log.Printf("🔧 Using kubeconfig from: %s", kubeconfigPath)
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

	log.Printf("🆕 Pod created: %s", createdPod.Name)
	return createdPod, nil
}

// DeletePod는 Pod를 삭제합니다
func (c *Client) DeletePod(ctx context.Context, name string) error {
	err := c.clientset.CoreV1().Pods(c.namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete pod %s: %w", name, err)
	}

	log.Printf("🗑️ Pod deleted: %s", name)
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

	log.Printf("👀 Watching pod: %s", name)

	for event := range watch.ResultChan() {
		pod, ok := event.Object.(*v1.Pod)
		if !ok {
			continue
		}

		log.Printf("📊 Pod %s status: %s", pod.Name, pod.Status.Phase)

		// 터미널 상태 감지
		if pod.Status.Phase == v1.PodSucceeded || pod.Status.Phase == v1.PodFailed {
			log.Printf("🏁 Pod %s completed with phase: %s", pod.Name, pod.Status.Phase)
			break
		}
	}

	return nil
}
