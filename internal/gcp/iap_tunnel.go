package gcp

import (
	"context"
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/oauth2"
)

const (
	// IAP Tunnel v4 バイナリフレーミングプロトコルのタグ
	tagConnectSuccessSID = 0x0001
	tagData              = 0x0004
	tagACK               = 0x0007

	// DATAフレームの最大ペイロードサイズ (16KB)
	maxDataFrameSize = 16 * 1024

	// ACK送信の閾値: 1MB受信ごと
	ackThreshold = 1024 * 1024

	// IAP Tunnel接続のOriginヘッダー
	iapOrigin = "bot:iap-tunneler"
)

var errInvalidConnectMessage = fmt.Errorf("invalid CONNECT_SUCCESS_SID message")

// IAPDialer はIAP TCP Tunnelプロトコルを使用してGCP Compute InstanceへのTCP接続を確立します
type IAPDialer interface {
	DialContext(ctx context.Context, projectID, zone, instance string, port int, ts oauth2.TokenSource) (net.Conn, error)
}

// DefaultIAPDialer はIAPDialerインターフェースの標準実装です
type DefaultIAPDialer struct{}

// DialContext はIAP TCP Tunnelプロトコルを使用してGCP Compute Instanceに接続します
func (d *DefaultIAPDialer) DialContext(ctx context.Context, projectID, zone, instance string, port int, ts oauth2.TokenSource) (net.Conn, error) {
	// パラメータバリデーション
	if projectID == "" {
		return nil, fmt.Errorf("project ID is empty")
	}
	if zone == "" {
		return nil, fmt.Errorf("zone is empty")
	}
	if instance == "" {
		return nil, fmt.Errorf("instance is empty")
	}
	if port <= 0 || port > 65535 {
		return nil, fmt.Errorf("invalid port: %d", port)
	}

	// OAuth2トークンを取得
	token, err := ts.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to get OAuth2 token: %w", err)
	}

	// WebSocket接続先URL
	params := url.Values{}
	params.Set("project", projectID)
	params.Set("zone", zone)
	params.Set("instance", instance)
	params.Set("interface", "nic0")
	params.Set("port", strconv.Itoa(port))
	params.Set("newWebsocket", "true")
	params.Set("_", fmt.Sprintf("%d-%d", time.Now().UnixNano(), rand.Int63())) //nolint:gosec // cache buster
	wsURL := fmt.Sprintf("wss://tunnel.cloudproxy.app/v4/connect?%s", params.Encode())

	// WebSocket接続
	headers := http.Header{
		"Authorization": []string{fmt.Sprintf("Bearer %s", token.AccessToken)},
		"Origin":        []string{iapOrigin},
	}

	dialer := &websocket.Dialer{
		Subprotocols: []string{"relay.tunnel.cloudproxy.app"},
	}

	wsConn, _, err := dialer.DialContext(ctx, wsURL, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to IAP tunnel: %w", err)
	}

	// 最初のメッセージで CONNECT_SUCCESS_SID を受信
	msgType, msg, err := wsConn.ReadMessage()
	if err != nil {
		_ = wsConn.Close()
		return nil, fmt.Errorf("failed to read CONNECT_SUCCESS_SID: %w", err)
	}

	if msgType != websocket.BinaryMessage || len(msg) < 2 {
		_ = wsConn.Close()
		return nil, errInvalidConnectMessage
	}

	tag := binary.BigEndian.Uint16(msg)
	if tag != tagConnectSuccessSID {
		_ = wsConn.Close()
		return nil, fmt.Errorf("expected CONNECT_SUCCESS_SID (0x0001), got 0x%04x", tag)
	}

	return &iapTunnelConn{
		wsConn:        wsConn,
		ctx:           ctx,
		readBuf:       nil,
		readPos:       0,
		receivedBytes: 0,
	}, nil
}

// iapTunnelConn はnet.Connインターフェースを実装するIAP Tunnel接続ラッパーです
// gorilla/websocketは1つのconcurrent reader + 1つのconcurrent writerをサポートするため、
// read/writeで別々のmutexを使用する。
type iapTunnelConn struct {
	wsConn        *websocket.Conn
	ctx           context.Context //nolint:containedctx // net.Conn実装のためコンテキストを保持
	readMu        sync.Mutex      // ReadMessage操作を保護
	writeMu       sync.Mutex      // WriteMessage操作を保護
	readBuf       []byte          // DATAフレームの残りバイト
	readPos       int             // readBuf内の現在位置
	receivedBytes uint64          // 受信した累計バイト数（ACK送信用）
	lastAckSent   uint64          // 最後にACK送信した時点の累計バイト数
}

