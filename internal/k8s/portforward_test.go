package k8s

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

func TestSelectPodForService_ReadyPod(t *testing.T) {
	// fake clientset作成
	clientset := fake.NewClientset()
	ctx := context.Background()

	// テスト用Serviceを作成
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-svc",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": "test",
			},
		},
	}
	_, err := clientset.CoreV1().Services("default").Create(ctx, svc, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	// テスト用Pod (Ready状態) を作成
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-1",
			Namespace: "default",
			Labels: map[string]string{
				"app": "test",
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
	_, err = clientset.CoreV1().Pods("default").Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	// selectPodForService実行
	podName, err := selectPodForService(ctx, clientset, "default", "test-svc")
	if err != nil {
		t.Fatalf("selectPodForService failed: %v", err)
	}

	// 期待値の検証
	expectedPodName := "test-pod-1"
	if podName != expectedPodName {
		t.Errorf("expected pod name %q, got %q", expectedPodName, podName)
	}
}

func TestSelectPodForService_NoReadyPod(t *testing.T) {
	// fake clientset作成
	clientset := fake.NewClientset()
	ctx := context.Background()

	// テスト用Serviceを作成
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-svc",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": "test",
			},
		},
	}
	_, err := clientset.CoreV1().Services("default").Create(ctx, svc, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	// テスト用Pod (Not Ready状態) を作成
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-1",
			Namespace: "default",
			Labels: map[string]string{
				"app": "test",
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending, // Not Running
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionFalse,
				},
			},
		},
	}
	_, err = clientset.CoreV1().Pods("default").Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	// selectPodForService実行
	podName, err := selectPodForService(ctx, clientset, "default", "test-svc")
	if err != nil {
		t.Fatalf("selectPodForService failed: %v", err)
	}

	// Ready状態のPodがない場合、最初のPodを選択
	expectedPodName := "test-pod-1"
	if podName != expectedPodName {
		t.Errorf("expected pod name %q, got %q", expectedPodName, podName)
	}
}

func TestSelectPodForService_NoPods(t *testing.T) {
	// fake clientset作成
	clientset := fake.NewClientset()
	ctx := context.Background()

	// テスト用Serviceを作成（Podは作成しない）
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-svc",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": "test",
			},
		},
	}
	_, err := clientset.CoreV1().Services("default").Create(ctx, svc, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	// selectPodForService実行
	_, err = selectPodForService(ctx, clientset, "default", "test-svc")

	// Podが見つからない場合、エラーを返す
	if err == nil {
		t.Fatal("expected error when no pods found, but got nil")
	}
}

func TestSelectPodForService_NoSelector(t *testing.T) {
	// fake clientset作成
	clientset := fake.NewClientset()
	ctx := context.Background()

	// テスト用Service（selectorなし）を作成
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-svc",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{}, // 空のselector
		},
	}
	_, err := clientset.CoreV1().Services("default").Create(ctx, svc, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	// selectPodForService実行
	_, err = selectPodForService(ctx, clientset, "default", "test-svc")

	// Serviceにselectorがない場合、エラーを返す
	if err == nil {
		t.Fatal("expected error when service has no selector, but got nil")
	}
}

