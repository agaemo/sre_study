# ハンズオン Phase 2：カスケード障害とサーキットブレーカー

## 扱う概念について

### カスケード障害（Cascade Failure）
1つのコンポーネントの障害が連鎖して他のコンポーネントにも波及する現象。

- 例：databaseが応答不能になる → backendがタイムアウトを待ち続ける → backendのスレッドが枯渇する → frontendへのレスポンスも遅延・失敗する
- 「データベースが落ちただけなのにフロントも落ちた」という障害の典型パターン

### サーキットブレーカー（Circuit Breaker）
電気回路のブレーカーと同じ発想で、**障害を検知したら接続を遮断し、連鎖を止める**パターン。

3つの状態を持つ：

```
Closed（正常）
  ↓ 一定回数失敗したらオープンになる
Open（遮断中）
  → 即座にエラーを返す（タイムアウトを待たない）
  ↓ 一定時間後にハーフオープンへ
Half-Open（試行中）
  → 1回リクエストを通して成功したらClosedに戻る
  → 失敗したらOpenに戻る
```

**なぜ有効か：** タイムアウトを待たずに即エラーを返すことで、スレッドの枯渇を防ぎカスケードを止める。

**このハンズオンでの実装について**

サーキットブレーカーはGoの標準ライブラリには含まれていない。このハンズオンでは `services/backend/main.go` に手動で実装している。`USE_CIRCUIT_BREAKER` という環境変数で有効/無効を切り替えられるようにしており、Step 2（無効）と Step 3（有効）の動作の違いを比較できるようにしている。

実際のプロダクションでは [resilience4j](https://resilience4j.readme.io/)（Java）や [go-breaker](https://github.com/sony/gobreaker)（Go）などのライブラリを使うことが多い。

**メリット・デメリット**

| | 内容 |
|---|---|
| メリット | 障害の連鎖を止めてシステム全体の生存率を上げる |
| メリット | タイムアウト待ちがなくなりリソース（スレッド・コネクション）の枯渇を防ぐ |
| メリット | 自動的に復旧を試みる（ハーフオープン）ため手動介入が不要 |
| デメリット | 閾値・タイムアウトの設定が難しい（厳しすぎると正常時も遮断される） |
| デメリット | エラーが即返るようになるため、クライアント側でのリトライ設計が必要になる |
| デメリット | 状態管理が複雑になり、デバッグ時に「なぜ503が返るのか」が分かりにくくなる |

### ポストモーテム（Postmortem）
障害発生後に「なぜ起きたか・どう防ぐか」を記録するドキュメント。

- 責任追及ではなく、**システムを改善するための振り返り**が目的
- 「なぜ？」を5回繰り返して根本原因（Root Cause）を掘り下げる
- SREの現場では障害のたびに書くことが文化として根付いている

## このハンズオンのサーバーについて

3つのサービスはすべて **Go** で実装されている（`services/*/main.go`）。

| サービス | 役割 | 実装のポイント |
|----------|------|----------------|
| frontend | ユーザーからのリクエストを受けてbackendに転送する | backendの応答をそのまま返す |
| backend | frontendからのリクエストを受けてdatabaseに問い合わせる | サーキットブレーカーの実装がある |
| database | 実際のDBの代わりに「OK」を返すだけのモックサーバー | 応答不能にすることでDBダウンを擬似再現する |

**サーキットブレーカーの実装**

サーキットブレーカーはGoの標準ライブラリには含まれていないため、`services/backend/main.go` に手動で実装している。`USE_CIRCUIT_BREAKER` 環境変数で有効/無効を切り替えられ、有効時は3回連続失敗でOpen状態になり、10秒後にハーフオープンになる。

---

## 構成

```
frontend（:8080）→ backend（:8081）→ database（:8082）
```

3層のサービスが連鎖している。databaseを応答不能にすると何が起きるかを観察する。

---

## Step 1：環境を起動する

ターミナルでリポジトリのルートに移動してから実行する：

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

### databaseを応答不能にする

```bash
docker compose pause database
```

> `stop` ではなく `pause` を使う。`stop` はコンテナをネットワークから切り離すためDNS解決が即座に失敗し、タイムアウトが発生しない。`pause` はコンテナをネットワークに残したまま処理を凍結するため、backendがタイムアウトまで待ち続ける状態を再現できる。

### frontendにリクエストを送り続ける

**新しいターミナルを開いて**以下を実行する：

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

環境変数を指定してbackendを再起動する（`docker-compose.yml` の編集は不要）：

```bash
USE_CIRCUIT_BREAKER=true docker compose up -d --build backend
```

databaseはまだ pause したままにしておく。

### リクエストを送る

**新しいターミナルを開いて**以下を実行する：

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
docker compose unpause database
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

`postmortem_template.md` を参考に、新しいファイルを作成して記入する：

```bash
cat postmortem_template.md  # テンプレートの構成を確認する
```

エディタで新規ファイルを作成する（例）：

```
postmortem_20260430.md
```

テンプレートの項目を埋めながら「なぜ？」を5回繰り返して根本原因を掘り下げる。書いたファイルはコミットしなくてよい。

---

## 環境を終了する

```bash
docker compose down
```

コンテナとネットワークが削除される。次回 `docker compose up -d --build` で再起動できる。

---

## 確認ポイント

- [ ] databaseが応答不能になるとfrontendまで遅くなることを体感した
- [ ] サーキットブレーカーがあるとタイムアウト待ちが発生しないことを確認した
- [ ] サーキットブレーカーのOpen→ハーフオープン→Closedの遷移を観察した
- [ ] ポストモーテムを1枚書いた
