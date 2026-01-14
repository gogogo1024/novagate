# 端到端演示指南（E2E Demo）

本指南演示如何在真实环境中跑通整个 Novagate RAG 流程。

## 快速开始（3 个命令）

```bash
# 1. 启动完整演示（自动启动所有服务、初始化数据、测试调用）
./scripts/e2e-demo.sh

# 2. 在另一个终端，运行 RAG 演示（向量检索 + ACL 过滤）
python3 scripts/rag-demo.py --demo-mode

# 3. 停止网关（按 Ctrl+C）
```

完整演示耗时约 1-2 分钟（取决于服务启动速度）。

## 演示流程详解

### Step 1：启动所有服务

```bash
# 启动核心服务
- Redis (ACL 存储) - ✓ 必需
- Kafka (消息队列) - ✓ 启动
- Milvus (向量数据库) - ✓ 启动
- 网关服务 - ✓ 启动

# 服务就绪后输出：
# [✓] Redis 就绪
# [✓] Kafka 就绪
# [✓] Milvus 就绪
# [✓] 网关已启动 (PID: xxxxx)
```

### Step 2：初始化数据

#### Redis ACL 数据

创建如下数据结构：

```
用户:
  - user-001 (Alice) - 可访问 doc-001, doc-002
  - user-002 (Bob) - 可访问 doc-001

租户:
  - tenant-001 (Acme Corp)
  - tenant-002 (Startup Inc)

文档:
  - doc-001: Python 最佳实践
  - doc-002: Go 并发编程
  - doc-003: JavaScript 框架对比
```

#### Milvus 向量数据

创建 `novagate_rag_documents` 集合，包含：

```
3 个文档 × 3 个 chunk = 9 条向量数据

doc-001: Python 最佳实践
  - chunk-0: "Python 是一门易于学习的编程语言..."
  - chunk-1: "在 Python 中应该优先使用列表推导式..."
  - chunk-2: "异常处理是编写健壮 Python 代码的关键..."

doc-002: Go 并发编程
  - chunk-0: "Goroutine 是 Go 语言的核心特性..."
  - chunk-1: "Channel 用于在 Goroutine 之间安全地传递数据..."
  - chunk-2: "使用 sync.Mutex 保护共享资源..."

doc-003: JavaScript 框架对比
  - chunk-0: "React 是一个用于构建用户界面..."
  - chunk-1: "Vue.js 提供了更温和的学习曲线..."
  - chunk-2: "Angular 是一个完整的框架..."
```

### Step 3：执行测试用例

#### Test 1: Ping（验证网关连接）

```bash
mise exec -- go run ./cmd/client -addr 127.0.0.1:9000 -cmd 0x0001 -payload "ping"

# 预期输出：
# resp: cmd=0x0001 request_id=1 payload="pong"
```

#### Test 2: RAG 流程（向量检索 + ACL 过滤）

```bash
python3 scripts/rag-demo.py --query "Python 编程最佳实践" --user user-001

# 流程：
# 1. 向量化查询
# 2. Milvus 向量检索 → 找到最相关的文档
# 3. Redis ACL 过滤 → 检查用户权限
# 4. 返回用户可访问的结果
```

#### Test 3: ACL 权限查询

```bash
docker-compose exec redis redis-cli

# 在 redis-cli 中：
> HGETALL user:user-001       # 查看用户信息
> SMEMBERS acl:tenant-001:user-001  # 查看用户权限
> HGETALL doc:doc-001         # 查看文档元数据
```

## 演示场景

### 场景 1：Alice 查询 Python 文档（权限充足）

```bash
python3 scripts/rag-demo.py \
    --query "Python 编程最佳实践" \
    --user user-001 \
    --tenant tenant-001
```

**预期结果**：
- 向量检索找到 doc-001, doc-002, doc-003
- ACL 过滤后：doc-001 ✓、doc-002 ✓、doc-003 ✗（无权限）
- 返回 2 条可访问文档

### 场景 2：Bob 查询 JavaScript 文档（权限受限）

```bash
python3 scripts/rag-demo.py \
    --query "JavaScript 框架" \
    --user user-002 \
    --tenant tenant-001
```