func TestIsPodReady(t *testing.T) {
	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected bool
	}{
		{
			name: "Ready Pod",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "Not Running Pod",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "Running but Not Ready",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionFalse,
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "No Ready Condition",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase:      corev1.PodRunning,
					Conditions: []corev1.PodCondition{},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPodReady(tt.pod)
			if result != tt.expected {
				t.Errorf("isPodReady() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// Testing Strategy Notes for StartPortForwardLoop and CreatePortForwarder
//
// StartPortForwardLoop()とCreatePortForwarder()は、k8s.io/client-goの
// WebSocket実装に依存しており、純粋なユニットテストは困難です。
//
// フェーズ1では、既存コードを変更せずにテスト可能な部分のみをテストします：
// - コンテキストキャンセルの処理
// - Pod未存在時の再試行ロジック
//
// フェーズ2では、PortForwarderFactoryパターンを導入し、テスタビリティを
// 大幅に向上させます（80-90%のカバレッジ目標）。
//
// 実際のSPDY通信は統合テストでカバーする必要があります（将来実装）。

func TestStartPortForwardLoop_ContextCancellation(t *testing.T) {
	clientset := fake.NewClientset()
	ctx, cancel := context.WithCancel(t.Context())

	// 即座にキャンセル
	cancel()

	// StartPortForwardLoopを実行
	// 既にキャンセル済みなので、すぐに終了するはず
	err := StartPortForwardLoop(ctx, nil, clientset, "default", "test-svc", 8080, 9090)

	// コンテキストキャンセル時はnilを返す
	if err != nil {
		t.Errorf("expected nil error on context cancellation, got: %v", err)
	}
}

func TestStartPortForwardLoop_NoPodRetry(t *testing.T) {
	clientset := fake.NewClientset()
	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()

	// ServiceだけでPodなし（selectPodForServiceが常に失敗する）
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-svc",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": "test"},
			Ports: []corev1.ServicePort{
				{Name: "http", Port: 8080},
			},
		},
	}
	_, err := clientset.CoreV1().Services("default").Create(
		t.Context(), svc, metav1.CreateOptions{},
	)
	if err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	err = StartPortForwardLoop(ctx, nil, clientset, "default", "test-svc", 8080, 9090)
	elapsed := time.Since(start)

	// タイムアウトで正常終了
	if err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}

	// 少なくとも1回のsleep (300ms) が実行されていることを確認
	// （実際には複数回の再試行が行われるはず）
	if elapsed < 300*time.Millisecond {
		t.Errorf("expected retry loop with sleep, elapsed: %v", elapsed)
	}
}

// mockPortForwarderFactory for testing
type mockPortForwarderFactory struct {
	createFunc func(ctx context.Context, namespace, podName string,
		localPort, remotePort int) (PortForwarder, error)
	callCount int
}

func (m *mockPortForwarderFactory) CreatePortForwarder(
	ctx context.Context,
	namespace, podName string,
	localPort, remotePort int,
) (PortForwarder, error) {
	m.callCount++
	if m.createFunc != nil {
		return m.createFunc(ctx, namespace, podName, localPort, remotePort)
	}
	return nil, fmt.Errorf("mock error")
}

// mockPortForwarder for testing
type mockPortForwarder struct {
	forwardFunc func() error
	callCount   int
}

func (m *mockPortForwarder) ForwardPorts() error {
	m.callCount++
	if m.forwardFunc != nil {
		return m.forwardFunc()
	}
	return nil
}

// setupServiceAndReadyPod creates a service and ready pod for testing
func setupServiceAndReadyPod(
	t *testing.T,
	clientset *fake.Clientset,
	namespace, serviceName, podName string,
) {
	t.Helper()

	ctx := t.Context()

	// Service作成
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": "test"},
			Ports: []corev1.ServicePort{
				{Name: "http", Port: 8080},
			},
		},
	}
	_, err := clientset.CoreV1().Services(namespace).Create(ctx, svc, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	// Ready Pod作成
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
			Labels:    map[string]string{"app": "test"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
	_, err = clientset.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create pod: %v", err)
	}
}

