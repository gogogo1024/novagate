# Docker Compose 完整启动指南

使用 Docker Compose 一键启动完整的 Novagate 系统。

## 一键启动（推荐）

```bash
# 启动所有服务（管理后台 + 网关 + 所有数据库）
docker-compose up -d

# 查看启动状态
docker-compose ps

# 查看日志
docker-compose logs -f
```

## 按需启动

### 仅启动基础服务（Redis + 管理后台 + 网关）

```bash
docker-compose up -d redis admin gateway
```

### 启动完整系统（包含 Kafka 和 Milvus）

```bash
# 启动所有服务
docker-compose --profile kafka --profile milvus up -d

# 或使用环境变量
COMPOSE_PROFILES=kafka,milvus docker-compose up -d
```

## 服务地址

| 服务 | 地址 | 说明 |
|------|------|------|
| **管理后台** | http://localhost:8888 | Web UI - 用户/文档/权限管理 |
| **网关** | 127.0.0.1:9000 | RPC 入口 |
| **Redis** | localhost:6379 | 权限存储 |
| **Kafka** | localhost:9092 | 消息队列（可选） |
| **Milvus** | localhost:19530 | 向量数据库（可选） |
| **Kafka UI** | http://localhost:8080 | Kafka 管理工具 |
| **Milvus Attu** | http://localhost:8000 | Milvus 管理工具 |
| **MinIO** | http://localhost:9001 | 对象存储（minioadmin/minioadmin） |

## 常见命令

### 启动和停止

```bash
# 启动所有服务
docker-compose up -d

# 停止所有服务（保留数据）
docker-compose stop

# 删除容器（保留数据卷）
docker-compose down

# 完全清理（删除所有数据）
docker-compose down -v

# 重启特定服务
docker-compose restart admin
docker-compose restart gateway
```

### 查看日志

```bash
# 查看所有日志
docker-compose logs

# 实时查看所有日志
docker-compose logs -f

# 查看特定服务日志
docker-compose logs -f admin
docker-compose logs -f gateway
docker-compose logs -f redis

# 显示最后 100 行
docker-compose logs --tail=100
```

### 进入容器

```bash
# 进入管理后台容器
docker-compose exec admin sh

# 进入网关容器
docker-compose exec gateway sh

# 进入 Redis 容器
docker-compose exec redis sh

# 进入 Redis CLI
docker-compose exec redis redis-cli
```

### 构建和更新镜像

```bash
# 重新构建镜像
docker-compose build

# 构建特定服务镜像
docker-compose build admin
docker-compose build gateway

# 启动时强制重新构建
docker-compose up -d --build
```

## 初始数据

Docker Compose 启动时会自动初始化以下数据到 Redis：

### 用户
```
user-001: Alice (alice@example.com)
user-002: Bob (bob@example.com)
```

### 文档
```
doc-001: Python 最佳实践
doc-002: Go 并发编程
doc-003: JavaScript 框架对比
```

### 权限规则
```
Alice: 可访问 doc-001, doc-002
Bob: 可访问 doc-001
```

## 快速验证

### 1. 检查服务健康状态

```bash
docker-compose ps
```

预期输出：所有服务显示 `Up` 和绿色的 `healthy`

### 2. 测试管理后台

```bash
curl http://localhost:8888/api/users
```

预期输出：返回用户列表 JSON

### 3. 测试网关

```bash
docker-compose exec -T gateway wget -O- http://localhost:9000/health 2>/dev/null || echo "网关运行中"
```

### 4. 测试 Redis

```bash
docker-compose exec redis redis-cli PING
```

预期输出：`PONG`

## 使用管理后台

1. 打开浏览器：http://localhost:8888
2. 功能菜单：
   - 👥 **用户管理** - 创建/删除用户
   - 📄 **文档管理** - 创建/删除文档
   - 🔒 **权限管理** - 授予/撤销权限
   - 📊 **仪表板** - 实时统计

## 环境变量配置

可以通过 `.env` 文件自定义端口：

```env
# 端口配置
REDIS_PORT=6379
ADMIN_PORT=8888
KAFKA_PORT=9092
MILVUS_PORT=19530

# Redis 配置
REDIS_MAX_MEMORY=256mb
REDIS_INSIGHTS_PORT=5540

# 数据库配置
POSTGRES_PORT=5432
POSTGRES_DB=novagate
POSTGRES_USER=novagate
POSTGRES_PASSWORD=novagate_dev

MYSQL_PORT=3306
MYSQL_DATABASE=novagate
MYSQL_ROOT_PASSWORD=novagate_dev
```

## 多种启动场景

### 场景 1：开发环境（仅核心服务）

```bash
docker-compose up -d redis admin gateway
```

**包含**：Redis + 管理后台 + 网关

**用途**：快速开发和测试

### 场景 2：完整测试（包含向量数据库）

```bash
docker-compose --profile milvus up -d
```

**包含**：上述服务 + Milvus + etcd + MinIO

**用途**：测试 RAG 功能

### 场景 3：完整演示（包含消息队列）

```bash
docker-compose --profile kafka --profile milvus up -d
```

**包含**：所有服务

**用途**：完整系统演示

## 故障排查

### 服务无法启动

```bash
# 查看详细错误信息
docker-compose logs <service_name>

# 例如：
docker-compose logs admin
docker-compose logs gateway
```

### 端口冲突

```bash
# 修改 .env 文件中的端口
ADMIN_PORT=9999
docker-compose up -d
```

### 构建失败

```bash
# 清理旧的镜像和容器
docker-compose down -v
docker system prune -a

# 重新构建
docker-compose build --no-cache
docker-compose up -d
```

### 服务间通信问题

```bash
# 检查网络
docker network inspect novagate-network

# 验证 DNS 解析
docker-compose exec admin nslookup redis
docker-compose exec gateway nslookup redis
```

## 高级配置

### 自定义网络

编辑 `docker-compose.yml` 的 `networks` 部分：

```yaml
networks:
  novagate-network:
    driver: bridge
    ipam:
      config:
        - subnet: 172.28.0.0/16
```

### 持久化数据

所有数据都存储在 named volumes 中：

```bash
# 查看所有数据卷
docker volume ls | grep novagate

# 备份数据
docker run --rm -v redis-data:/data -v $(pwd):/backup \
  alpine tar czf /backup/redis-backup.tar.gz -C /data .

# 恢复数据
docker run --rm -v redis-data:/data -v $(pwd):/backup \
  alpine tar xzf /backup/redis-backup.tar.gz -C /data
```

## 下一步

1. **修改初始数据** - 编辑 `scripts/init-redis.sh`
2. **自定义配置** - 修改 `novagate.yaml`
3. **扩展功能** - 在 `internal/admin/service.go` 添加新 API
4. **生产部署** - 使用 Kubernetes 或其他编排工具

## 参考资源

- [快速开始指南](quick-start.md)
- [管理后台指南](admin-guide.md)
- [Docker Compose 官方文档](https://docs.docker.com/compose/)
