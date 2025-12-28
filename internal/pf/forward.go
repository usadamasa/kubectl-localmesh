package pf

import (
	"context"
	"fmt"
	"io"
	"net"
	"os/exec"
	"time"
)

func FreeLocalPort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// StartPortForwardLoop starts `kubectl port-forward` and auto-reconnects
func StartPortForwardLoop(
	ctx context.Context,
	namespace string,
	service string,
	localPort int,
	remotePort int,
) (*exec.Cmd, error) {

	script := fmt.Sprintf(`
set -e
while true; do
  kubectl -n %s port-forward svc/%s %d:%d >/dev/null 2>&1 || true
  sleep 0.3
done
`, namespace, service, localPort, remotePort)

	cmd := exec.CommandContext(ctx, "bash", "-lc", script)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func Terminate(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
	time.Sleep(50 * time.Millisecond)
}