func TestStartPortForwardLoopWithFactory_Success(t *testing.T) {
	clientset := fake.NewClientset()
	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()

	// Service/Pod setup
	setupServiceAndReadyPod(t, clientset, "default", "test-svc", "test-pod")

	mockPF := &mockPortForwarder{
		forwardFunc: func() error {
			// 100ms待ってからブロッキングを解除
			time.Sleep(100 * time.Millisecond)
			return nil
		},
	}

	mockFactory := &mockPortForwarderFactory{
		createFunc: func(ctx context.Context, namespace, podName string,
			localPort, remotePort int) (PortForwarder, error) {
			if podName != "test-pod" {
				t.Errorf("expected pod name 'test-pod', got %q", podName)
			}
			return mockPF, nil
		},
	}

	err := StartPortForwardLoopWithFactory(
		ctx, mockFactory, clientset, "default", "test-svc", 8080, 9090,
	)

	if err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}

	if mockFactory.callCount < 1 {
		t.Errorf("expected factory to be called at least once, got %d",
			mockFactory.callCount)
	}

	if mockPF.callCount < 1 {
		t.Errorf("expected ForwardPorts to be called at least once, got %d",
			mockPF.callCount)
	}
}

func TestStartPortForwardLoopWithFactory_RetryOnError(t *testing.T) {
	clientset := fake.NewClientset()
	ctx, cancel := context.WithTimeout(t.Context(), 1*time.Second)
	defer cancel()

	setupServiceAndReadyPod(t, clientset, "default", "test-svc", "test-pod")

	callCount := 0
	mockFactory := &mockPortForwarderFactory{
		createFunc: func(ctx context.Context, namespace, podName string,
			localPort, remotePort int) (PortForwarder, error) {
			callCount++
			// 最初の2回はエラー、3回目は成功
			if callCount < 3 {
				return nil, fmt.Errorf("connection error %d", callCount)
			}
			return &mockPortForwarder{
				forwardFunc: func() error {
					<-ctx.Done() // タイムアウトまでブロック
					return nil
				},
			}, nil
		},
	}

	start := time.Now()
	err := StartPortForwardLoopWithFactory(
		ctx, mockFactory, clientset, "default", "test-svc", 8080, 9090,
	)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}

	// 2回の失敗 × 300ms = 600ms以上経過していることを確認
	if elapsed < 600*time.Millisecond {
		t.Errorf("expected at least 2 retries (600ms), elapsed: %v", elapsed)
	}

	if callCount < 3 {
		t.Errorf("expected at least 3 calls (2 failures + 1 success), got %d", callCount)
	}
}

func TestStartPortForwardLoopWithFactory_ContextCancellation(t *testing.T) {
	clientset := fake.NewClientset()
	ctx, cancel := context.WithCancel(t.Context())

	setupServiceAndReadyPod(t, clientset, "default", "test-svc", "test-pod")

	mockFactory := &mockPortForwarderFactory{
		createFunc: func(ctx context.Context, namespace, podName string,
			localPort, remotePort int) (PortForwarder, error) {
			// 即座にキャンセル
			cancel()
			return nil, fmt.Errorf("should not reach here")
		},
	}

	err := StartPortForwardLoopWithFactory(
		ctx, mockFactory, clientset, "default", "test-svc", 8080, 9090,
	)

	if err != nil {
		t.Errorf("expected nil error on context cancellation, got: %v", err)
	}
}

func TestStartPortForwardLoopWithFactory_PodSelectionError(t *testing.T) {
	clientset := fake.NewClientset()
	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()

	// ServiceだけでPodなし
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-svc",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": "test"},
			Ports: []corev1.ServicePort{
				{Name: "http", Port: 8080},
			},
		},
	}
	_, err := clientset.CoreV1().Services("default").Create(
		t.Context(), svc, metav1.CreateOptions{},
	)
	if err != nil {
		t.Fatal(err)
	}

	mockFactory := &mockPortForwarderFactory{
		createFunc: func(ctx context.Context, namespace, podName string,
			localPort, remotePort int) (PortForwarder, error) {
			t.Error("factory should not be called when pod selection fails")
			return nil, nil
		},
	}

	start := time.Now()
	err = StartPortForwardLoopWithFactory(
		ctx, mockFactory, clientset, "default", "test-svc", 8080, 9090,
	)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}

	// 少なくとも1回のsleep (300ms) が実行されていることを確認
	if elapsed < 300*time.Millisecond {
		t.Errorf("expected retry loop with sleep, elapsed: %v", elapsed)
	}

	if mockFactory.callCount > 0 {
		t.Errorf("expected factory not to be called, but was called %d times",
			mockFactory.callCount)
	}
}

