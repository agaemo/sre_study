# ハンズオン：Prometheus + Grafana でメトリクス計測

## 使うツールについて

### Prometheus（プロメテウス）
アプリケーションやインフラの**メトリクス（数値データ）を収集・保存**するツール。
SREが「今何が起きているか」を数値で把握するための基盤。

- アプリが `/metrics` エンドポイントで公開した数値を、Prometheus が定期的に取りに行く（Pull型）
- 取得したデータは時系列で保存され、PromQL というクエリ言語で集計・分析できる
- 例：「1分間のリクエスト数」「エラー率」「レイテンシのP99」などを計算できる

### Grafana（グラファナ）
Prometheus などのデータソースと接続して**グラフ・ダッシュボードを作る**可視化ツール。

- Prometheus 単体でもグラフは見られるが、Grafana を使うと複数のグラフを1画面にまとめたり、アラートを設定したりできる
- SREの現場では「Grafana を見れば今のシステムの状態がわかる」状態を目指す

### このハンズオンの構成

```
[サンプルAPI] → /metrics を公開
     ↓ Prometheus が定期収集
[Prometheus] → データを保存・クエリに応答
     ↓ Grafana がデータを取得
[Grafana]    → グラフとして可視化
```

---

## このハンズオンのサーバーについて

計測対象のAPIサーバーは **Go** で実装されている（`api/main.go`）。Goを知らなくてもハンズオンは進められるが、何をしているサーバーかを理解しておくと観察結果が腑に落ちやすい。

**エンドポイントの構成**

| パス | 内容 |
|------|------|
| `/api/hello` | 5〜20ms でレスポンスを返す正常なエンドポイント |
| `/api/slow` | 200〜800ms かけてレスポンスを返す遅いエンドポイント |
| `/api/error` | 常に500エラーを返すエンドポイント |
| `/metrics` | Prometheusがメトリクスを収集しに来るエンドポイント |

**Prometheus用の実装**

Prometheusはアプリが `/metrics` で公開した数値を定期的に取りに来る。このサーバーでは以下の2つのメトリクスを記録している：

- `http_requests_total`：リクエスト数（パス・ステータスコード別）
- `http_request_duration_seconds`：レイテンシの分布（P50・P99の計算に使う）

リクエストのたびに `instrument()` 関数がこれらを自動で記録する仕組みになっている。

---

## 前提
- Docker Desktop がインストール済みであること

---

## Step 1：環境を起動する

ターミナルでリポジトリのルートに移動してから実行する：

```bash
cd labs/phase1
docker compose up -d
```

