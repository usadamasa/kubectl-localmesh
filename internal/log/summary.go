package log

import (
	"fmt"
	"strings"

	"github.com/usadamasa/kubectl-localmesh/internal/port"
)

// ServiceSummary はサービスのサマリー情報を表します。
type ServiceSummary struct {
	// Host はローカルアクセス用のホスト名
	Host string
	// Protocol はプロトコル（http, grpc, tcp）
	Protocol string
	// DisplayType は表示用のタイプ（HTTP/gRPC, TCP）
	DisplayType string
	// Backend はバックエンドの情報
	Backend string
	// ListenPort はリスナーポート（0の場合はデフォルトを使用）
	ListenPort port.ListenerPort
}

// EffectiveListenPort はListenPortが0の場合はデフォルトポートを返します。
func (s ServiceSummary) EffectiveListenPort(defaultPort port.ListenerPort) port.ListenerPort {
	if s.ListenPort > 0 {
		return s.ListenPort
	}
	return defaultPort
}

// formatProtocolLabel はプロトコル名を表示用にフォーマットします。
func formatProtocolLabel(protocol string) string {
	switch protocol {
	case "grpc":
		return "gRPC"
	case "http":
		return "HTTP"
	case "http2":
		return "HTTP/2"
	default:
		return strings.ToUpper(protocol)
	}
}

// GenerateSummary はサービスメッシュ起動完了時のサマリーを生成します。
func GenerateSummary(services []ServiceSummary, listenerPort port.ListenerPort) string {
	var sb strings.Builder

	sb.WriteString("\nService Mesh is ready!\n\n")
	sb.WriteString("Access your services:\n")

	// HTTP/gRPCサービスとTCPサービスを分類
	var httpServices, tcpServices []ServiceSummary
	for _, svc := range services {
		if svc.Protocol == "tcp" {
			tcpServices = append(tcpServices, svc)
		} else {
			httpServices = append(httpServices, svc)
		}
	}

	// HTTP/gRPCサービスのセクション
	if len(httpServices) > 0 {
		sb.WriteString("  HTTP/gRPC Services:\n")
		for _, svc := range httpServices {
			p := svc.EffectiveListenPort(listenerPort)
			protocolLabel := formatProtocolLabel(svc.Protocol)
			fmt.Fprintf(&sb, "  • http://%s:%d (%s) -> %s\n",
				svc.Host, p, protocolLabel, svc.Backend)
		}
		sb.WriteString("\n")
	}

	// TCPサービスのセクション
	if len(tcpServices) > 0 {
		sb.WriteString("  TCP Services:\n")
		for _, svc := range tcpServices {
			p := svc.EffectiveListenPort(listenerPort)
			fmt.Fprintf(&sb, "  • tcp://%s:%d -> %s\n",
				svc.Host, p, svc.Backend)
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Press Ctrl+C to stop and cleanup.\n")

	return sb.String()
}
