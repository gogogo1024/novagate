# 数据库快速参考

## 当前使用的数据库

| 数据库 | 用途 | 状态 | 文档 |
|--------|------|------|------|
| **Redis** | ACL 服务主存储 | ✅ 生产使用 | [Redis Store](services/acl/internal/acl/redis_store.go) |
| **Kafka** | 消息队列、事件流 | ✅ 生产使用 | [初始化脚本](scripts/kafka/init-topics.sh) |
| **Milvus** | 向量数据库、RAG 检索 | ✅ 生产使用 | [初始化脚本](scripts/milvus/init-collections.py) |
| PostgreSQL | 预留（会话/审计日志） | 🔮 未使用 | [初始化脚本](scripts/db/init-postgres.sql) |
| MySQL | 预留（备选关系型DB） | 🔮 未使用 | [初始化脚本](scripts/db/init-mysql.sql) |

## 一键命令

```bash
# 查看数据库连接信息
./scripts/db.sh info

# 启动 Redis（最常用）
./scripts/db.sh start redis
# 或
docker-compose up -d redis

# 启动所有数据库（开发/测试）
./scripts/db.sh start all

# 启动 Kafka
./scripts/db.sh start kafka

# 启动 Milvus（含 etcd + MinIO）
./scripts/db.sh start milvus

# 查看状态
./scripts/db.sh status

# 连接到 Redis CLI
./scripts/db.sh redis-cli
# 或
docker-compose exec redis redis-cli

# 备份所有数据库
./scripts/db.sh backup

# 停止（保留数据）
./scripts/db.sh stop

# 完全清理（删除数据）
./scripts/db.sh clean
```

## Redis 使用（ACL 服务）

### 配置

**services/acl/config.example.yaml**:
```yaml
redis:
  addr: "127.0.0.1:6379"  # 本地开发
  # addr: "redis:6379"    # Docker 容器内
  password: ""
  db: 0
  key_prefix: "acl:"
```

### Key 结构

```
acl:tenant:{tenant_id}:doc:{doc_id}:vis        → "public"|"private"
acl:tenant:{tenant_id}:doc:{doc_id}:users      → Set<user_id>
acl:tenant:{tenant_id}:doc:{doc_id}:expires    → ZSet<user_id, expiry_unix>
acl:tenant:{tenant_id}:user:{user_id}:docs     → Set<doc_id>
```

### 常用操作

```bash
# 查看所有 ACL key
./scripts/db.sh redis-cli KEYS "acl:*"

# 查看某个文档的可见性
./scripts/db.sh redis-cli GET "acl:tenant:xxx:doc:yyy:vis"

# 查看某个用户的授权文档
./scripts/db.sh redis-cli SMEMBERS "acl:tenant:xxx:user:zzz:docs"

# 查看过期时间
./scripts/db.sh redis-cli ZRANGE "acl:tenant:xxx:doc:yyy:expires" 0 -1 WITHSCORES

# 清空所有 ACL 数据
./scripts/db.sh redis-cli --scan --pattern "acl:*" | xargs ./scripts/db.sh redis-cli DEL

# 查看内存使用
./scripts/db.sh redis-cli INFO memory

# 监控实时命令
./scripts/db.sh redis-cli MONITOR
```

## PostgreSQL（预留）

### 启动

```bash
docker-compose --profile postgres up -d
```

### 连接

```bash
# CLI
./scripts/db.sh postgres-cli

# 或直接连接
psql -h 127.0.0.1 -p 5432 -U novagate -d novagate
```

### 预定义表

- `sessions`：会话管理（如需实现多用户会话）
- `acl_audit_log`：ACL 操作审计日志

## MySQL（预留）

### 启动

```bash
docker-compose --profile mysql up -d
```

### 连接

```bash
# CLI
./scripts/db.sh mysql-cli

# 或直接连接
mysql -h 127.0.0.1 -P 3306 -u novagate -pnovagate_dev novagate
```

## 环境配置

### 开发环境

复制 `.env.example` 为 `.env`：

```bash
cp .env.example .env
```

关键配置：
```bash
# Redis
REDIS_PORT=6379
REDIS_MAX_MEMORY=512mb

# PostgreSQL（如使用）
POSTGRES_PORT=5432
POSTGRES_PASSWORD=change_in_prod

# MySQL（如使用）
MYSQL_PORT=3306
MYSQL_PASSWORD=change_in_prod
```

### 测试环境

使用 `docker-compose.test.yml`（无持久化，更快）：

```bash
docker-compose -f docker-compose.test.yml up -d
```

### 生产环境

⚠️ **必须修改默认密码**：