起動するコンテナ：
- Prometheus（メトリクス収集）: http://localhost:9090
- Grafana（可視化）: [http://localhost:3000/login](http://localhost:3000/login)（初期ログイン: admin/admin → ログイン後にパスワード変更が求められる。今回は adminadmin に変更）
- サンプルAPIサーバー（計測対象）: http://localhost:8080/api/hello

> `http://localhost:8080` のルートは404になるが正常。`/api/hello` など特定のパスにのみエンドポイントが存在する。

---

## Step 2：サンプルAPIに負荷をかける

**新しいターミナルを開いて**以下を実行する。これにより Prometheus に記録されるメトリクスが増え、Grafana のグラフに変化が現れる。

```bash
# 正常リクエスト（100回）
for i in $(seq 1 100); do curl -s http://localhost:8080/api/hello > /dev/null; done

# 意図的に遅いエンドポイントを叩く（20回）
# → レイテンシのグラフが跳ね上がる
for i in $(seq 1 20); do curl -s http://localhost:8080/api/slow > /dev/null; done

# エラーを発生させる（10回）
# → エラー率のグラフが上昇する
for i in $(seq 1 10); do curl -s http://localhost:8080/api/error > /dev/null; done
```

実行後、[Grafana](http://localhost:3000/login) を開くとグラフに変化が出ている。

---

## Step 3：Prometheusで確認する

http://localhost:9090 にアクセスし、画面上部の Expression バーにクエリを貼り付けて「Execute」を押すと結果が表示される。

### PromQL クエリの意味

```promql
# リクエスト成功率（SLI）
# 「全リクエストのうち2xx が返った割合」= サービスが正常に応答できている比率
# SLO（例：99.9%）と比較してエラーバジェットの消費を判断する
rate(http_requests_total{status=~"2.."}[1m]) / rate(http_requests_total[1m])
```

```promql
# P99レイテンシ
# 「上位1%を除いた最も遅いリクエストの応答時間」
# 「ほぼ全員がこの時間以内に応答を受け取っている」という指標
# 平均より外れ値の影響を受けにくく、SLO設定によく使われる
histogram_quantile(0.99, rate(http_request_duration_seconds_bucket[5m]))
```

```promql
# P50レイテンシ（中央値）
# 「ちょうど真ん中のユーザーが体験しているレイテンシ」
# P99 と比較することで「一部のユーザーだけが遅い」かどうかが分かる
histogram_quantile(0.50, rate(http_request_duration_seconds_bucket[5m]))
```

---

## Step 4：Grafanaでダッシュボードを作る

1. http://localhost:3000/login にアクセスし、adminadmin でログイン
2. Connections → Data sources → Add → **Prometheus** を選択（"Alertmanager" や "Prometheus AlertManager Datasource" ではないので注意）
3. URL に `http://prometheus:9090` を設定して保存
   - `localhost` ではなく `prometheus` と指定する（Docker ネットワーク内でのホスト名）
4. Dashboards → New → **New dashboard** → **+ Add visualization**
5. データソースに Prometheus を選択
6. 以下のパネル構成でクエリを追加する（詳細は下記参照）
7. 右上の **Save** を押す → ダイアログが開くので Title を `SRE ハンズオン Phase1` などに変えて **Save**

ダッシュボードに複数のグラフを並べることで、「エラー率」「レイテンシ」「リクエスト数」を一覧で監視できる状態になる。これが SRE の言う「オブザーバビリティ（可観測性）」の出発点。

---

### パネル1：リクエスト成功率

Query A に成功率のクエリを貼り付ける。

```promql
rate(http_requests_total{status=~"2.."}[1m]) / rate(http_requests_total[1m])
```

---

### パネル2：レイテンシ比較（P99 と P50）

- Query A に P99 のクエリを貼り付ける
- 画面下の **+ Add query** をクリックして Query B に P50 のクエリを追加する
- P50・P99 を同じパネルに並べると外れ値の影響が一目で分かる

> クエリ入力欄が Builder（GUI）モードになっている場合、PromQL を直接貼り付けられない。入力欄の右上にある **Code** ボタンをクリックして切り替える。

```promql
# Query A（P99）
histogram_quantile(0.99, rate(http_request_duration_seconds_bucket[5m]))

# Query B（P50）
histogram_quantile(0.50, rate(http_request_duration_seconds_bucket[5m]))
```

**グラフの読み方**

Y軸は秒単位。グラフはエンドポイントごとに自動で色分けされる。

- `/api/slow` の線が他より大きく上に出る → このエンドポイントだけ遅い
- P99（上の線）と P50（下の線）の差が大きいほど、遅いリクエストが一部に偏っている
- `/api/hello` の線はほぼ0に張り付く → 高速なエンドポイントはグラフ上で目立たない

「どのエンドポイントが遅いか」をパスごとに比較できるのがこのパネルの目的。

---

## Step 5：SLO違反を観察する

```bash
# 大量のエラーを発生させてSLO違反を起こす
for i in $(seq 1 50); do curl -s http://localhost:8080/api/error > /dev/null; done
```

Grafana の成功率グラフが 99.9% を下回る瞬間を確認する。
「この状態がエラーバジェットを消費している」という感覚を掴む。

---

## 環境を終了する

```bash
docker compose down
```

コンテナとネットワークが削除される。次回 `docker compose up -d` で再起動できる。

---

## 確認ポイント

- [ ] P50とP99の差を見て「外れ値がどれだけいるか」を読める
- [ ] エラー率が上がったときにグラフで即座に分かる
- [ ] 平均レイテンシとP99レイテンシが異なる値を示すことを確認した
