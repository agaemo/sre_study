# SRE学習

SWEとしての開発経験をベースに、SREの考え方・技術をゼロから学ぶためのリポジトリ。

---

## 参考文献

| 書籍 | URL |
|------|-----|
| Site Reliability Engineering（SRE本） | https://sre.google/sre-book/table-of-contents/ |
| The Site Reliability Workbook（実践編） | https://sre.google/workbook/table-of-contents/ |
| Building Secure & Reliable Systems | https://sre.google/books/building-secure-reliable-systems/ |

すべてGoogleが無料で公開しているオンライン版。

---

## 必要なツール

| ツール | 用途 | インストール |
|--------|------|-------------|
| **Docker Desktop** | ハンズオン環境の起動 | https://www.docker.com/products/docker-desktop/ |
| Git | バージョン管理 | 大抵インストール済み |

ハンズオン（`labs/`）はすべてDocker Composeで動かすため、**Docker Desktopが必須**。

---

## 学習フェーズ

### はじめに：SREとは何か
SREの歴史・目的・SWEとの違いを理解する。

→ [SREとは何か・自動化の5段階](intro/what_is_sre.md)

---

### フェーズ1：オブザーバビリティ（計測と可視化）
「システムの今の状態を数値で語れる」ようになる。

- SLI / SLO / SLA / エラーバジェット
- レイテンシ（P50・P95・P99）・スループット・可用性
- ログ・メトリクス・トレーシング（オブザーバビリティの3本柱）
- CPU・メモリ・ネットワーク・ディスクの読み方

→ [教材を読む](phase1_observability/README.md) / [ハンズオンをやる](labs/phase1/README.md)

---

### フェーズ2：障害設計（壊れることを前提に作る）
リスク設計・カスケード障害・サーキットブレーカー・インシデント対応・ポストモーテム文化。

→ [教材を読む](phase2_resilience/README.md) / [ハンズオンをやる](labs/phase2/README.md)

---

### フェーズ3：実践設計（SWEとSREを統合する）
リリース戦略・ロードバランシング・Kubernetesリソース管理・SLOを意識したコード設計。

→ [教材を読む](phase3_design/README.md) / [ハンズオンをやる](labs/phase3/README.md)

---

## ハンズオン一覧

| フェーズ | 内容 | ツール |
|----------|------|--------|
| [Phase 1](labs/phase1/README.md) | Prometheus + Grafana でメトリクス計測・SLO監視 | Docker, Go |
| [Phase 2](labs/phase2/README.md) | カスケード障害とサーキットブレーカー | Docker, Go |
| [Phase 3](labs/phase3/README.md) | カナリアデプロイとグレースフルシャットダウン | Docker, Go, nginx |
