# ハンズオン Phase 3：カナリアデプロイとグレースフルシャットダウン

## 扱う概念について

### カナリアデプロイ（Canary Deployment）
新バージョンを全ユーザーに一度に公開せず、**一部のトラフィックだけを新バージョンに流して問題がないか確認しながら段階的に切り替える**デプロイ手法。

- 炭鉱のカナリア（異常を早期検知するための鳥）が名前の由来
- 例：最初は10%だけv2に流し、エラーが出なければ50%→100%と拡大する
- 問題が起きてもすぐに0%に戻せるため、障害の影響範囲を最小化できる

### nginx（エンジンエックス）
このハンズオンでは**リバースプロキシ・ロードバランサー**として使う。

- ユーザーからのリクエストを受け取り、バックエンドの複数サーバーに振り分ける
- `weight` パラメータで振り分け比率を制御できる
- `nginx -s reload` で設定をリロードしてもダウンタイムが発生しない（処理中リクエストを中断しない）

### グレースフルシャットダウン（Graceful Shutdown）
プロセスを停止するとき、**処理中のリクエストが完了するのを待ってから終了する**仕組み。

```
通常の停止（グレースフルなし）:
SIGTERM → 即プロセス終了 → 処理中リクエストが途中で切断される → クライアントにエラー

グレースフルシャットダウン:
SIGTERM → 新規リクエストを受け付けない → 処理中リクエストが完了するのを待つ → プロセス終了
```

デプロイやスケールダウン時にユーザーへのエラーを出さないための必須テクニック。

## 構成

```
ユーザー → nginx（:8080）→ api-v1（weight=9, 90%）
                        ↘ api-v2（weight=1, 10%）← カナリア
```

nginxが重み付きロードバランシングでトラフィックを振り分ける。

---

## このハンズオンのサーバーについて

api-v1・api-v2 はともに **Go** で実装されたHTTPサーバーで（`services/api-v1/main.go`・`services/api-v2/main.go`）、レスポンスに含まれる `"version"` フィールドだけが異なる。

**グレースフルシャットダウンの実装**

GoのHTTPサーバーは何も実装しないと、停止時に処理中のリクエストが途中で切断される。このサーバーでは以下の実装でグレースフルシャットダウンを実現している：

```go
// SIGTERMを受け取るチャンネルを作る
quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

// シグナルが来るまでここで待機する（メインの処理はgoroutineで動いている）
sig := <-quit

// 新規リクエストを受け付けず、処理中のリクエストが終わるのを最大30秒待って終了
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
server.Shutdown(ctx)
```

`docker compose stop` を実行するとDockerがSIGTERMを送り、このコードが動く。ログに `shutting down gracefully...` → `shutdown complete` と表示されれば正常に動作している。

---

## Step 1：環境を起動する

ターミナルでリポジトリのルートに移動してから実行する：

```bash
cd labs/phase3
docker compose up -d --build
```

---

## Step 2：カナリアデプロイを観察する

**新しいターミナルを開いて**20回リクエストを送り、v1とv2の比率を確認する：

```bash
for i in $(seq 1 20); do
  curl -s http://localhost:8080/api/hello | python3 -m json.tool --no-ensure-ascii
done
```

### 観察ポイント

- `"version": "v1"` が約18回、`"version": "v2"` が約2回返る（10%カナリア）
- `X-Upstream` ヘッダーでどのサーバーに振られたかが分かる

```bash
# ヘッダーも確認する
curl -sv http://localhost:8080/api/hello 2>&1 | grep -E "version|X-Upstream"
```

---

## Step 3：カナリアの比率を変える

v2に問題がないことを確認したので、50%まで拡大する（`nginx.conf` の編集は不要）：

```bash
V1_WEIGHT=5 V2_WEIGHT=5 docker compose up -d nginx
```

再度20回リクエストを送り、比率が変わったことを確認する。

---

## Step 4：v2に問題が起きたときのロールバック

カナリア中にv2でエラーが発生したとする。

**本来のKubernetes環境でのロールバック：**
v2のPodをスケールアウト（`replicas: 0`）にするだけで、nginx相当のロードバランサが自動的にv2を除外し全トラフィックがv1に切り替わる。

**このハンズオン（nginx）での近似：**
nginxはコンテナが停止するとDNS解決に失敗してreloadできないため、コンテナを残したまま比率で擬似的にロールバックする：

```bash
V1_WEIGHT=999 V2_WEIGHT=1 docker compose up -d nginx
```

20回リクエストを送ると、ほぼすべてv1に戻ることを確認する。

> これは近似であり1/1000のリクエストはv2に流れる。完全な切り替えにはなっていないが、カナリアの「比率を戻す」操作の体験として捉える。

---

## Step 5：グレースフルシャットダウンを観察する

```bash
docker compose stop api-v1  # SIGTERMが送られる
```

コンテナのログを確認する：

```bash
docker compose logs api-v1
```

`received signal terminated, shutting down gracefully...` と表示されてから `shutdown complete` になっていることを確認する。

コードで実装した部分（`services/api-v1/main.go`）：

```go
signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
<-quit  // シグナルを待つ
server.Shutdown(ctx)  // 30秒以内に処理中リクエストが終わるのを待つ
```

---

## Step 6：v1 → v2 への完全切り替え（ゼロダウンタイム）

カナリアで問題がないことを確認できたので、v2の比率を極端に上げて完全切り替えを擬似的に再現する：

```bash
V1_WEIGHT=1 V2_WEIGHT=999 docker compose up -d nginx
```

全リクエストがほぼv2になったことを確認する。これで移行完了。

---

## 環境を終了する

```bash
docker compose down
```

コンテナとネットワークが削除される。次回 `docker compose up -d --build` で再起動できる。

---

## 確認ポイント

- [ ] 20リクエスト中、約2回v2に振られることを確認した（10%カナリア）
- [ ] nginx reload でダウンタイムなしに比率を変えられることを体感した
- [ ] `down` キーワードで即座にロールバックできることを確認した
- [ ] グレースフルシャットダウンのログを確認した
- [ ] v2への完全切り替えをダウンタイムなしで完了した