**预期结果**：
- 向量检索找到 doc-003
- ACL 过滤后：doc-003 ✗（Bob 只有 doc-001 权限）
- 返回 0 条可访问文档，1 条无权限访问

### 场景 3：多个演示场景自动运行

```bash
python3 scripts/rag-demo.py --demo-mode
```

自动执行 3 个场景：
1. Alice 查询 Python
2. Alice 查询 Go
3. Bob 查询 JavaScript

## 访问管理界面

演示运行期间，可以访问各服务的管理界面：

```
网关服务:
  127.0.0.1:9000
  测试: mise exec -- go run ./cmd/client -addr 127.0.0.1:9000 -cmd 0x0001

Redis Insights (ACL 存储查看):
  http://localhost:8081

Kafka UI (消息队列管理):
  http://localhost:8080

Milvus Attu (向量数据库管理):
  http://localhost:8000

MinIO Console (对象存储):
  http://localhost:9001
  用户名: minioadmin
  密码: minioadmin
```

## 常见命令

### 手动启动服务（不运行演示）

```bash
# 仅启动数据库
docker-compose up -d
docker-compose --profile kafka up -d
docker-compose --profile milvus up -d

# 启动网关
mise exec -- go run ./cmd/server

# 启动 Kafka 管理 UI
docker-compose --profile kafka up -d kafka-ui

# 启动 Milvus 管理 UI
docker-compose --profile milvus up -d attu
```

### 查看服务日志

```bash
# 网关日志（运行网关的终端）
# 直接显示在终端

# Docker 服务日志
docker-compose logs redis
docker-compose logs kafka
docker-compose logs milvus
docker-compose logs -f  # 实时跟踪所有服务

# 特定容器日志
docker-compose logs --tail=50 milvus
```

### 直接数据库操作

```bash
# Redis CLI
./scripts/db.sh redis-cli
# 或
docker-compose exec redis redis-cli

# Kafka CLI
./scripts/db.sh kafka-cli topics
./scripts/db.sh kafka-cli produce my-topic
./scripts/db.sh kafka-cli consume my-topic

# Milvus 信息
./scripts/db.sh milvus-info
```

### 清理资源

```bash
# 停止容器（保留数据）
docker-compose stop

# 删除容器（保留数据卷）
docker-compose down

# 完全清理（删除所有数据）
docker-compose down -v

# 清理悬垂镜像
docker image prune -f
```

## 故障排查

### 问题 1：Milvus 启动超时

```
[!] Milvus 未启动，跳过
```

**解决**：Milvus 启动较慢，等待 30+ 秒，手动检查：

```bash
docker-compose logs milvus
docker-compose ps milvus
```

### 问题 2：Redis 连接失败

```
[✗] Redis 未启动
```

**解决**：确保 Docker 运行，手动启动：

```bash
docker-compose up -d redis
./scripts/db.sh redis-cli PING
```

### 问题 3：Python 包缺失

```
ModuleNotFoundError: No module named 'pymilvus'
```

**解决**：安装依赖

```bash
pip install pymilvus redis
```

### 问题 4：网关连接失败

```
[✗] Ping 失败
```

**解决**：检查网关是否启动，查看日志：

```bash
ps aux | grep "cmd/server"
mise exec -- go run ./cmd/server  # 手动启动并查看错误
```

## 理解演示的目的

这个演示展示了：

1. **协议层** - 网关正确接收和响应请求
2. **向量检索** - Milvus 找到相关文档
3. **权限控制** - ACL 过滤用户不可访问的文档
4. **完整流程** - 真实的 RAG 应用场景

## 后续步骤

演示成功后，你可以：

1. **修改查询** - 改变 `--query` 参数测试不同查询
2. **修改权限** - 在 Redis 中修改 ACL 规则，观察变化
3. **添加文档** - 运行 `scripts/milvus/init-collections.py` 扩展向量数据
4. **集成业务** - 在 `internal/service/registry.go` 添加真实业务逻辑

## 参考资源

- [Kafka + Milvus 快速上手](kafka-milvus-quickstart.md)
- [数据库参考文档](database-reference.md)
- [ACL-RAG 对接契约](acl-rag-contract.md)
- [协议规范](protocol.md)
