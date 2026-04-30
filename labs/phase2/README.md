# ハンズオン Phase 2：カスケード障害とサーキットブレーカー

## 構成

```
frontend（:8080）→ backend（:8081）→ database（:8082）
```

3層のサービスが連鎖している。databaseを落とすと何が起きるかを観察する。

---

## Step 1：環境を起動する

```bash
cd labs/phase2
docker compose up -d --build
```

正常動作を確認する：

```bash
curl http://localhost:8080/
# → {"data":"hello from backend", "duration_ms":"XXms", "status":"200"}
```

---

## Step 2：カスケード障害を観察する（サーキットブレーカーなし）

`docker-compose.yml` の `USE_CIRCUIT_BREAKER=false`（デフォルト）の状態で実行する。

### databaseを落とす

```bash
docker compose stop database
```

### frontendにリクエストを送り続ける

```bash
for i in $(seq 1 20); do
  echo -n "[$i] "
  curl -s http://localhost:8080/ | python3 -m json.tool --no-ensure-ascii
  sleep 0.5
done
```

### 観察ポイント

- `status: 502` が返り始める（backendがdatabaseに繋がれない）
- `duration_ms` がタイムアウトの2000msに近づく（タイムアウトまで待ち続けている）
- frontendのリクエストもすべて遅くなる（databaseの障害がfrontendまで伝播している）

**これがカスケード障害。** databaseの障害がbackend → frontendと連鎖した。

---

## Step 3：サーキットブレーカーを有効にする

`docker-compose.yml` を編集する：

```yaml
backend:
  environment:
    - USE_CIRCUIT_BREAKER=true  # falseからtrueに変更
```

backendを再起動する：

```bash
docker compose up -d --build backend
```

databaseはまだ停止したままにしておく。

### リクエストを送る

```bash
for i in $(seq 1 20); do
  echo -n "[$i] "
  curl -s http://localhost:8080/ | python3 -m json.tool --no-ensure-ascii
  sleep 0.5
done
```

### 観察ポイント

- 最初の数回は `502`（databaseに繋ごうとしている）
- **3回失敗するとサーキットがオープンになる**
- それ以降は即座に `503`（circuit breaker open）が返る
- `duration_ms` が激減する（タイムアウトを待たなくなった）

**サーキットブレーカーの効果：**
- タイムアウト2秒の待ちがなくなり、frontendのスレッドが詰まらない
- databaseの障害がbackendで止まり、frontendへの伝播が軽減される

---

## Step 4：databaseを復旧させる

```bash
docker compose start database
```

10秒後（サーキットブレーカーのタイムアウト）にハーフオープンになり、リクエストが通り始める：

```bash
for i in $(seq 1 10); do
  echo -n "[$i] "
  curl -s http://localhost:8080/ | python3 -m json.tool --no-ensure-ascii
  sleep 1
done
```

`circuit_breaker: closed` に戻ったら復旧完了。

---

## Step 5：ポストモーテムを書く

今回のカスケード障害を題材に、ポストモーテムを書いてみる。

```bash
cp postmortem_template.md postmortem_$(date +%Y%m%d).md
```

テンプレートを埋めながら「なぜ？」を5回繰り返して根本原因を掘り下げる。

---

## 確認ポイント

- [ ] databaseを落とすとfrontendまで遅くなることを体感した
- [ ] サーキットブレーカーがあるとタイムアウト待ちが発生しないことを確認した
- [ ] サーキットブレーカーのOpen→ハーフオープン→Closedの遷移を観察した
- [ ] ポストモーテムを1枚書いた
