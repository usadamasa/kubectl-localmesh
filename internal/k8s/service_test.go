package k8s

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestResolveServicePort(t *testing.T) {
	tests := []struct {
		name          string
		namespace     string
		serviceName   string
		portName      string
		port          int
		service       *corev1.Service
		expectedPort  int
		expectedError string
	}{
		{
			name:         "ポート明示指定",
			namespace:    "default",
			serviceName:  "test-svc",
			port:         8080,
			expectedPort: 8080,
		},
		{
			name:        "ポート明示指定がportName指定より優先される",
			namespace:   "default",
			serviceName: "test-svc",
			portName:    "http",
			port:        9000,
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-svc",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{Name: "http", Port: 8080},
						{Name: "grpc", Port: 9090},
					},
				},
			},
			expectedPort: 9000,
		},
		{
			name:        "portName指定 - grpc",
			namespace:   "default",
			serviceName: "test-svc",
			portName:    "grpc",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-svc",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{Name: "http", Port: 8080},
						{Name: "grpc", Port: 9090},
					},
				},
			},
			expectedPort: 9090,
		},
		{
			name:        "portName指定 - http",
			namespace:   "users",
			serviceName: "users-api",
			portName:    "http",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "users-api",
					Namespace: "users",
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{Name: "http", Port: 8080},
						{Name: "grpc", Port: 9090},
						{Name: "metrics", Port: 9100},
					},
				},
			},
			expectedPort: 8080,
		},
		{
			name:        "デフォルト（ports[0]）",
			namespace:   "default",
			serviceName: "test-svc",
			portName:    "",
			port:        0,
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-svc",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{Name: "http", Port: 8080},
						{Name: "grpc", Port: 9090},
					},
				},
			},
			expectedPort: 8080,
		},
		{
			name:        "デフォルト（ports[0]） - 単一ポート",
			namespace:   "default",
			serviceName: "single-port-svc",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "single-port-svc",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{Port: 80},
					},
				},
			},
			expectedPort: 80,
		},
		{
			name:          "エラー: Serviceが存在しない",
			namespace:     "default",
			serviceName:   "nonexistent-svc",
			expectedError: "failed to get service",
		},
		{
			name:        "エラー: Service.Spec.Portsが空",
			namespace:   "default",
			serviceName: "no-ports-svc",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "no-ports-svc",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{},
				},
			},
			expectedError: "has no ports defined",
		},
		{
			name:        "エラー: 指定されたportNameが存在しない",
			namespace:   "default",
			serviceName: "test-svc",
			portName:    "nonexistent-port",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-svc",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{Name: "http", Port: 8080},
						{Name: "grpc", Port: 9090},
					},
				},
			},
			expectedError: "has no port named",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientset := fake.NewClientset()
			ctx := t.Context()

			if tt.service != nil {
				_, err := clientset.CoreV1().Services(tt.namespace).Create(
					ctx,
					tt.service,
					metav1.CreateOptions{},
				)
				if err != nil {
					t.Fatal(err)
				}
			}

			port, err := ResolveServicePort(
				ctx,
				clientset,
				tt.namespace,
				tt.serviceName,
				tt.portName,
				tt.port,
			)

			// アサーション
			if tt.expectedError != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.expectedError)
				}
				if !strings.Contains(err.Error(), tt.expectedError) {
					t.Fatalf("expected error containing %q, got %q", tt.expectedError, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if port != tt.expectedPort {
					t.Errorf("expected port %d, got %d", tt.expectedPort, port)
				}
			}
		})
	}
}
