# Singleflight Demo

Cache Stampede（Thundering Herd）問題と、`singleflight` パターンによる解決策をデモンストレーションする環境です。

## Cache Stampede とは？

キャッシュが切れた瞬間に大量のリクエストが同時に到達すると、すべてのリクエストが「キャッシュがない」と判断し、全員がDBに問い合わせてしまう問題です。

```
1,000 Requests → Cache Miss → 1,000 DB Calls → DB過負荷
```

## Singleflight による解決

`singleflight` パターンは、同じキーに対する重複したリクエストを1つにまとめます。

```
1,000 Requests → Cache Miss → Singleflight → 1 DB Call → 結果を共有
```

## アーキテクチャ

```mermaid
graph LR
    k6[k6<br/>負荷生成] --> API[Go API Server]
    API --> Redis[Redis<br/>Cache]
    API --> Prom[Prometheus<br/>+ Grafana]
```

## クイックスタート

### 1. 環境起動

```bash
docker compose up -d
```

### 2. Grafana にアクセス

- URL: http://localhost:3000
- ユーザー: `admin`
- パスワード: `admin`
- ダッシュボード: 「Singleflight Demo」を開く

### 3. 負荷テスト実行

**Singleflight 無し（Cache Stampede 発生）:**

```bash
docker compose run --rm k6 run /scripts/without-singleflight.js
```

**Singleflight 有り（リクエスト集約）:**

```bash
docker compose run --rm k6 run /scripts/with-singleflight.js
```

**比較テスト（両方を連続実行）:**

```bash
docker compose run --rm k6 run /scripts/comparison.js
```

### 4. Grafana で結果を確認

- **DB Calls Rate**: Singleflight無しでは多数のDB呼び出しが発生
- **Singleflight Shared Rate**: リクエストが共有された回数
- **Cache Misses Rate**: キャッシュミスの発生率

## エンドポイント

| エンドポイント | 説明 |
|---------------|------|
| `GET /api/without-singleflight` | Singleflight無し（Cache Stampede発生） |
| `GET /api/with-singleflight` | Singleflight有り（リクエスト集約） |
| `GET /api/clear-cache` | キャッシュをクリア |
| `GET /metrics` | Prometheusメトリクス |
| `GET /health` | ヘルスチェック |

## メトリクス

| メトリクス | 説明 |
|-----------|------|
| `cache_hits_total` | キャッシュヒット数 |
| `cache_misses_total` | キャッシュミス数 |
| `db_calls_total` | DBコール数 |
| `singleflight_shared_total` | Singleflightで共有されたリクエスト数 |
| `request_duration_seconds` | リクエスト処理時間 |

## 期待される結果

100同時リクエストでキャッシュクリア直後の場合:

| メトリクス | Without Singleflight | With Singleflight |
|-----------|---------------------|-------------------|
| DB Calls | ~100 | ~1 |
| Cache Miss | 100 | 100 |
| Singleflight Shared | 0 | ~99 |

## 停止

```bash
docker compose down
```

## 参考

- [golang.org/x/sync/singleflight](https://pkg.go.dev/golang.org/x/sync/singleflight)
- [Cache Stampede](https://en.wikipedia.org/wiki/Cache_stampede)
