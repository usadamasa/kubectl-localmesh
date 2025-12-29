package k8s

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ResolveServicePort resolves the service port based on the provided parameters.
// Priority:
// 1. If port is explicitly specified (non-zero), return it
// 2. If portName is specified, find the port by name in service.Spec.Ports
// 3. Otherwise, return the first port (service.Spec.Ports[0])
func ResolveServicePort(
	ctx context.Context,
	clientset kubernetes.Interface,
	namespace, serviceName, portName string,
	port int,
) (int, error) {
	// 明示的なport指定があればそれを返す
	if port != 0 {
		return port, nil
	}

	// Serviceを取得
	svc, err := clientset.CoreV1().Services(namespace).Get(
		ctx,
		serviceName,
		metav1.GetOptions{},
	)
	if err != nil {
		return 0, fmt.Errorf("failed to get service %s/%s: %w", namespace, serviceName, err)
	}

	// Serviceにポートが定義されていない場合
	if len(svc.Spec.Ports) == 0 {
		return 0, fmt.Errorf("service %s/%s has no ports defined", namespace, serviceName)
	}

	// portName指定がある場合: svc.Spec.Portsから該当するポートを検索
	if strings.TrimSpace(portName) != "" {
		for _, p := range svc.Spec.Ports {
			if p.Name == portName {
				return int(p.Port), nil
			}
		}
		// 該当するポートが見つからない場合
		return 0, fmt.Errorf("service %s/%s has no port named '%s'", namespace, serviceName, portName)
	}

	// portName指定がない場合: svc.Spec.Ports[0]を返す
	return int(svc.Spec.Ports[0].Port), nil
}
