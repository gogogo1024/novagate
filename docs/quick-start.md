# 快速开始指南 - 完整系统

一键启动完整的 Novagate 系统，包括管理后台、网关、数据库。

## 最快速的开始方式（推荐）

### 1. 一键启动所有服务

```bash
./scripts/start-all.sh
```

这个脚本会自动：
- ✅ 启动 Redis、Kafka、Milvus（Docker）
- ✅ 初始化 ACL 数据（3 个用户 + 3 个文档）
- ✅ 初始化 Milvus 向量数据（9 条向量）
- ✅ 启动管理后台（http://localhost:8888）
- ✅ 启动网关（127.0.0.1:9000）
- ✅ 显示所有服务访问方式

**耗时：约 1-2 分钟**

### 2. 打开浏览器

访问管理后台：**http://localhost:8888**

![管理后台功能]
- 👥 **用户管理** - 查看/创建/删除用户
- 📄 **文档管理** - 查看/创建/删除文档
- 🔒 **权限管理** - 授予/撤销权限
- 📊 **仪表板** - 实时统计数据

### 3. 运行演示（在另一个终端）

#### RAG 演示 - 向量检索 + 权限过滤

```bash
# 运行所有演示场景
python3 scripts/rag-demo.py --demo-mode

# 或查询特定内容
python3 scripts/rag-demo.py --query "Python 最佳实践" --user user-001
```

#### 网关测试 - Ping

```bash
mise exec -- go run ./cmd/client -addr 127.0.0.1:9000 -cmd 0x0001 -payload ping

# 预期输出：
# resp: cmd=0x0001 request_id=1 payload="pong"
```

## 完整的服务清单

| 服务 | 地址 | 功能 |
|------|------|------|
| **管理后台** | http://localhost:8888 | Web UI 管理用户/文档/权限 |
| **网关** | 127.0.0.1:9000 | RPC 网关 |
| **Redis** | localhost:6379 | ACL 权限存储 |
| **Kafka** | localhost:9092 | 消息队列 |
| **Milvus** | localhost:19530 | 向量数据库 |
| **Redis Insights** | http://localhost:8081 | Redis 可视化工具 |
| **Kafka UI** | http://localhost:8080 | Kafka 管理工具 |
| **Milvus Attu** | http://localhost:8000 | Milvus 管理工具 |
| **MinIO Console** | http://localhost:9001 | 对象存储管理（minioadmin/minioadmin） |

## 初始数据

启动脚本自动创建以下数据：

### 用户
```
user-001 (Alice) - alice@example.com
user-002 (Bob) - bob@example.com
```

### 文档
```
doc-001: Python 最佳实践 (3 个 chunks)
doc-002: Go 并发编程 (3 个 chunks)
doc-003: JavaScript 框架对比 (3 个 chunks)
```

### 权限
```
Alice (user-001): 可访问 doc-001, doc-002
Bob (user-002): 可访问 doc-001 只
```

## 常见操作

### 创建新用户

在管理后台 → 用户管理 → "+ 新增用户"

或通过 API：
```bash
curl -X POST http://localhost:8888/api/users/create \
  -H "Content-Type: application/json" \
  -d '{
    "id": "user-003",
    "name": "Charlie",
    "email": "charlie@example.com"
  }'
```

### 授予权限

在管理后台 → 权限管理 → "+ 授予权限"

或通过 API：
```bash
curl -X POST http://localhost:8888/api/permissions/grant \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "tenant-001",
    "user_id": "user-003",
    "doc_id": "doc-001"
  }'
```

### 测试权限过滤

```bash
# 查询 Charlie 可访问的文档
python3 scripts/rag-demo.py --user user-003 --query "Python"
```

## 查看日志

```bash
# 管理后台日志
tail -f /tmp/admin.log

# 网关日志
tail -f /tmp/gateway.log

# Redis
docker-compose logs -f redis

# Kafka
docker-compose logs -f kafka

# Milvus
docker-compose logs -f milvus
```

## 关闭服务

### 关闭后台进程

```bash
# 从启动脚本的输出中获取 PID
kill <ADMIN_PID>      # 关闭管理后台
kill <GATEWAY_PID>    # 关闭网关
```

### 停止 Docker 容器

```bash
# 停止（保留数据）
docker-compose stop

# 删除容器（保留数据卷）
docker-compose down

# 完全清理（删除所有数据）
docker-compose down -v
```

## 故障排查

### 端口被占用

```bash
# 查看占用端口的进程
lsof -i :8888  # 管理后台
lsof -i :9000  # 网关
lsof -i :6379  # Redis

# 杀死进程
kill -9 <PID>
```

### Redis 连接失败

```bash
# 验证 Redis 是否运行
docker-compose ps redis

# 手动启动
docker-compose up -d redis

# 测试连接
docker-compose exec redis redis-cli PING
```

### Milvus 连接失败

```bash
# Milvus 启动较慢，等待 30+ 秒
sleep 30

# 检查日志
docker-compose logs milvus

# 重启
docker-compose restart milvus
```

### 管理后台无法加载

1. 检查后台是否运行：`ps aux | grep "cmd/admin"`
2. 查看日志：`tail -f /tmp/admin.log`
3. 手动启动并查看错误：`mise exec -- go run ./cmd/admin`

## 下一步

1. **自定义演示数据** - 在管理后台创建你自己的用户/文档/权限
2. **研究权限规则** - 测试权限对 RAG 查询的影响
3. **集成业务逻辑** - 在 `internal/service` 中添加真实业务逻辑
4. **扩展管理功能** - 在 `internal/admin/service.go` 中添加新的 API

## 详细文档

- [管理后台指南](admin-guide.md)
- [端到端演示指南](e2e-demo-guide.md)
- [数据库参考文档](database-reference.md)
- [Kafka + Milvus 快速上手](kafka-milvus-quickstart.md)
- [协议规范](protocol.md)
