package k8s

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// NewClient creates a new Kubernetes client using the default kubeconfig rules.
// It follows the same discovery order as kubectl:
// 1. $KUBECONFIG environment variable
// 2. ~/.kube/config
// Uses the current-context from the kubeconfig.
func NewClient() (*kubernetes.Clientset, *rest.Config, error) {
	// kubeconfigのロードルール
	// clientcmd.NewDefaultClientConfigLoadingRules() は以下の順序で検索:
	// 1. $KUBECONFIG環境変数で指定されたパス
	// 2. ~/.kube/config（デフォルト）
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()

	// current-contextを使用してRESTConfigを作成
	configOverrides := &clientcmd.ConfigOverrides{}

	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		configOverrides,
	)

	// RESTConfig取得
	restConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, nil, err
	}

	// clientset作成
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, nil, err
	}

	return clientset, restConfig, nil
}
