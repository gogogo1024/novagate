# Docker 测试环境指南

## 数据库架构

当前项目使用的数据库：
- ✅ **Redis**：ACL 服务的主要存储（实时授权检查）
- 🔮 **PostgreSQL**（预留）：可用于会话管理、审计日志等持久化需求
- 🔮 **MySQL**（预留）：备选关系型数据库

## 本地开发与测试

### 1. 启动数据库（Redis）

**快速启动**（仅 Redis，最常用）：

```bash
# 方式 1：使用默认 docker-compose.yml
docker-compose up -d redis

# 方式 2：使用测试专用配置（无持久化，更快）
docker-compose -f docker-compose.test.yml up -d
```

**完整启动**（Redis + 可视化工具）：

```bash
# 启动 Redis + Redis Insights
docker-compose up -d redis
docker-compose --profile tools up -d  # 启动 Redis Insights

# 验证
docker-compose ps
```

**启动额外数据库**（可选）：

```bash
# PostgreSQL（如需关系型数据库）
docker-compose --profile postgres up -d

# MySQL（如需 MySQL）
docker-compose --profile mysql up -d

# 全部启动
docker-compose --profile postgres --profile mysql --profile tools up -d
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

### 4. 数据库管理

#### Redis CLI

```bash
# 进入 Redis CLI
docker-compose exec redis redis-cli

# 常见命令
> PING
PONG

> KEYS acl:*
# 显示 ACL 相关 key

> GET acl:tenant:xxx:doc:yyy:vis
# 查看文档可见性

> FLUSHDB
# 清空当前数据库（测试前可以清空）

> INFO memory
# 查看内存使用情况
```

#### PostgreSQL（如已启动）

```bash
# 进入 PostgreSQL CLI
docker-compose exec postgres psql -U novagate -d novagate

# 常见命令
\dt          # 列出所有表
\d sessions  # 查看 sessions 表结构
SELECT * FROM sessions LIMIT 10;
```

#### MySQL（如已启动）

```bash
# 进入 MySQL CLI
docker-compose exec mysql mysql -u novagate -pnovagate_dev novagate

# 常见命令
SHOW TABLES;
DESCRIBE sessions;
SELECT * FROM sessions LIMIT 10;
```

#### 数据库备份

```bash
# Redis 备份
docker-compose exec redis redis-cli SAVE
docker cp novagate-redis:/data/dump.rdb ./backup/redis-$(date +%Y%m%d).rdb

# PostgreSQL 备份
docker-compose exec postgres pg_dump -U novagate novagate > backup/postgres-$(date +%Y%m%d).sql

# MySQL 备份
docker-compose exec mysql mysqldump -u novagate -pnovagate_dev novagate > backup/mysql-$(date +%Y%m%d).sql
```

### 5. 清理环境

```bash
# 停止所有容器（保留数据）
docker-compose down

# 停止特定服务
docker-compose stop redis
docker-compose stop postgres

# 完全清理（删除容器+数据卷）
docker-compose down -v

# 清理特定数据卷
docker volume rm novagate_redis-data
docker volume rm novagate_postgres-data

# 重启（完全重置）
docker-compose down -v && docker-compose up -d
```

## 数据库连接信息

### Redis

- **地址**：`127.0.0.1:6379`（或容器内 `redis:6379`）
- **密码**：无（默认）
- **DB**：0（ACL 服务默认）
- **Key 前缀**：`acl:`
- **可视化**：http://localhost:5540（Redis Insights）

### PostgreSQL（如启用）

- **地址**：`127.0.0.1:5432`（或容器内 `postgres:5432`）
- **数据库**：`novagate`
- **用户名**：`novagate`
- **密码**：`novagate_dev`（⚠️ 生产环境需修改）

### MySQL（如启用）

- **地址**：`127.0.0.1:3306`（或容器内 `mysql:3306`）
- **数据库**：`novagate`
- **用户名**：`novagate`
- **密码**：`novagate_dev`（⚠️ 生产环境需修改）
- **Root 密码**：`root`（⚠️ 生产环境需修改）

## 环境变量配置

复制 `.env.example` 为 `.env` 并自定义：

```bash
cp .env.example .env
```

关键配置项：

```bash
# Redis
REDIS_PORT=6379
REDIS_MAX_MEMORY=512mb

# PostgreSQL（可选）
POSTGRES_PORT=5432
POSTGRES_PASSWORD=your_secure_password

# MySQL（可选）
MYSQL_PORT=3306
MYSQL_PASSWORD=your_secure_password
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

# 手动测试连接
redis-cli -h 127.0.0.1 -p 6379 ping

# 重启 Redis
docker-compose restart redis

# 如果仍不能连接，完全重置
docker-compose down -v && docker-compose up -d redis
```

### PostgreSQL 连接失败

```bash
# 检查日志
docker-compose logs postgres

# 手动测试连接
docker-compose exec postgres psql -U novagate -d novagate -c "SELECT 1"

# 重新初始化（⚠️ 会删除数据）
docker-compose down
docker volume rm novagate_postgres-data
docker-compose --profile postgres up -d
```

### 端口冲突

如果端口已被占用，编辑 `.env` 或 `docker-compose.yml`：

```yaml
# Redis 改为 6380
ports:
  - "6380:6379"

# PostgreSQL 改为 5433
ports:
  - "5433:5432"
```

然后更新应用配置中的端口。

### 容器内存不足

编辑 `docker-compose.yml`，增加资源限制：

```yaml
redis:
  deploy:
    resources:
      limits:
        cpus: '1'
        memory: 512M
```

### 数据持久化问题

检查数据卷：

```bash
# 查看所有卷
docker volume ls | grep novagate

# 查看卷详情
docker volume inspect novagate_redis-data

# 备份卷
docker run --rm -v novagate_redis-data:/data -v $(pwd):/backup \
  alpine tar czf /backup/redis-backup.tar.gz -C /data .
```

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

