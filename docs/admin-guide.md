# Novagate 管理后台指南

一个功能完整的 Web 管理界面，用于管理用户、权限和文档。

## 快速启动

### 前置条件

```bash
# 1. 启动 Redis（ACL 存储）
docker-compose up -d redis

# 2. 等待 Redis 就绪
sleep 3
```

### 启动管理后台

```bash
# 方式 1：使用脚本
./scripts/admin.sh

# 方式 2：直接命令
mise exec -- go run ./cmd/admin -addr :8888 -redis localhost:6379
```

### 访问管理后台

打开浏览器访问：**http://localhost:8888**

## 功能介绍

### 📊 仪表板

- 显示用户、文档、权限的统计数据
- 快速概览系统信息

### 👥 用户管理

**新增用户**：
1. 点击"+ 新增用户"按钮
2. 填写用户信息：
   - 用户 ID（唯一标识）
   - 用户名（显示名称）
   - 邮箱（联系方式）
3. 点击"创建"按钮

**删除用户**：
- 在用户列表中点击"删除"按钮
- 确认删除（会同时删除该用户的所有权限）

### 📄 文档管理

**新增文档**：
1. 点击"+ 新增文档"按钮
2. 填写文档信息：
   - 文档 ID（唯一标识）
   - 标题（文档名称）
   - 分类（编程、前端、后端等）
   - 所有者（创建者用户 ID）
3. 点击"创建"按钮

**删除文档**：
- 在文档列表中点击"删除"按钮
- 确认删除（注意：不会自动删除权限规则）

### 🔒 权限管理

**授予权限**：
1. 点击"+ 授予权限"按钮
2. 选择要授予权限的用户和文档
3. 点击"授予"按钮

**撤销权限**：
- 在权限规则中找到要撤销的权限
- 点击"✕"按钮删除该权限

**权限说明**：
- 用户可以访问多个文档
- 删除用户会自动删除其所有权限
- 权限规则基于"用户-文档"映射

### 📋 审计日志

自动记录以下操作：
- 用户创建/删除
- 文档创建/删除
- 权限授予/撤销

## API 接口

所有 API 返回 JSON 格式的响应：

```json
{
    "code": 200,
    "message": "success",
    "data": {...}
}
```

### Users

**列出所有用户**：
```bash
curl http://localhost:8888/api/users
```

**创建用户**：
```bash
curl -X POST http://localhost:8888/api/users/create \
  -H "Content-Type: application/json" \
  -d '{
    "id": "user-003",
    "name": "Charlie",
    "email": "charlie@example.com"
  }'
```

**删除用户**：
```bash
curl -X POST http://localhost:8888/api/users/delete \
  -H "Content-Type: application/json" \
  -d '{"id": "user-003"}'
```

### Documents

**列出所有文档**：
```bash
curl http://localhost:8888/api/documents
```

**创建文档**：
```bash
curl -X POST http://localhost:8888/api/documents/create \
  -H "Content-Type: application/json" \
  -d '{
    "id": "doc-004",
    "title": "新文档",
    "category": "demo",
    "owner_id": "user-001"
  }'
```

**删除文档**：
```bash
curl -X POST http://localhost:8888/api/documents/delete \
  -H "Content-Type: application/json" \
  -d '{"id": "doc-004"}'
```

### Permissions

**列出权限规则**：
```bash
curl 'http://localhost:8888/api/permissions?tenant_id=tenant-001'
```

**授予权限**：
```bash
curl -X POST http://localhost:8888/api/permissions/grant \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "tenant-001",
    "user_id": "user-001",
    "doc_id": "doc-004"
  }'
```

**撤销权限**：
```bash
curl -X POST http://localhost:8888/api/permissions/revoke \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "tenant-001",
    "user_id": "user-001",
    "doc_id": "doc-004"
  }'
```

### Audit Logs

**获取审计日志**：
```bash
curl 'http://localhost:8888/api/audit-logs?limit=50'
```

## 使用示例

### 场景 1：创建新用户并授予权限

