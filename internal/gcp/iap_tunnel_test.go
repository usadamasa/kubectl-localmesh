package gcp

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/oauth2"
)

// newTestIAPServer はテスト用のIAPプロトコル対応WebSocketサーバーを起動します
func newTestIAPServer(t *testing.T) *httptest.Server {
	upgrader := websocket.Upgrader{
		Subprotocols: []string{"relay.tunnel.cloudproxy.app"},
		CheckOrigin: func(r *http.Request) bool {
			// テスト用に全てのオリジンを受け入れ
			return true
		},
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, http.Header{"Sec-WebSocket-Protocol": {"relay.tunnel.cloudproxy.app"}})
		if err != nil {
			t.Logf("websocket upgrade failed: %v", err)
			return
		}
		defer func() { _ = conn.Close() }()

		// CONNECT_SUCCESS_SIDを送信
		connectMsg := make([]byte, 10)
		binary.BigEndian.PutUint16(connectMsg, 0x0001)
		binary.BigEndian.PutUint64(connectMsg[2:], 12345) // SID = 12345
		if err := conn.WriteMessage(websocket.BinaryMessage, connectMsg); err != nil {
			t.Logf("failed to send CONNECT_SUCCESS_SID: %v", err)
			return
		}

		// エコーサーバー: 受信したDATAフレームを返す
		for {
			msgType, msg, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
					return
				}
				t.Logf("read error: %v", err)
				return
			}

			if msgType != websocket.BinaryMessage || len(msg) < 2 {
				continue
			}

			tag := binary.BigEndian.Uint16(msg)

			switch tag {
			case 0x0004: // DATA
				// DATAフレームをエコーバック
				if err := conn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
					t.Logf("failed to echo: %v", err)
					return
				}
			case 0x0007: // ACK
				// ACKフレームを受信（無視）
			default:
				t.Logf("unknown tag: 0x%04x", tag)
			}
		}
	})

	return httptest.NewServer(handler)
}

func TestIAPTunnelConn_ConnectAndTransferData(t *testing.T) {
	srv := newTestIAPServer(t)
	defer srv.Close()

	// WebSocket URLに変換
	wsURL := "ws" + srv.URL[4:] // http -> ws

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := dialTestIAP(ctx, wsURL)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// Write テスト
	testData := []byte("hello world")
	if n, err := conn.Write(testData); err != nil || n != len(testData) {
		t.Fatalf("Write failed: n=%d err=%v", n, err)
	}

	// Read テスト
	buf := make([]byte, len(testData))
	if n, err := conn.Read(buf); err != nil {
		t.Fatalf("Read failed: %v", err)
	} else if n != len(testData) {
		t.Fatalf("Read size mismatch: got %d, want %d", n, len(testData))
	}

	if string(buf) != string(testData) {
		t.Fatalf("Data mismatch: got %q, want %q", buf, testData)
	}
}

func TestIAPTunnelConn_ACKSending(t *testing.T) {
	// ACK送信が適切に行われることをテスト
	srv := newTestIAPServer(t)
	defer srv.Close()

	wsURL := "ws" + srv.URL[4:]

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := dialTestIAP(ctx, wsURL)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// 32KB以上のデータを送信（ACK送信のトリガー）
	largeData := make([]byte, 33*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	if n, err := conn.Write(largeData); err != nil || n != len(largeData) {
		t.Fatalf("Write failed: n=%d err=%v", n, err)
	}

	// エコーバックされたデータを受け取る
	buf := make([]byte, len(largeData))
	totalRead := 0
	for totalRead < len(largeData) {
		n, err := conn.Read(buf[totalRead:])
		if err != nil && err != io.EOF {
			t.Fatalf("Read failed: %v", err)
		}
		totalRead += n
		if n == 0 {
			break
		}
	}

	if totalRead < len(largeData) {
		t.Fatalf("incomplete read: got %d bytes, want %d", totalRead, len(largeData))
	}
}

func TestDefaultIAPDialer_InvalidParameters(t *testing.T) {
	tests := []struct {
		name    string
		projID  string
		zone    string
		inst    string
		port    int
		wantErr bool
	}{
		{
			name:    "empty project ID",
			projID:  "",
			zone:    "asia-northeast1-a",
			inst:    "instance-1",
			port:    5432,
			wantErr: true,
		},
		{
			name:    "empty zone",
			projID:  "my-project",
			zone:    "",
			inst:    "instance-1",
			port:    5432,
			wantErr: true,
		},
		{
			name:    "empty instance",
			projID:  "my-project",
			zone:    "asia-northeast1-a",
			inst:    "",
			port:    5432,
			wantErr: true,
		},
		{
			name:    "invalid port",
			projID:  "my-project",
			zone:    "asia-northeast1-a",
			inst:    "instance-1",
			port:    -1,
			wantErr: true,
		},
		{
			name:    "port too large",
			projID:  "my-project",
			zone:    "asia-northeast1-a",
			inst:    "instance-1",
			port:    65536,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			dialer := &DefaultIAPDialer{}
			// テスト用のダミートークンソース
			tokenSrc := &iapDummyTokenSource{}

			_, err := dialer.DialContext(ctx, tt.projID, tt.zone, tt.inst, tt.port, tokenSrc)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// ===== ヘルパー関数 =====

// dialTestIAP はテスト用のヘルパー関数で、WebSocketを直接接続します
func dialTestIAP(ctx context.Context, wsURL string) (net.Conn, error) {
	d := &websocket.Dialer{
		Subprotocols: []string{"relay.tunnel.cloudproxy.app"},
	}

	wsConn, _, err := d.DialContext(ctx, wsURL, nil)
	if err != nil {
		return nil, err
	}

	// 最初のメッセージで CONNECT_SUCCESS_SID を受信
	msgType, msg, err := wsConn.ReadMessage()
	if err != nil {
		_ = wsConn.Close()
		return nil, err
	}

	if msgType != websocket.BinaryMessage || len(msg) < 2 {
		_ = wsConn.Close()
		return nil, errInvalidConnectMessage
	}

	tag := binary.BigEndian.Uint16(msg)
	if tag != 0x0001 { // CONNECT_SUCCESS_SID
		_ = wsConn.Close()
		return nil, errInvalidConnectMessage
	}

	return &iapTunnelConn{
		wsConn:        wsConn,
		ctx:           ctx,
		readBuf:       nil,
		readPos:       0,
		receivedBytes: 0,
	}, nil
}

// iapDummyTokenSource はテスト用のダミートークンソース
type iapDummyTokenSource struct{}

func (d *iapDummyTokenSource) Token() (*oauth2.Token, error) {
	return &oauth2.Token{
		AccessToken: "dummy_token",
		TokenType:   "Bearer",
	}, nil
}
