# ACL Service (Hertz)

这是一个用于 RAG/检索场景的 ACL HTTP 子服务：

- 支持 **tenant 隔离** + **逐用户授权** + **临时授权有效期**
- 支持两种常见访问模式：
  - 在线过滤：给定 `doc_ids[]` 做 `check-batch`
  - 管理/联调：列出用户已授权的 doc，或一键撤销

默认使用 InMemory store；当配置了 `redis.addr` 时使用 Redis store。

## 启动

```bash
cd services/acl

go run . -config ./config.example.yaml
```

也可以用环境变量指定配置文件路径：`ACL_CONFIG=/path/to/acl.yaml`。

## 配置（YAML）

```yaml
server:
  addr: ":8888"
  # 开启 debug 统计接口：GET /v1/acl/stats
  enable_stats: false

redis:
  # 留空则使用 in-memory store
  addr: "127.0.0.1:6379"
  password: ""
  db: 0
  key_prefix: "acl:"

  # Go duration strings
  dial_timeout: "1s"
  read_timeout: "1s"
  write_timeout: "1s"
```

## API

所有 ID 字段（tenant_id/user_id/doc_id）都要求是 UUID。

### 1) 批量可见性过滤

`POST /v1/acl/check-batch`

Request:

```json
{
  "tenant_id": "...",
  "user_id": "...",
  "doc_ids": ["...", "..."],
  "now": "2026-01-03T12:34:56.123Z"
}
```

- `now` 可选（RFC3339Nano），主要用于 debug/测试；默认用服务端当前时间。

Response:

```json
{
  "allowed_doc_ids": ["..."]
}
```

### 2) 授权

`POST /v1/acl/grant`

Request:

```json
{
  "tenant_id": "...",
  "doc_id": "...",
  "user_id": "...",
  "valid_from": "2026-01-03T12:00:00Z",
  "valid_to": "2026-01-10T12:00:00Z",
  "restricted": true
}
```

- `valid_to` 为空表示永久授权。
- `restricted=true` 可选：同时把文档标记为 restricted（否则默认 public）。

### 3) 撤销单个授权

`POST /v1/acl/revoke`

Request:

```json
{
  "tenant_id": "...",
  "doc_id": "...",
  "user_id": "..."
}
```

### 4) 列出用户显式授权（不枚举 public）

`GET /v1/acl/grants?tenant_id=...&user_id=...&now=...`

Response:

```json
{
  "granted_doc_ids": ["..."]
}
```

### 5) 一键撤销用户所有显式授权

`POST /v1/acl/revoke-all`

Request:

```json
{
  "tenant_id": "...",
  "user_id": "..."
}
```

## Debug：统计接口（A + B）

`GET /v1/acl/stats`

- 需要配置 `server.enable_stats: true` 才会启用；否则返回 404。
- 默认只返回全库量级（A）。
- 带参数 `estimate_prefix=true` 会做“限时采样估算”（B）。

### A：全库量级（零扫描）

```bash
curl 'http://127.0.0.1:8888/v1/acl/stats'
```

返回包含：`dbsize`、`INFO memory`、`INFO keyspace`、`INFO stats`。

### B：按 prefix 估算 key 数量级（限时采样）

```bash
curl 'http://127.0.0.1:8888/v1/acl/stats?estimate_prefix=true&budget_ms=50&scan_count=1000'
```

- `estimate_prefix=true|1`：开启估算
- `budget_ms`：采样时间预算（默认 50ms，范围 10~2000）
- `scan_count`：每次 scan 的 count（默认 1000，范围 10~10000）

估算结果在 `redis.estimate`：
- `scanned_keys/matched_keys/matched_ratio`
- `estimated_keys`（估算值）以及 `estimated_keys_min/max`（粗范围，仅用于提示不确定性）