// Read はDATAフレームからペイロードを読み取ります
func (c *iapTunnelConn) Read(b []byte) (n int, err error) {
	c.readMu.Lock()
	defer c.readMu.Unlock()

	if len(b) == 0 {
		return 0, nil
	}

	// readBufに残りデータがあれば、それを返す
	if c.readBuf != nil && c.readPos < len(c.readBuf) {
		n := copy(b, c.readBuf[c.readPos:])
		c.readPos += n
		c.receivedBytes += uint64(n)

		if err := c.maybeSendACK(); err != nil {
			return 0, err
		}

		return n, nil
	}

	// 新しいフレームを読む
	for {
		msgType, msg, err := c.wsConn.ReadMessage()
		if err != nil {
			return 0, err
		}

		if msgType != websocket.BinaryMessage || len(msg) < 6 {
			continue // 無効なメッセージは無視（DATAフレームは最低6バイト）
		}

		tag := binary.BigEndian.Uint16(msg[0:2])

		if tag == tagData {
			// DATAフレーム: Tag(2) + Length(4) + Payload(n)
			dataLen := binary.BigEndian.Uint32(msg[2:6])
			if int(dataLen) > len(msg)-6 {
				continue // フレームが不完全
			}
			payload := msg[6 : 6+dataLen]
			n := copy(b, payload)
			c.readBuf = payload
			c.readPos = n
			c.receivedBytes += uint64(n)

			if err := c.maybeSendACK(); err != nil {
				return 0, err
			}

			return n, nil
		}
		// ACK等のその他のタグは無視
	}
}

// Write はDATAフレームとしてペイロードを書き込みます
// 16KBを超えるデータは複数フレームに分割して送信します
func (c *iapTunnelConn) Write(b []byte) (n int, err error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	if len(b) == 0 {
		return 0, nil
	}

	remaining := b
	for len(remaining) > 0 {
		chunkSize := len(remaining)
		if chunkSize > maxDataFrameSize {
			chunkSize = maxDataFrameSize
		}
		chunk := remaining[:chunkSize]

		// DATAフレーム: Tag(2) + Length(4) + Payload(n)
		frame := make([]byte, 6+chunkSize)
		binary.BigEndian.PutUint16(frame[0:2], tagData)
		binary.BigEndian.PutUint32(frame[2:6], uint32(chunkSize))
		copy(frame[6:], chunk)

		if err := c.wsConn.WriteMessage(websocket.BinaryMessage, frame); err != nil {
			return n, err
		}

		n += chunkSize
		remaining = remaining[chunkSize:]
	}

	return n, nil
}

// Close はWebSocket接続をクローズします
func (c *iapTunnelConn) Close() error {
	return c.wsConn.Close()
}

// LocalAddr はダミーアドレスを返します
func (c *iapTunnelConn) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.IP{127, 0, 0, 1}, Port: 0}
}

// RemoteAddr はダミーアドレスを返します
func (c *iapTunnelConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.IP{127, 0, 0, 1}, Port: 0}
}

// SetDeadline はWebSocket側にデリゲートします
func (c *iapTunnelConn) SetDeadline(t time.Time) error {
	if err := c.wsConn.SetReadDeadline(t); err != nil {
		return err
	}
	return c.wsConn.SetWriteDeadline(t)
}

// SetReadDeadline はWebSocket側にデリゲートします
func (c *iapTunnelConn) SetReadDeadline(t time.Time) error {
	return c.wsConn.SetReadDeadline(t)
}

// SetWriteDeadline はWebSocket側にデリゲートします
func (c *iapTunnelConn) SetWriteDeadline(t time.Time) error {
	return c.wsConn.SetWriteDeadline(t)
}

// maybeSendACK は受信バイト数がACK閾値を超えた場合にACKを送信します。
// readMuの中から呼ばれるため、writeMuを内部で取得する。
func (c *iapTunnelConn) maybeSendACK() error {
	if c.receivedBytes-c.lastAckSent >= ackThreshold {
		if err := c.sendACK(c.receivedBytes); err != nil {
			return fmt.Errorf("failed to send ACK: %w", err)
		}
		c.lastAckSent = c.receivedBytes
	}
	return nil
}

// sendACK はACKフレームを送信します（内部メソッド）
// writeMuを内部で取得するため、呼び出し元はwriteMuを保持していないこと。
func (c *iapTunnelConn) sendACK(totalBytes uint64) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	// ACKフレーム: Tag(2) + TotalBytesReceived(8) = 10バイト
	frame := make([]byte, 10)
	binary.BigEndian.PutUint16(frame[0:2], tagACK)
	binary.BigEndian.PutUint64(frame[2:10], totalBytes)

	return c.wsConn.WriteMessage(websocket.BinaryMessage, frame)
}