1. 编辑 `.env` 或使用环境变量
2. 修改所有 `*_PASSWORD` 配置
3. 限制数据库网络访问（不暴露到公网）
4. 配置备份策略

## 数据持久化

### 数据卷

```bash
# 查看所有卷
docker volume ls | grep novagate

# 查看卷详情
docker volume inspect novagate_redis-data

# 备份卷（手动）
docker run --rm \
  -v novagate_redis-data:/data \
  -v $(pwd)/backup:/backup \
  alpine tar czf /backup/redis-backup.tar.gz -C /data .
```

### 自动备份

使用 `./scripts/db.sh backup`：

- Redis → `backup/redis-YYYYMMDD_HHMMSS.rdb`
- PostgreSQL → `backup/postgres-YYYYMMDD_HHMMSS.sql`
- MySQL → `backup/mysql-YYYYMMDD_HHMMSS.sql`

## 监控

### Redis Insights UI

启动可视化工具：

```bash
docker-compose --profile tools up -d
```

访问：http://localhost:5540

功能：
- 实时监控（内存、命令、连接数）
- Key 浏览和编辑
- 慢查询分析
- Redis Streams 可视化

### 命令行监控

```bash
# Redis 实时统计
./scripts/db.sh redis-cli --stat

# Redis 内存分析
./scripts/db.sh redis-cli --bigkeys

# PostgreSQL 活动连接
./scripts/db.sh postgres-cli -c "SELECT * FROM pg_stat_activity;"

# MySQL 进程列表
./scripts/db.sh mysql-cli -e "SHOW PROCESSLIST;"
```

## 故障排查

### 容器无法启动

```bash
# 查看日志
docker-compose logs redis
docker-compose logs postgres

# 检查端口占用
lsof -i :6379
lsof -i :5432

# 重置（删除数据）
docker-compose down -v
docker-compose up -d
```

### Redis 内存不足

```bash
# 查看当前内存
./scripts/db.sh redis-cli INFO memory

# 修改最大内存（临时）
./scripts/db.sh redis-cli CONFIG SET maxmemory 1gb

# 永久修改：编辑 docker-compose.yml
# command: redis-server --maxmemory 1gb
```

### 连接被拒绝

```bash
# 确认容器运行
docker-compose ps

# 检查 healthcheck
docker-compose ps redis

# 测试连接
redis-cli -h 127.0.0.1 -p 6379 ping
```

## 性能优化

### Redis

```yaml
# docker-compose.yml
redis:
  command: |
    redis-server
    --appendonly yes
    --maxmemory 512mb
    --maxmemory-policy allkeys-lru
    --save ""  # 禁用 RDB（如不需要持久化）
```

### PostgreSQL

```yaml
postgres:
  command: |
    postgres
    -c shared_buffers=256MB
    -c max_connections=100
    -c work_mem=16MB
```

## Kafka（消息队列）

### 启动

```bash
docker-compose --profile kafka up -d
```

### 连接信息

- **Bootstrap Server**：`localhost:9092`
- **管理界面**：http://localhost:8080 (Kafka UI)
- **Zookeeper**：`localhost:2181`（内部依赖）

### 默认 Topics

运行初始化脚本创建预定义 topics：

```bash
docker-compose exec kafka bash /scripts/kafka/init-topics.sh
```

Topics：
- `novagate.gateway.events`：网关事件（3 分区，7 天保留）
- `novagate.acl.audit`：ACL 审计日志（3 分区，30 天保留，compact）
- `novagate.vector.updates`：向量索引更新（6 分区，1 天保留）
- `novagate.rag.queries`：RAG 查询事件（3 分区，7 天保留）

### 常用操作

```bash
# 列出所有 topics
./scripts/db.sh kafka-cli topics

# 创建 topic
./scripts/db.sh kafka-cli create my-topic 3 1
# 参数：topic名, 分区数, 副本因子

# 生产消息
./scripts/db.sh kafka-cli produce my-topic
# 输入消息，Ctrl+C 停止

# 消费消息
./scripts/db.sh kafka-cli consume my-topic
# 从头开始消费，Ctrl+C 停止

# 查看 topic 详情
docker-compose exec kafka kafka-topics \
  --bootstrap-server localhost:9092 \
  --describe --topic my-topic

# 删除 topic
docker-compose exec kafka kafka-topics \
  --bootstrap-server localhost:9092 \
  --delete --topic my-topic
```

### Go 客户端示例