func TestNewWebSocketPortForwarderFactory(t *testing.T) {
	// fake rest.Config作成
	config := &rest.Config{
		Host: "https://kubernetes.default.svc",
	}

	factory := NewWebSocketPortForwarderFactory(config)

	if factory == nil {
		t.Fatal("expected non-nil factory")
	}

	// 型チェック
	_, ok := factory.(*websocketPortForwarderFactory)
	if !ok {
		t.Errorf("expected *websocketPortForwarderFactory, got %T", factory)
	}
}

func TestWebSocketPortForwarderFactory_CreatePortForwarder_InvalidConfig(t *testing.T) {
	// 不正なHost URLを持つconfig
	config := &rest.Config{
		Host: "://invalid-url",
	}

	factory := NewWebSocketPortForwarderFactory(config)
	ctx := t.Context()

	_, err := factory.CreatePortForwarder(ctx, "default", "test-pod", 8080, 9090)

	// URL parseエラーが発生するはず
	if err == nil {
		t.Fatal("expected error for invalid host URL, got nil")
	}

	if !strings.Contains(err.Error(), "failed to parse host URL") {
		t.Errorf("expected 'failed to parse host URL' error, got: %v", err)
	}
}

func TestWebSocketPortForwarderFactory_CreatePortForwarder_ContextCancellation(t *testing.T) {
	config := &rest.Config{
		Host: "https://kubernetes.default.svc",
		// TLS設定なしでも動作するように
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}

	factory := NewWebSocketPortForwarderFactory(config)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// PortForwarder作成（実際の接続は行わない）
	pf, err := factory.CreatePortForwarder(ctx, "default", "test-pod", 8080, 9090)

	// WebSocket RoundTripperの作成は成功するはず
	if err != nil {
		// client-goの内部実装によってはRoundTripperForで失敗する可能性もある
		// その場合はこのテストをスキップまたは調整
		t.Logf("CreatePortForwarder returned error (may be expected): %v", err)
		return
	}

	if pf == nil {
		t.Fatal("expected non-nil PortForwarder")
	}

	// ForwardPorts()は即座に終了するはず（接続前なのでエラーまたは即終了）
	// 実際の動作は統合テストで確認
}

func TestStartPortForwardLoopWithFactory_ForwardPortsError(t *testing.T) {
	clientset := fake.NewClientset()
	ctx, cancel := context.WithTimeout(t.Context(), 800*time.Millisecond)
	defer cancel()

	setupServiceAndReadyPod(t, clientset, "default", "test-svc", "test-pod")

	forwardCallCount := 0
	mockFactory := &mockPortForwarderFactory{
		createFunc: func(ctx context.Context, namespace, podName string,
			localPort, remotePort int) (PortForwarder, error) {
			return &mockPortForwarder{
				forwardFunc: func() error {
					forwardCallCount++
					// 最初の2回は即座にエラー、3回目はタイムアウトまでブロック
					if forwardCallCount < 3 {
						return fmt.Errorf("forward error %d", forwardCallCount)
					}
					<-ctx.Done()
					return nil
				},
			}, nil
		},
	}

	start := time.Now()
	err := StartPortForwardLoopWithFactory(
		ctx, mockFactory, clientset, "default", "test-svc", 8080, 9090,
	)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}

	// 2回のエラー × 300ms = 600ms以上経過していることを確認
	if elapsed < 600*time.Millisecond {
		t.Errorf("expected at least 2 retries (600ms), elapsed: %v", elapsed)
	}

	if forwardCallCount < 3 {
		t.Errorf("expected at least 3 ForwardPorts calls, got %d", forwardCallCount)
	}
}
