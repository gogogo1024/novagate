# Docker Compose 命令速查表

## 📌 最常用命令（复制即用）

```bash
# 🚀 启动所有服务
docker-compose up -d

# 查看状态
docker-compose ps

# 查看日志
docker-compose logs -f

# 停止服务
docker-compose stop

# 删除服务（保留数据）
docker-compose down

# 完全清理（删除数据）
docker-compose down -v
```

## 🎯 按场景的命令

### 场景 1：开发/测试（仅核心服务）

```bash
# 启动
docker-compose up -d redis admin gateway

# 查看日志
docker-compose logs -f admin
docker-compose logs -f gateway

# 进入管理后台容器
docker-compose exec admin sh

# 访问管理后台
curl http://localhost:8888/api/users
```

### 场景 2：完整测试（加入 Kafka）

```bash
# 启动
docker-compose --profile kafka up -d

# 查看 Kafka 日志
docker-compose logs -f kafka

# 打开 Kafka UI
open http://localhost:8080
```

### 场景 3：RAG 演示（加入 Milvus）

```bash
# 启动
docker-compose --profile milvus up -d

# 或同时启动 Kafka 和 Milvus
docker-compose --profile kafka --profile milvus up -d

# 查看 Milvus 日志
docker-compose logs -f milvus

# 打开 Milvus UI
open http://localhost:8000
```

### 场景 4：全功能（所有可选服务）

```bash
docker-compose --profile kafka --profile milvus up -d
```

## 🔧 容器操作

### 基础操作

```bash
# 启动特定容器
docker-compose start redis
docker-compose start admin

# 停止特定容器
docker-compose stop admin
docker-compose stop gateway

# 重启容器
docker-compose restart admin

# 删除容器（保留数据卷）
docker-compose rm admin

# 强制重启
docker-compose up -d --force-recreate
```

### 进入容器

```bash
# 进入 admin 容器
docker-compose exec admin sh

# 进入 redis 容器
docker-compose exec redis sh

# 进入 redis-cli
docker-compose exec redis redis-cli

# 进入 gateway 容器
docker-compose exec gateway sh

# 进入 postgres
docker-compose exec postgres psql -U novagate -d novagate
```

### 查看容器信息

```bash
# 列出所有容器
docker-compose ps

# 显示详细信息
docker-compose ps -a

# 查看容器进程
docker-compose top admin

# 查看容器资源占用
docker stats
```

## 📊 日志查看

```bash
# 查看所有日志
docker-compose logs

# 实时查看所有日志
docker-compose logs -f

# 查看最后 50 行
docker-compose logs --tail=50

# 查看最后 1 小时的日志
docker-compose logs --since 1h

# 查看特定服务
docker-compose logs admin
docker-compose logs -f gateway
docker-compose logs redis

# 组合查看（多个服务）
docker-compose logs -f admin gateway redis

# 显示时间戳
docker-compose logs -t
```

## 🏗️ 构建和镜像

```bash
# 构建所有镜像
docker-compose build

# 构建特定镜像
docker-compose build admin
docker-compose build gateway

# 强制重新构建（不用缓存）
docker-compose build --no-cache

# 启动时重新构建
docker-compose up -d --build

# 查看构建历史
docker images | grep novagate

# 删除镜像
docker rmi novagate-admin:latest
```

## 💾 数据操作

### Redis 数据操作

```bash
# 进入 Redis CLI
docker-compose exec redis redis-cli

# 查看所有 Key
docker-compose exec redis redis-cli KEYS '*'

# 查看特定前缀的 Key
docker-compose exec redis redis-cli KEYS 'user:*'
docker-compose exec redis redis-cli KEYS 'doc:*'
docker-compose exec redis redis-cli KEYS 'acl:*'

# 查看 Key 的值
docker-compose exec redis redis-cli GET user:user-001
docker-compose exec redis redis-cli HGETALL user:user-001

# 清空所有数据
docker-compose exec redis redis-cli FLUSHALL

# 清空特定数据库
docker-compose exec redis redis-cli -n 0 FLUSHDB

# 导出数据
docker-compose exec redis redis-cli --rdb /tmp/dump.rdb
docker cp novagate-redis:/tmp/dump.rdb ./redis-backup.rdb

# 数据持久化状态
docker-compose exec redis redis-cli INFO persistence
```

### 数据库操作

```bash
# PostgreSQL
docker-compose exec postgres psql -U novagate -d novagate -c "SELECT * FROM users;"

# MySQL
docker-compose exec mysql mysql -u root -pnovagate_dev novagate -e "SELECT * FROM users;"
```

