package k8s

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
)

// PortForwarder forwards ports from local to remote pod.
type PortForwarder interface {
	ForwardPorts() error
}

// PortForwarderFactory creates PortForwarder instances.
type PortForwarderFactory interface {
	CreatePortForwarder(
		ctx context.Context,
		namespace, podName string,
		localPort, remotePort int,
	) (PortForwarder, error)
}

// websocketPortForwarderFactory implements PortForwarderFactory using WebSocket protocol.
type websocketPortForwarderFactory struct {
	config *rest.Config
}

// NewWebSocketPortForwarderFactory creates a new WebSocket-based PortForwarderFactory.
func NewWebSocketPortForwarderFactory(config *rest.Config) PortForwarderFactory {
	return &websocketPortForwarderFactory{config: config}
}

// CreatePortForwarder creates a new PortForwarder instance using WebSocket protocol.
func (f *websocketPortForwarderFactory) CreatePortForwarder(
	ctx context.Context,
	namespace, podName string,
	localPort, remotePort int,
) (PortForwarder, error) {
	// Pod port-forward用のURL構築
	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", namespace, podName)
	serverURL, err := url.Parse(f.config.Host)
	if err != nil {
		return nil, fmt.Errorf("failed to parse host URL: %w", err)
	}
	serverURL.Path = path

	// WebSocket dialerを作成（client-go v0.30+の新しいAPI）
	dialer, err := portforward.NewSPDYOverWebsocketDialer(serverURL, f.config)
	if err != nil {
		return nil, fmt.Errorf("failed to create WebSocket dialer: %w", err)
	}

	// ポート仕様（"localPort:remotePort"形式）
	ports := []string{fmt.Sprintf("%d:%d", localPort, remotePort)}

	// stopChanとreadyChanの準備
	stopChan := make(chan struct{}, 1)
	readyChan := make(chan struct{})

	// contextキャンセル時にstopChanをクローズ
	go func() {
		<-ctx.Done()
		close(stopChan)
	}()

	// PortForwarder作成
	pf, err := portforward.New(
		dialer,
		ports,
		stopChan,
		readyChan,
		io.Discard, // stdout
		io.Discard, // stderr
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create port forwarder: %w", err)
	}

	return pf, nil
}

// StartPortForwardLoop starts port-forwarding with automatic reconnection.
// It continuously forwards localPort to remotePort on the specified service,
// retrying every 300ms on disconnection or error.
// The loop exits when ctx is cancelled.
func StartPortForwardLoop(
	ctx context.Context,
	config *rest.Config,
	clientset kubernetes.Interface,
	namespace, serviceName string,
	localPort, remotePort int,
) error {
	factory := NewWebSocketPortForwarderFactory(config)
	return StartPortForwardLoopWithFactory(
		ctx, factory, clientset, namespace, serviceName, localPort, remotePort,
	)
}

// StartPortForwardLoopWithFactory starts port-forwarding with automatic reconnection
// using a custom PortForwarderFactory. This function is designed for testability.
func StartPortForwardLoopWithFactory(
	ctx context.Context,
	factory PortForwarderFactory,
	clientset kubernetes.Interface,
	namespace, serviceName string,
	localPort, remotePort int,
) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		// Pod名を取得
		podName, err := selectPodForService(ctx, clientset, namespace, serviceName)
		if err != nil {
			// エラー時は0.3秒待って再試行
			time.Sleep(300 * time.Millisecond)
			continue
		}

		// PortForwarder作成
		pf, err := factory.CreatePortForwarder(ctx, namespace, podName, localPort, remotePort)
		if err != nil {
			// エラー時は0.3秒待って再試行
			time.Sleep(300 * time.Millisecond)
			continue
		}

		// ForwardPorts実行（ブロッキング）
		// エラーまたは切断時は下記のsleepの後に再試行される
		_ = pf.ForwardPorts()

		// contextキャンセル時は正常終了
		if ctx.Err() != nil {
			return nil
		}

		// エラーまたは切断時は0.3秒待って再接続
		time.Sleep(300 * time.Millisecond)
	}
}

// selectPodForService は、Serviceのselectorに基づいてReady状態のPodを選択する。
// kubectl port-forward svc/xxxと同じロジックを実装。
func selectPodForService(
	ctx context.Context,
	clientset kubernetes.Interface,
	namespace, serviceName string,
) (string, error) {
	// 1. Serviceを取得してselectorを取得
	svc, err := clientset.CoreV1().Services(namespace).Get(
		ctx,
		serviceName,
		metav1.GetOptions{},
	)
	if err != nil {
		return "", fmt.Errorf("failed to get service %s/%s: %w", namespace, serviceName, err)
	}

	// 2. selectorが空の場合はエラー
	if len(svc.Spec.Selector) == 0 {
		return "", fmt.Errorf("service %s/%s has no selector", namespace, serviceName)
	}

	// 3. selectorをラベルセレクタに変換
	selector := labels.SelectorFromSet(svc.Spec.Selector)

	// 4. Podリストを取得
	pods, err := clientset.CoreV1().Pods(namespace).List(
		ctx,
		metav1.ListOptions{
			LabelSelector: selector.String(),
		},
	)
	if err != nil {
		return "", fmt.Errorf("failed to list pods for service %s/%s: %w", namespace, serviceName, err)
	}

	// 5. Podが見つからない場合はエラー
	if len(pods.Items) == 0 {
		return "", fmt.Errorf("no pods found for service %s/%s with selector %v",
			namespace, serviceName, svc.Spec.Selector)
	}

	// 6. Ready状態のPodを優先的に選択
	for _, pod := range pods.Items {
		if isPodReady(&pod) {
			return pod.Name, nil
		}
	}

	// 7. Ready状態のPodがない場合は最初のPodを返す（kubectlの動作と同じ）
	return pods.Items[0].Name, nil
}

// isPodReady は、PodがReady状態かどうかを判定する。
func isPodReady(pod *corev1.Pod) bool {
	// Podのフェーズチェック
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}

	// Conditionsチェック
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady {
			return cond.Status == corev1.ConditionTrue
		}
	}
	return false
}
