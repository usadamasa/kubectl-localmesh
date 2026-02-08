# IAP TCP Tunnel v4 プロトコル詳細

`internal/gcp/iap_tunnel.go` の実装に関する詳細な仕様リファレンス。

> このドキュメントはIAP TCP Tunnelの深い理解が必要な場合に参照してください。
> 通常のトラブルシューティングにはSKILL.mdのユースケース別ワークフローで十分です。

## WebSocket接続先URL

```
wss://tunnel.cloudproxy.app/v4/connect?project=XXX&zone=XXX&instance=XXX&interface=nic0&port=22&newWebsocket=true&_=TIMESTAMP-RANDOM
```

### URLパラメータ

| パラメータ | 値 | 説明 |
|-----------|-----|------|
| `project` | GCPプロジェクトID | 接続先のプロジェクト |
| `zone` | ゾーン名 | Compute Instanceのゾーン |
| `instance` | インスタンス名 | Bastion Instanceの名前 |
| `interface` | `nic0` | ネットワークインターフェース(固定) |
| `port` | `22` | SSHポート(固定) |
| `newWebsocket` | `true` | v4プロトコル使用フラグ |
| `_` | `TIMESTAMP-RANDOM` | キャッシュバスター |

## HTTPヘッダー

```
Authorization: Bearer <OAuth2 access token>
Origin: bot:iap-tunneler
Sec-WebSocket-Protocol: relay.tunnel.cloudproxy.app
```

- `Authorization`: ADCから取得したOAuth2アクセストークン
- `Origin`: IAP tunnelerを識別する固定値
- `Sec-WebSocket-Protocol`: WebSocketサブプロトコル(gorilla/websocketの`Subprotocols`で設定)

## バイナリフレーミング仕様

### フレーム構造

すべてのフレームはBigEndianエンコーディング。

### TAG定義

| タグ (2バイト) | 名前 | フレーム構造 | 説明 |
|---------------|------|------------|------|
| `0x0001` | CONNECT_SUCCESS_SID | Tag(2) + SID(可変) | 接続成功通知。最初のメッセージで受信 |
| `0x0004` | DATA | Tag(2) + Length(4) + Payload(n) | データ転送。最大ペイロード16KB |
| `0x0007` | ACK | Tag(2) + TotalBytesReceived(8) | 受信確認。1MB受信ごとに送信 |

### CONNECT_SUCCESS_SID (0x0001)

WebSocket接続確立直後に受信する最初のメッセージ。
このメッセージを受信するまでデータの送受信はできない。

```
[0x00 0x01] [SID bytes...]
```

### DATA (0x0004)

```
[0x00 0x04] [Length: 4バイト BigEndian] [Payload: Lengthバイト]
```

- 最大ペイロードサイズ: **16KB** (`maxDataFrameSize = 16 * 1024`)
- 16KBを超えるデータは複数のDATAフレームに分割して送信

### ACK (0x0007)

```
[0x00 0x07] [TotalBytesReceived: 8バイト BigEndian]
```

- ACK送信閾値: **1MB** (`ackThreshold = 1024 * 1024`)
- 累計受信バイト数が前回ACK送信時から1MB増加するごとに送信

## net.Conn実装 (`iapTunnelConn`)

`iapTunnelConn` は `net.Conn` インターフェースを実装し、WebSocket上のバイナリフレーミングを透過的なTCPストリームに変換する。

### フィールド

| フィールド | 型 | 用途 |
|-----------|-----|------|
| `wsConn` | `*websocket.Conn` | 基盤となるWebSocket接続 |
| `ctx` | `context.Context` | キャンセル制御 |
| `readMu` | `sync.Mutex` | ReadMessage操作の排他制御 |
| `writeMu` | `sync.Mutex` | WriteMessage操作の排他制御 |
| `readBuf` | `[]byte` | DATAフレームの未読バイトバッファ |
| `readPos` | `int` | readBuf内の現在読み取り位置 |
| `receivedBytes` | `uint64` | 受信した累計バイト数 |
| `lastAckSent` | `uint64` | 最後にACK送信した時点の累計バイト数 |

### Read操作

1. `readBuf`に未読データがあればそこから返す
2. なければ新しいDATAフレームを受信
3. DATAフレーム以外(ACK等)は無視してループ
4. 受信バイト数を更新し、ACK閾値チェック

### Write操作

1. 入力データを16KB以下のチャンクに分割
2. 各チャンクをDATAフレームとしてエンコード
3. WebSocketバイナリメッセージとして送信

### mutex設計

gorilla/websocketは1つのconcurrent reader + 1つのconcurrent writerをサポートする。
そのためread/writeで**別々のmutex**を使用している。

```
readMu  → ReadMessage操作を保護
writeMu → WriteMessage操作を保護
```

**ロック順序: `readMu` → `writeMu`**

ACK送信は`Read`メソッド内(`readMu`保持中)から`maybeSendACK` → `sendACK`を呼び出す。
`sendACK`は内部で`writeMu`を取得する。

このロック順序が逆転するコードパスは存在しないため、デッドロックは発生しない。

### 16KB分割送信の設計理由

IAP Tunnel v4プロトコルは1フレームあたり最大16KBのペイロードを許容する。
これはGCPのIAPサーバー側の制約に合わせたもので、超過するとフレームが拒否される。

### 1MB ACK閾値の設計理由

ACKを頻繁に送信するとオーバーヘッドが増加する一方、送信しないとIAPサーバーが接続を切断する。
1MBは実用上のバランスポイントとして採用されている。

## 関連ファイル

| ファイル | 内容 |
|---------|------|
| `internal/gcp/iap_tunnel.go` | IAP TCP Tunnel v4実装 |
| `internal/gcp/iap_tunnel_test.go` | テスト |
