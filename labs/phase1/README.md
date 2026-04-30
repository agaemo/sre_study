# ハンズオン：Prometheus + Grafana でメトリクス計測

## 前提
- Docker Desktop がインストール済みであること

---

## Step 1：環境を起動する

```bash
cd labs/phase1
docker compose up -d
```

起動するコンテナ：
- Prometheus（メトリクス収集）: http://localhost:9090
- Grafana（可視化）: http://localhost:3000（admin/admin）
- サンプルAPIサーバー（計測対象）: http://localhost:8080

---

## Step 2：サンプルAPIに負荷をかける

```bash
# 正常リクエスト
for i in $(seq 1 100); do curl -s http://localhost:8080/api/hello > /dev/null; done

# 意図的に遅いエンドポイントを叩く
for i in $(seq 1 20); do curl -s http://localhost:8080/api/slow > /dev/null; done

# エラーを発生させる
for i in $(seq 1 10); do curl -s http://localhost:8080/api/error > /dev/null; done
```

---

## Step 3：Prometheusで確認する

http://localhost:9090 にアクセスして以下のクエリを試す：

```promql
# リクエスト成功率（SLI）
rate(http_requests_total{status=~"2.."}[1m]) / rate(http_requests_total[1m])

# P99レイテンシ
histogram_quantile(0.99, rate(http_request_duration_seconds_bucket[5m]))

# P50レイテンシ
histogram_quantile(0.50, rate(http_request_duration_seconds_bucket[5m]))
```

---

## Step 4：Grafanaでダッシュボードを作る

1. http://localhost:3000 にアクセス
2. Connections → Data sources → Add → Prometheus
3. URL に `http://prometheus:9090` を設定
4. Dashboards → New → Add visualization
5. 上記のPromQLクエリを貼り付けてグラフを作る

---

## Step 5：SLO違反を観察する

```bash
# 大量のエラーを発生させてSLO違反を起こす
for i in $(seq 1 50); do curl -s http://localhost:8080/api/error > /dev/null; done
```

Grafanaのグラフが99.9%を下回る瞬間を確認する。
「この状態がエラーバジェットを消費している」という感覚を掴む。

---

## 確認ポイント

- [ ] P50とP99の差を見て「外れ値がどれだけいるか」を読める
- [ ] エラー率が上がったときにグラフで即座に分かる
- [ ] 平均レイテンシとP99レイテンシが異なる値を示すことを確認した
