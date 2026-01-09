# RAG/Chat ↔ ACL 调用契约（HTTP）

目标：在“向量召回（Milvus）→ ACL 判定 → 回源取句子文本 → LLM 生成”的链路里，确保 **不泄露未授权内容**，同时满足“多数文档公开、少数私有、且私有支持临时授权有效期”的需求。

> 本文描述的是 **ACL 服务的对接契约**，不绑定具体 RAG/Chat 实现语言。

## 核心原则（防泄露）

- Milvus 召回阶段只返回 `doc_id`（或 `sentence_id`/`chunk_id` 的引用），**不要返回可读文本**。
- 在拿到候选集合后，先调用 ACL 服务过滤，再去“原文/句子存储”回源取文本。
- 若 ACL 服务不可用：
  - 推荐 fail-closed（宁可少答、不给私有内容）；
  - 若业务必须 fail-open，需要在产品/合规层明确风险，并单独做隔离开关。

## 标识符

- `tenant_id`：租户 UUID
- `user_id`：用户 UUID
- `doc_id`：文档 UUID

可扩展（不影响 ACL MVP）：
- `sentence_id`：句子 UUID（用于句子级引用）
- `chunk_id`：分块 UUID

## 调用时机（在线检索链路）

### 1) 向量召回

RAG 服务调用向量库（Milvus）获取候选集合（示例）：

```json
{
  "candidates": [
    {"doc_id": "...", "score": 0.83},
    {"doc_id": "...", "score": 0.81}
  ]
}
```

### 2) ACL 批量过滤（强制）

调用 ACL：`POST /v1/acl/check-batch`

Request：

```json
{
  "tenant_id": "...",
  "user_id": "...",
  "doc_ids": ["...", "..."],
  "now": "2026-01-04T12:34:56.123Z"
}
```

- `now` 可选（RFC3339Nano），主要用于测试/回放；默认用服务端当前时间。

Response：

```json
{
  "allowed_doc_ids": ["..."]
}
```

RAG 服务将召回结果按 `allowed_doc_ids` 过滤，然后进入回源。

### 3) 回源取句子/段落文本

RAG 服务用 `allowed_doc_ids` 去文档存储（对象存储/DB/全文索引/段落库）取句子文本。

建议：回源接口支持“按 doc_id 批量取句子”，并返回可用于引用的结构，例如：

```json
{
  "doc_id": "...",
  "sentences": [
    {
      "sentence_id": "...",
      "text": "...",
      "offset_start": 123,
      "offset_end": 156
    }
  ]
}
```

### 4) LLM 生成与引用

Chat 服务返回答案时携带引用（citation），至少能回指到 `doc_id`，最好能回指到 `sentence_id`。

## 管理/运营场景（非在线）

### 授权（永久或临时）

`POST /v1/acl/grant`

- `valid_to` 为空表示永久授权
- `restricted=true` 可选：同时把文档设置为 restricted（否则默认 public）

### 撤销

- 单个撤销：`POST /v1/acl/revoke`
- 一键撤销某用户全部授权：`POST /v1/acl/revoke-all`

### 列出用户已授权文档（不枚举 public）

`GET /v1/acl/grants?tenant_id=...&user_id=...&now=...`

## 可观测性（量级决策）

- `/v1/acl/stats`（需要 `server.enable_stats: true`）
  - A：默认返回全库 `DBSIZE/INFO`（零扫描）
  - B：`?estimate_prefix=true&budget_ms=50&scan_count=1000` 返回按 prefix 的量级估算（限时采样）

## 默认策略与过期策略（实现约束）

- **Visibility 默认 public**：Redis 中缺少 `vis` key 即视为 public；将文档设置为 public 时会删除该 key。
- **临时授权过期**：通过 `valid_to` 判断是否仍有效；过期成员会在写入临时授权时做一次“机会型清理”（避免过期集合无限增长）。