## 🔍 故障排查

```bash
# 检查容器健康状态
docker-compose ps
# 查看 STATUS 列，应该显示 "Up" 和 "healthy"

# 查看详细错误日志
docker-compose logs admin | grep -i error
docker-compose logs gateway | grep -i error

# 检查网络连接
docker-compose exec admin ping redis
docker-compose exec gateway ping redis

# 检查端口绑定
docker port novagate-admin
docker port novagate-gateway

# 验证网络
docker network ls | grep novagate
docker network inspect novagate-network

# 检查卷挂载
docker inspect novagate-admin | grep -A 10 Mounts
```

## 🚨 常见问题解决

```bash
# 1. 端口被占用
# 修改 .env 文件或直接指定：
docker-compose -f docker-compose.yml -e ADMIN_PORT=9999 up -d

# 2. 容器启动失败
docker-compose logs admin  # 查看错误
docker-compose rm admin    # 删除失败的容器
docker-compose build --no-cache admin  # 重新构建
docker-compose up -d admin # 重新启动

# 3. 网络问题
docker network prune  # 清理无用网络
docker-compose down   # 删除网络和容器
docker-compose up -d  # 重新创建

# 4. 磁盘空间不足
docker system prune -a  # 清理所有未使用资源
docker volume prune     # 清理未使用的卷

# 5. 内存不足
docker-compose down -v  # 停止并删除卷
# 增加 Docker 内存限制后再启动
docker-compose up -d
```

## 🌐 API 测试

```bash
# 获取用户列表
curl http://localhost:8888/api/users

# 获取文档列表
curl http://localhost:8888/api/documents

# 获取权限列表
curl http://localhost:8888/api/permissions

# 获取审计日志
curl http://localhost:8888/api/audit-logs

# 创建用户
curl -X POST http://localhost:8888/api/users/create \
  -H "Content-Type: application/json" \
  -d '{"id":"user-003","name":"Charlie","email":"charlie@example.com"}'

# 创建文档
curl -X POST http://localhost:8888/api/documents/create \
  -H "Content-Type: application/json" \
  -d '{"id":"doc-004","title":"Rust Guide","content":"..."}'

# 授予权限
curl -X POST http://localhost:8888/api/permissions/grant \
  -H "Content-Type: application/json" \
  -d '{"user_id":"user-001","doc_id":"doc-001"}'
```

## 📈 性能监控

```bash
# 查看容器资源占用
docker stats

# 查看特定容器资源占用
docker stats novagate-admin
docker stats novagate-gateway
docker stats novagate-redis

# 查看详细的资源历史
docker inspect novagate-admin
```

## 🔄 常用工作流程

### 完整的开发循环

```bash
# 1. 启动所有服务
docker-compose up -d

# 2. 等待服务就绪
sleep 10 && docker-compose ps

# 3. 查看管理后台是否可用
curl http://localhost:8888/api/users

# 4. 开发阶段
# ...修改代码...

# 5. 重建镜像
docker-compose build --no-cache

# 6. 重启服务
docker-compose up -d

# 7. 验证
curl http://localhost:8888/api/users

# 8. 清理（开发结束）
docker-compose down
```

### 数据验证

```bash
# 1. 启动服务
docker-compose up -d

# 2. 验证初始数据
docker-compose exec redis redis-cli HGETALL user:user-001

# 3. 修改数据
docker-compose exec redis redis-cli HSET user:user-001 email newemail@example.com

# 4. 导出数据备份
docker-compose exec redis redis-cli BGSAVE

# 5. 验证修改
docker-compose exec redis redis-cli HGETALL user:user-001
```

## 💡 实用脚本片段

### 监控服务健康

```bash
# 连续监控所有服务状态
watch -n 2 'docker-compose ps'
```

### 一键清理并重启

```bash
# 完全重置
docker-compose down -v && \
docker system prune -a -f && \
docker-compose up -d
```

### 导出容器日志

```bash
# 导出所有日志到文件
docker-compose logs > docker-compose.log 2>&1

# 导出特定服务日志
docker-compose logs admin > admin.log
docker-compose logs gateway > gateway.log
```

### 批量操作容器

```bash
# 重启所有服务
docker-compose restart

# 停止除了 redis 外的所有服务
docker-compose stop admin gateway

# 启动除了 redis 外的所有服务
docker-compose start admin gateway
```

---

💡 **提示**：大多数命令都可以在项目目录中执行，Docker Compose 会自动找到 `docker-compose.yml`。