```bash
# 1. 创建用户
curl -X POST http://localhost:8888/api/users/create \
  -H "Content-Type: application/json" \
  -d '{"id": "user-004", "name": "Diana", "email": "diana@example.com"}'

# 2. 为用户授予文档访问权限
curl -X POST http://localhost:8888/api/permissions/grant \
  -H "Content-Type: application/json" \
  -d '{"tenant_id": "tenant-001", "user_id": "user-004", "doc_id": "doc-001"}'

# 3. 验证权限
curl 'http://localhost:8888/api/permissions?tenant_id=tenant-001'
```

### 场景 2：上传新文档

```bash
# 1. 创建文档
curl -X POST http://localhost:8888/api/documents/create \
  -H "Content-Type: application/json" \
  -d '{
    "id": "doc-005",
    "title": "Rust 系统编程",
    "category": "programming",
    "owner_id": "user-002"
  }'

# 2. 为 Alice 授予访问权限
curl -X POST http://localhost:8888/api/permissions/grant \
  -H "Content-Type: application/json" \
  -d '{"tenant_id": "tenant-001", "user_id": "user-001", "doc_id": "doc-005"}'
```

## 与 Novagate 网关的集成

管理后台管理的数据都存储在 Redis 中，与网关共享相同的数据存储：

```
Redis 结构：
├── user:user-001          # 用户信息
├── user:user-002
├── doc:doc-001            # 文档信息
├── doc:doc-002
├── acl:tenant-001:user-001  # 权限规则
├── acl:tenant-001:user-002
└── audit:logs             # 审计日志
```

当你在管理后台更改权限时，网关可以立即读取到最新的权限规则。

## 与 RAG 流程的配合

1. **创建文档** → 管理后台创建文档元数据
2. **授予权限** → 使用管理后台配置用户权限
3. **查询时过滤** → 网关和 RAG 查询服务使用权限规则过滤结果

示例：
```
用户 Alice 查询 "Python 编程"
  ↓
网关收到请求，提取用户 ID: user-001
  ↓
RAG 服务向 Milvus 查询相关文档
  ↓
在 Redis 中查询 acl:tenant-001:user-001 的权限
  ↓
过滤只返回 Alice 有权访问的文档
```

## 故障排查

### Redis 连接失败

```
failed to create admin service: redis connection failed
```

**解决**：
```bash
# 启动 Redis
docker-compose up -d redis

# 验证连接
docker-compose exec redis redis-cli PING
```

### 端口被占用

```
listen tcp :8888: bind: address already in use
```

**解决**：
```bash
# 使用其他端口
./scripts/admin.sh -addr :9999

# 或杀死占用端口的进程
lsof -i :8888
kill -9 <PID>
```

### 静态文件未找到

确保在项目根目录运行：
```bash
cd /path/to/novagate
mise exec -- go run ./cmd/admin
```

## 开发与扩展

### 添加新的管理功能

1. **后端**：在 `internal/admin/service.go` 中添加新的处理函数
2. **前端**：在 `web/index.html` 中添加新的 UI 和 API 调用

### 修改数据模型

数据存储在 Redis 中，使用 Hash 和 Set 的组合：
- Hash：存储结构化数据（用户、文档）
- Set：存储集合数据（权限）

## 常见问题

**Q: 能否导入/导出用户和权限？**
A: 目前不支持，可以通过 API 批量操作。

**Q: 审计日志如何持久化？**
A: 当前存储在 Redis 中，重启后丢失。建议集成 Kafka 或持久化存储。

**Q: 支持多租户吗？**
A: 支持，通过 `tenant_id` 参数实现。管理界面默认使用 `tenant-001`。

## 后续改进

- [ ] 批量导入用户和权限（CSV/Excel）
- [ ] 向量数据管理（Milvus 集合管理）
- [ ] 审计日志持久化和查询
- [ ] 权限模板和角色管理
- [ ] 用户 API Token 管理
- [ ] 操作日志和修改历史

## 参考资源

- [端到端演示指南](e2e-demo-guide.md)
- [数据库参考文档](database-reference.md)
- [ACL-RAG 对接契约](acl-rag-contract.md)
