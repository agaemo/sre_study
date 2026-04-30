# ハンズオン Phase 3：カナリアデプロイとグレースフルシャットダウン

## 構成

```
ユーザー → nginx（:8080）→ api-v1（weight=9, 90%）
                        ↘ api-v2（weight=1, 10%）← カナリア
```

nginxが重み付きロードバランシングでトラフィックを振り分ける。

---

## Step 1：環境を起動する

```bash
cd labs/phase3
docker compose up -d --build
```

---

## Step 2：カナリアデプロイを観察する

20回リクエストを送り、v1とv2の比率を確認する：

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

v2に問題がないことを確認したので、50%まで拡大する。

`nginx.conf` を編集する：

```nginx
upstream api_canary {
    server api-v1:8080 weight=5;  # 9 → 5
    server api-v2:8080 weight=5;  # 1 → 5
}
```

nginxをリロードする（ダウンタイムなし）：

```bash
docker compose exec nginx nginx -s reload
```

再度20回リクエストを送り、比率が変わったことを確認する。

---

## Step 4：v2に問題が起きたときのロールバック

カナリア中にv2でエラーが発生したとする。`nginx.conf` でv2を除外する：

```nginx
upstream api_canary {
    server api-v1:8080 weight=1;
    server api-v2:8080 weight=1 down;  # down をつけるとトラフィックを送らない
}
```

```bash
docker compose exec nginx nginx -s reload
```

全リクエストがv1に戻ることを確認する。再デプロイなし・ダウンタイムなしで即時ロールバックできる。

---

## Step 5：グレースフルシャットダウンを観察する

### 通常のシャットダウン（比較用）

```bash
# リクエストを送りながらコンテナを強制停止する
docker compose stop api-v1  # SIGTERMが送られる
```

コンテナのログを確認する：

```bash
docker compose logs api-v1
```

`received signal terminated, shutting down gracefully...` と表示されてから `shutdown complete` になっていることを確認する。

### グレースフルシャットダウンの意味

```
通常の停止（グレースフルなし）:
SIGTERM → 即プロセス終了 → 処理中リクエストが途中で切断される → クライアントにエラー

グレースフルシャットダウン（今回の実装）:
SIGTERM → 新規リクエストを受け付けない → 処理中のリクエストが完了するのを待つ → プロセス終了
```

コードで実装した部分（`services/api-v1/main.go`）：

```go
signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
<-quit  // シグナルを待つ
server.Shutdown(ctx)  // 30秒以内に処理中リクエストが終わるのを待つ
```

---

## Step 6：v1 → v2 への完全切り替え（ゼロダウンタイム）

カナリアで問題がないことを確認できたので、完全に切り替える。

`nginx.conf` を編集する：

```nginx
upstream api_canary {
    server api-v2:8080 weight=1;  # v1を削除してv2のみに
}
```

```bash
docker compose exec nginx nginx -s reload
```

全リクエストがv2になったことを確認する。これで移行完了。

---

## 確認ポイント

- [ ] 20リクエスト中、約2回v2に振られることを確認した（10%カナリア）
- [ ] nginx reload でダウンタイムなしに比率を変えられることを体感した
- [ ] `down` キーワードで即座にロールバックできることを確認した
- [ ] グレースフルシャットダウンのログを確認した
- [ ] v2への完全切り替えをダウンタイムなしで完了した
