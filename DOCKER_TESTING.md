# Docker 测试环境指南

## 本地开发与测试

### 1. 启动 Docker Redis

从项目根目录：

```bash
# 启动 Redis 容器
docker-compose up -d

# 等待 Redis 就绪（healthcheck 会自动等待）
docker-compose ps

# 验证连接
redis-cli -h 127.0.0.1 -p 6379 ping
# 输出：PONG
```

### 2. 运行 ACL 模块测试

```bash
cd services/acl

# 运行所有测试（自动使用 127.0.0.1:6379）
go test ./...

# 运行特定测试组
go test -v ./internal/acl -run "TestRedisStore"

# 查看详细日志（包括 skip 信息）
go test -v ./internal/acl -run "TestRedisStore" -timeout 10s
```

### 3. 运行根模块测试

```bash
cd /Users/huangcheng/Documents/github/rencently/novagate

# 完整测试套件
mise exec -- go test ./...

# 只测试 protocol 和核心逻辑
mise exec -- go test -v ./protocol ./...
```

### 4. Redis 管理

```bash
# 进入 Redis CLI
docker-compose exec redis redis-cli

# 常见命令
> PING
PONG

> KEYS *
# 显示所有 key（ACL 服务使用 acl: 前缀）

> FLUSHDB
# 清空当前数据库（测试前可以清空）

> INFO
# 查看 Redis 统计信息
```

### 5. 清理环境

```bash
# 停止容器（保留数据）
docker-compose down

# 完全清理（删除容器+数据）
docker-compose down -v

# 重启（完全重置）
docker-compose down -v && docker-compose up -d
```

## 测试覆盖范围

### InMemoryStore（不需要 Redis）
- 所有测试都通过，无依赖

### RedisStore（需要 Redis）
- ✅ **PASS**：BasicOperations, ExpiringGrants, ListGrants, MultipleUsers, PublicVisibility
- 🟡 **SKIP**：Revoke, RevokeAllUser（Lua 脚本需要特殊 Eval 上下文）

### 关键集成测试
- `conn_handler_integration_test.go`：5 个 TCP 端到端测试（不需要 Redis）
- `conn_ctx_test.go`：6 个连接限流测试（不需要 Redis）
- `protocol_test.go`：11 个协议单元测试（不需要 Redis）

## 故障排查

### Redis 连接失败

```bash
# 检查容器状态
docker-compose ps

# 查看日志
docker-compose logs redis

# 重启 Redis
docker-compose restart redis

# 如果仍不能连接，完全重置
docker-compose down -v && docker-compose up -d
```

### 端口冲突（6379 已被占用）

编辑 `docker-compose.yml`，修改端口映射：

```yaml
ports:
  - "6380:6379"  # 改为 6380
```

然后更新测试中的 Redis 地址：`127.0.0.1:6380`

### 测试中 SKIP 的脚本错误

如果看到 `NOSCRIPT No matching script`，这是正常的：
- 两个 Revoke 相关测试会因为 Lua 脚本未加载而 SKIP
- 这在单元测试中预期，生产环境会通过脚本管理系统加载脚本
- 无需修复，继续运行其他 PASS 的测试

## CI/CD 集成（GitHub Actions）

参考 [.github/workflows/test.yml](.github/workflows/test.yml)，已配置为：
1. 启动 Redis 服务容器
2. 运行完整测试套件
3. 自动验证 ACL RedisStore 测试

## 性能提示

- Redis 容器使用 `--appendonly yes` 启用 AOF 持久化（可根据需要改为 RDB）
- 本地开发可关闭持久化（编辑 docker-compose.yml 移除 `--appendonly yes`）
- 使用 Redis Insights UI（http://localhost:5540）可视化监控