```go
import "github.com/segmentio/kafka-go"

// Producer
writer := &kafka.Writer{
    Addr:     kafka.TCP("localhost:9092"),
    Topic:    "novagate.gateway.events",
    Balancer: &kafka.LeastBytes{},
}

err := writer.WriteMessages(context.Background(),
    kafka.Message{
        Key:   []byte("key"),
        Value: []byte("value"),
    },
)

// Consumer
reader := kafka.NewReader(kafka.ReaderConfig{
    Brokers: []string{"localhost:9092"},
    Topic:   "novagate.gateway.events",
    GroupID: "my-group",
})

for {
    msg, err := reader.ReadMessage(context.Background())
    if err != nil {
        break
    }
    fmt.Printf("message: %s\n", string(msg.Value))
}
```

## Milvus（向量数据库）

### 启动

```bash
docker-compose --profile milvus up -d
```

等待约 30 秒让所有依赖（etcd, MinIO）就绪。

### 连接信息

- **Endpoint**：`localhost:19530`（gRPC）
- **Metric API**：`localhost:9091`
- **管理界面**：http://localhost:8000 (Attu)
- **MinIO API**：http://localhost:9000
- **MinIO Console**：http://localhost:9001（minioadmin / minioadmin）

### 初始化集合

运行初始化脚本创建 RAG 向量集合：

```bash
# 安装依赖
pip install pymilvus

# 运行初始化
python3 scripts/milvus/init-collections.py
```

创建的集合：
- `novagate_rag_documents`：文档级向量（1536 维，OpenAI ada-002）
- `novagate_rag_sentences`（可选）：句子级向量（768 维，sentence-transformers）

### Python 客户端示例

```python
from pymilvus import connections, Collection

# 连接
connections.connect("default", host="localhost", port="19530")

# 获取集合
collection = Collection("novagate_rag_documents")

# 插入向量
entities = [
    {
        "doc_id": "uuid-1",
        "chunk_id": "uuid-1-chunk-0",
        "tenant_id": "tenant-uuid",
        "embedding": [0.1] * 1536,  # 实际向量
        "metadata": {"title": "Document Title"}
    }
]
collection.insert(entities)
collection.flush()

# 向量检索
search_params = {"metric_type": "COSINE", "params": {"ef": 64}}
results = collection.search(
    data=[[0.1] * 1536],  # 查询向量
    anns_field="embedding",
    param=search_params,
    limit=10,
    expr="tenant_id == 'tenant-uuid'",  # 过滤条件
    output_fields=["doc_id", "chunk_id", "metadata"]
)

for hits in results:
    for hit in hits:
        print(f"ID: {hit.id}, Distance: {hit.distance}, Doc: {hit.entity.get('doc_id')}")
```

### Go 客户端示例

```go
import "github.com/milvus-io/milvus-sdk-go/v2/client"

// 连接
cli, err := client.NewGrpcClient(context.Background(), "localhost:19530")

// 加载集合
cli.LoadCollection(context.Background(), "novagate_rag_documents", false)

// 向量检索
sp, _ := entity.NewIndexHNSWSearchParam(64)
searchResult, err := cli.Search(
    context.Background(),
    "novagate_rag_documents",
    []string{},
    "tenant_id == 'tenant-uuid'",  // 过滤
    []string{"doc_id", "chunk_id"},
    []entity.Vector{entity.FloatVector(queryVector)},
    "embedding",
    entity.COSINE,
    10,
    sp,
)
```

### 管理操作

```bash
# 查看 Milvus 信息
./scripts/db.sh milvus-info

# 查看集合
docker-compose exec milvus milvus-cli
> show collections

# 查看集合详情
> describe collection -c novagate_rag_documents

# 查看索引
> show index -c novagate_rag_documents

# 查询数据量
> query -c novagate_rag_documents -e "count(*)"
```

### RAG 集成流程

1. **索引阶段**：
   - 文档切分 → 生成 embedding → 插入 Milvus
   - 同时存储 doc_id/tenant_id 用于 ACL 过滤

2. **检索阶段**：
   ```python
   # 1. 向量检索（带租户过滤）
   results = collection.search(
       data=[query_embedding],
       anns_field="embedding",
       param=search_params,
       limit=50,
       expr=f"tenant_id == '{tenant_id}'"
   )
   
   # 2. 提取 doc_ids
   doc_ids = [hit.entity.get('doc_id') for hit in results[0]]
   
   # 3. ACL 过滤
   allowed_ids = acl_check_batch(tenant_id, user_id, doc_ids)
   
   # 4. 过滤结果
   filtered_results = [
       hit for hit in results[0]
       if hit.entity.get('doc_id') in allowed_ids
   ]
   
   # 5. 回源获取文本
   texts = fetch_documents(allowed_ids)
   ```

## 相关文档

- [Docker 测试环境指南](DOCKER_TESTING.md)
- [ACL 服务文档](services/acl/README.md)
- [ACL-RAG 对接契约](acl-rag-contract.md)
- [CI/CD 指南](.github/CI_CD_GUIDE.md)

