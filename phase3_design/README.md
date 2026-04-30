# フェーズ3：実践設計（SWEとSREを統合する）

> 参照：[Ch.7 Automation](https://sre.google/sre-book/automation-at-google/) / [Ch.8 Release Engineering](https://sre.google/sre-book/release-engineering/) / [Ch.9 Simplicity](https://sre.google/sre-book/simplicity/) / [Ch.19 Load Balancing Frontend](https://sre.google/sre-book/load-balancing-frontend/) / [Ch.20 Load Balancing Datacenter](https://sre.google/sre-book/load-balancing-datacenter/) / [Ch.21 Handling Overload](https://sre.google/sre-book/handling-overload/)

## 目標
SWEとして書くコード・設計の判断に、SREの視点を組み込む。「信頼性のためのコードを書く」実践力を身につける。

---

## 1. リリース戦略

> [Chapter 8 - Release Engineering](https://sre.google/sre-book/release-engineering/)

### なぜリリース戦略がSREの仕事か

> "Running reliable services requires reliable release processes."

ほとんどの障害は変更に起因する。したがってリリースの設計がそのまま信頼性の設計になる。

### カナリアデプロイ（Canary Deployment）

新バージョンを全体ではなく、まず一部のサーバーやユーザーにのみリリースする手法。

```
全体         ┌────────────────────────────┐
トラフィック │  旧バージョン（95%）        │
             │  新バージョン（5%）← カナリア│
             └────────────────────────────┘

問題なければ → 新バージョンを段階的に増やす（10% → 50% → 100%）
問題あれば  → カナリアをすぐにロールバック（影響は5%のユーザーのみ）
```

Googleではビルドアーティファクトに `canary` ラベルを付け、canaryラベルをインストールしているサーバーが自動的に新バージョンを受け取る仕組みにしている。

**カナリアの設計で決めること：**
- カナリアのサイズ（何%のトラフィック、何台のサーバー）
- カナリアの期間（何時間観察するか）
- Go/No-Goの判断基準（エラー率・レイテンシのSLO）

### ブルーグリーンデプロイ（Blue-Green Deployment）

本番環境（Blue）と同じ構成の環境（Green）を別に用意し、切り替えはロードバランサのトラフィック切り替えのみで行う。

```
Blue（旧バージョン）← 本番トラフィック
Green（新バージョン）← テスト済み、待機中

切り替え：ロードバランサを向き先を変えるだけ
ロールバック：ロードバランサをBlueに戻すだけ（即時）
```

**メリット：** ゼロダウンタイムデプロイ、即時ロールバック。  
**デメリット：** 常時2倍のリソースが必要。DBスキーマ変更との相性が悪い。

### Push on Green

すべてのテストをパスしたビルドを自動的に本番にデプロイする戦略。Googleの一部チームが採用。

**前提条件：** テストカバレッジが高く、CIが信頼できること。

### フィーチャーフラグ（Feature Flag）

コードはデプロイ済みだが、機能のON/OFFを設定で制御する。

```python
if feature_flags.is_enabled("new_checkout_flow", user_id):
    # 新しい決済フロー
else:
    # 既存の決済フロー
```

**メリット：** デプロイとリリースを分離できる。問題が起きたら再デプロイなしに即OFFにできる。

### ロールバックの設計

リリースで問題が起きたとき、素早くロールバックできる設計が重要。

- DBマイグレーションは後方互換性を持たせる（新旧コードが同じDBで動ける期間を設ける）
- バイナリは前のバージョンを保存しておく
- 設定変更もバージョン管理する

---

## 2. ロードバランシング

> [Chapter 19 - Load Balancing at the Frontend](https://sre.google/sre-book/load-balancing-frontend/) / [Chapter 20 - Load Balancing in the Datacenter](https://sre.google/sre-book/load-balancing-datacenter/)

### なぜロードバランシングが必要か

1台のサーバーに全リクエストが集中すると、その1台が落ちたときにサービス全断になる。複数のサーバーに分散させることで：
- 単一障害点をなくす
- キャパシティを水平に拡張できる

### DNSロードバランシング（フロントエンド）

ユーザーの近くのデータセンターへリクエストを振り分ける最初の層。

```
ユーザー → DNS解決 → 最寄りのデータセンターのIPを返す
```

**TTL（Time To Live）が重要：** TTLが長いとデータセンター障害時の切り替えが遅くなる。

### L4/L7ロードバランサ（データセンター内）

| 種類 | 動作層 | 特徴 |
|------|--------|------|
| L4（トランスポート層） | TCP/UDP | 高速・シンプル。HTTPの中身は見ない |
| L7（アプリケーション層） | HTTP | パス・ヘッダーで振り分けができる。SSL終端も担う |

### ラメダック（Lame Duck）

ヘルスチェックに失敗したサーバーをロードバランサが「跛行状態」と判定し、トラフィックを送らなくなる仕組み。サーバー側から「シャットダウン中」とシグナルを送ることで、グレースフルシャットダウンが可能になる。

### 適切なロードバランシングポリシー

| ポリシー | 内容 | 向いているケース |
|----------|------|----------------|
| Round Robin | 順番に振り分け | サーバーのスペックが均一な場合 |
| Least Connections | 接続数が少ないサーバーへ | リクエストの処理時間が不均一な場合 |
| Weighted | 重みに応じて振り分け | スペックが異なるサーバーが混在する場合 |

---

## 3. オーバーロード（過負荷）への対処

> [Chapter 21 - Handling Overload](https://sre.google/sre-book/handling-overload/)

### 過負荷時の正しい振る舞い

サーバーが処理できる以上のリクエストが来たとき、**全リクエストを遅くするより、一部を早く切り捨てる**方が全体のスループットは上がる。

### ロードシェディング（Load Shedding）

過負荷時に低優先度のリクエストを意図的に切り捨てる設計。

```
優先度高：ユーザーの決済処理 → 過負荷でも処理する
優先度低：バックグラウンドの集計バッチ → 過負荷時はエラーを返す
```

### クライアントサイドスロットリング

サーバーが限界に達したとき、クライアント（呼び出し側）が自分でリクエストを制限する仕組み。サーバーへのリクエストを送る前に切ることで、サーバーの負荷をさらに下げる。

---

## 4. Kubernetesのリソース管理

Kubernetes環境でのSRE実践として特に重要な設計判断。

### requests と limits

```yaml
resources:
  requests:
    cpu: "250m"     # スケジューリング時に保証するCPU
    memory: "256Mi" # スケジューリング時に保証するメモリ
  limits:
    cpu: "1000m"    # 超えた場合はスロットリング（速度制限）
    memory: "512Mi" # 超えた場合はOOMKillで強制終了
```

| 設定 | 意味 | SREとしての関心 |
|------|------|----------------|
| `requests` | Podが必要とする最低保証量 | ノードの配置計画に使われる |
| `limits` | Podが使える上限 | これを超えるとKubernetesが介入する |

**requestsとlimitsの差が大きすぎる問題：**
- CPU: スロットリングが予測しにくくなり、レイテンシスパイクが起きる
- メモリ: OOMKillが突然起きてユーザーへの影響が出る

**推奨：** まず実際の使用量をPrometheusで計測してから設定する。

### HPA（Horizontal Pod Autoscaler）

CPU使用率やカスタムメトリクスに応じてPod数を自動調整する。

```yaml
spec:
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 60  # CPU60%になったらスケールアウト
```

**設計の注意点：**
- スケールアウトに時間がかかる（Pod起動 + ウォームアップ）
- 急激なトラフィックスパイクには間に合わないことがある
- スケールインのクールダウン時間を適切に設定する

---

## 5. SLOを意識したコードの書き方

SWEがコードを書く段階で意識できるSREの視点。

### グレースフルシャットダウン

```
通常のシャットダウン
SIGTERM受信 → 即プロセス終了 → 処理中リクエストが切断される

グレースフルシャットダウン
SIGTERM受信 → 新規リクエストを受け付けない → 処理中リクエストが完了するまで待つ → プロセス終了
```

ロールバランサがラメダックとして認識する時間を確保することも必要。

### 冪等性（Idempotency）の設計

同じリクエストを複数回実行しても、結果が変わらない設計。

**なぜ重要か：** リトライが安全に行えるようになる。

```
冪等でない例：POST /purchase （毎回購入が実行される）
冪等な例   ：POST /purchase（idempotency_key付き）→ 同じkeyのリクエストは2回目以降は無視
```

### ヘルスチェックエンドポイント

Kubernetesのlivenessとreadinessを適切に実装する。

```
liveness  : プロセスが生きているか。Falseになると再起動
readiness : リクエストを受け入れられる状態か。Falseになるとロードバランサが外す
```

**よくあるミス：** livenessとreadinessを同じ実装にする。readinessは依存サービスの状態も含めて判定すべき。

### タイムアウトとデッドラインの伝播

```
ユーザーリクエスト (タイムアウト: 5s)
  → APIサーバー呼び出し (タイムアウト: 4s)
    → DBクエリ (タイムアウト: 3s)
```

上流のタイムアウトを下流にも引き継ぐことで、タイムアウト後の無駄な処理を防ぐ。

### 環境変数による設定の外部化

```python
# ダメな例
DATABASE_HOST = "db.prod.example.com"  # コードにハードコード

# 良い例
DATABASE_HOST = os.environ["DATABASE_HOST"]
```

コードを変えずに環境ごとに設定を変えられ、オートスケール時の設定変更が不要になる。

---

## 6. シンプリシティ（Simplicity）

> [Chapter 9 - Simplicity](https://sre.google/sre-book/simplicity/)

> "The Virtue of Boring" — 退屈さの美徳

複雑なシステムは壊れやすく、デバッグが難しい。SREの観点では、**退屈なほどシンプルなシステムが最も信頼性が高い**。

### Negative Lines of Code Metric

コードの行数が増えるほどバグの可能性も増える。削除したコードはゼロバグ。不要なコードを積極的に削除することが信頼性向上につながる。

### 最小限のAPI

APIは追加が簡単、削除が難しい。最初から最小限のAPIで始め、必要に応じて追加する。

---

## ハンズオン（labs/phase3/）

1. カナリアデプロイをKubernetes上で実装する
2. HPAを設定して負荷テストでオートスケールを確認する
3. グレースフルシャットダウンを実装してゼロダウンタイムデプロイを体験する

→ `labs/phase3/` に手順を用意予定

---

## 学習の完成

フェーズ3まで終えることで：

- **フェーズ1**：見える（計測・可視化）
- **フェーズ2**：耐える（障害設計・インシデント対応）
- **フェーズ3**：進める（安全なリリース・スケール設計・SREを意識したコード）

この3つが揃うことで、SWEとしての開発にSREの視点が統合される。
