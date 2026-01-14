# Kafka + Milvus 快速上手

## 概览

本指南帮助你快速启动并使用 Kafka 和 Milvus。

## 前提条件

- Docker Desktop 已启动
- 系统内存 ≥ 8GB（Milvus 需要约 4GB）

## 启动数据库

### 1. Kafka（消息队列）

```bash
# 启动 Kafka + Zookeeper + Kafka UI
./scripts/db.sh start kafka

# 等待约 10 秒让服务就绪，然后验证
./scripts/db.sh kafka-cli topics
```

**访问界面**：http://localhost:8080

### 2. Milvus（向量数据库）

```bash
# 启动 Milvus + etcd + MinIO + Attu
./scripts/db.sh start milvus

# 等待约 30 秒让所有依赖就绪，然后验证
./scripts/db.sh milvus-info
```

**访问界面**：
- Attu（Milvus 管理）：http://localhost:8000
- MinIO Console：http://localhost:9001（minioadmin/minioadmin）

## Kafka 快速示例

### 创建 Topic

```bash
# 方式 1：使用管理脚本
./scripts/db.sh kafka-cli create my-topic 3 1

# 方式 2：直接运行初始化脚本（创建所有预定义 topics）
docker cp scripts/kafka/init-topics.sh novagate-kafka:/tmp/
docker-compose exec kafka bash /tmp/init-topics.sh
```

### 生产和消费消息

```bash
# 生产消息（在终端中输入，按回车发送）
./scripts/db.sh kafka-cli produce my-topic
> Hello Novagate
> Message 2
> ^C

# 消费消息（从头开始）
./scripts/db.sh kafka-cli consume my-topic
```

### Go 代码示例

```go
package main

import (
    "context"
    "fmt"
    "github.com/segmentio/kafka-go"
)

func main() {
    // 生产者
    writer := &kafka.Writer{
        Addr:     kafka.TCP("localhost:9092"),
        Topic:    "my-topic",
        Balancer: &kafka.LeastBytes{},
    }
    defer writer.Close()

    err := writer.WriteMessages(context.Background(),
        kafka.Message{Key: []byte("key1"), Value: []byte("Hello Kafka!")},
    )
    if err != nil {
        panic(err)
    }
    fmt.Println("Message sent!")

    // 消费者
    reader := kafka.NewReader(kafka.ReaderConfig{
        Brokers: []string{"localhost:9092"},
        Topic:   "my-topic",
        GroupID: "my-group",
    })
    defer reader.Close()

    msg, err := reader.ReadMessage(context.Background())
    if err != nil {
        panic(err)
    }
    fmt.Printf("Received: %s\n", string(msg.Value))
}
```

**安装依赖**：`go get github.com/segmentio/kafka-go`

## Milvus 快速示例

### 初始化集合

```bash
# 安装 Python 客户端
pip install pymilvus

# 运行初始化脚本（创建 RAG 集合）
python3 scripts/milvus/init-collections.py
```

### Python 代码示例

```python
from pymilvus import connections, Collection, FieldSchema, CollectionSchema, DataType
import numpy as np

# 1. 连接 Milvus
connections.connect("default", host="localhost", port="19530")

# 2. 获取集合
collection = Collection("novagate_rag_documents")
collection.load()

# 3. 插入向量数据
import uuid

entities = [
    {
        "doc_id": str(uuid.uuid4()),
        "chunk_id": str(uuid.uuid4()),
        "tenant_id": "tenant-001",
        "embedding": np.random.rand(1536).tolist(),  # 模拟 OpenAI embedding
        "metadata": {"title": "测试文档", "source": "demo"}
    }
]

insert_result = collection.insert(entities)
collection.flush()
print(f"插入成功，ID: {insert_result.primary_keys}")

# 4. 向量检索
query_vector = np.random.rand(1536).tolist()

search_params = {
    "metric_type": "COSINE",
    "params": {"ef": 64}
}

results = collection.search(
    data=[query_vector],
    anns_field="embedding",
    param=search_params,
    limit=5,
    expr="tenant_id == 'tenant-001'",  # 过滤条件
    output_fields=["doc_id", "metadata"]
)

# 5. 输出结果
for hits in results:
    for hit in hits:
        print(f"距离: {hit.distance:.4f}")
        print(f"文档ID: {hit.entity.get('doc_id')}")
        print(f"元数据: {hit.entity.get('metadata')}")
        print("-" * 40)

connections.disconnect("default")
```

### Go 代码示例

```go
package main

import (
    "context"
    "fmt"
    "github.com/milvus-io/milvus-sdk-go/v2/client"
    "github.com/milvus-io/milvus-sdk-go/v2/entity"
)

func main() {
    // 1. 连接 Milvus
    cli, err := client.NewGrpcClient(context.Background(), "localhost:19530")
    if err != nil {
        panic(err)
    }
    defer cli.Close()

    // 2. 加载集合
    err = cli.LoadCollection(context.Background(), "novagate_rag_documents", false)
    if err != nil {
        panic(err)
    }

    // 3. 准备查询向量（实际使用时替换为真实 embedding）
    queryVector := make([]float32, 1536)
    for i := range queryVector {
        queryVector[i] = 0.1
    }

    // 4. 向量检索
    sp, _ := entity.NewIndexHNSWSearchParam(64)
    searchResult, err := cli.Search(
        context.Background(),
        "novagate_rag_documents",
        []string{},
        "tenant_id == 'tenant-001'",
        []string{"doc_id", "metadata"},
        []entity.Vector{entity.FloatVector(queryVector)},
        "embedding",
        entity.COSINE,
        5,
        sp,
    )
    if err != nil {
        panic(err)
    }

    // 5. 输出结果
    for _, result := range searchResult {
        for i := 0; i < result.ResultCount; i++ {
            docID, _ := result.Fields.GetColumn("doc_id").Get(i)
            fmt.Printf("文档ID: %v, 距离: %.4f\n", docID, result.Scores[i])
        }
    }
}
```

**安装依赖**：`go get github.com/milvus-io/milvus-sdk-go/v2`

## RAG 典型流程

### 1. 索引文档（离线）

```python
# 假设已有文档和 embedding
documents = [
    {
        "doc_id": "doc-001",
        "tenant_id": "tenant-001",
        "chunks": [
            {"text": "...", "embedding": [...]},
            {"text": "...", "embedding": [...]},
        ]
    }
]

# 插入 Milvus
collection = Collection("novagate_rag_documents")
for doc in documents:
    for i, chunk in enumerate(doc["chunks"]):
        entities = [{
            "doc_id": doc["doc_id"],
            "chunk_id": f"{doc['doc_id']}-chunk-{i}",
            "tenant_id": doc["tenant_id"],
            "embedding": chunk["embedding"],
            "metadata": {"text": chunk["text"]}
        }]
        collection.insert(entities)
collection.flush()
```

### 2. 检索（在线）

```python
import requests

# 1. 向量检索
query_embedding = get_embedding(user_query)  # 调用 OpenAI API
results = collection.search(
    data=[query_embedding],
    anns_field="embedding",
    param={"metric_type": "COSINE", "params": {"ef": 64}},
    limit=50,
    expr=f"tenant_id == '{tenant_id}'"
)

# 2. 提取 doc_ids
doc_ids = list(set([hit.entity.get('doc_id') for hit in results[0]]))

# 3. ACL 过滤
acl_response = requests.post('http://localhost:8888/v1/acl/check-batch', json={
    "tenant_id": tenant_id,
    "user_id": user_id,
    "doc_ids": doc_ids
})
allowed_ids = set(acl_response.json()["allowed_doc_ids"])

# 4. 过滤结果
filtered_hits = [
    hit for hit in results[0]
    if hit.entity.get('doc_id') in allowed_ids
]

# 5. 构建上下文，调用 LLM
context = "\n\n".join([hit.entity.get('metadata')['text'] for hit in filtered_hits[:5]])
llm_response = call_llm(user_query, context)
```

## 监控与管理

### Kafka UI

访问 http://localhost:8080，可以：
- 查看所有 topics
- 查看消息内容
- 查看 consumer groups
- 管理配置

### Attu（Milvus）

访问 http://localhost:8000，可以：
- 可视化集合结构
- 执行向量检索
- 查看索引状态
- 监控性能指标

### MinIO Console

访问 http://localhost:9001（minioadmin/minioadmin），可以：
- 查看 Milvus 存储的对象
- 管理 buckets
- 查看存储使用情况

## 停止和清理

```bash
# 停止服务（保留数据）
docker-compose stop kafka milvus

# 完全清理（删除数据）
docker-compose down -v
```

## 故障排查

### Kafka 无法启动

```bash
# 查看日志
docker-compose logs kafka
docker-compose logs zookeeper

# 重启
docker-compose restart kafka
```

### Milvus 连接失败

```bash
# 检查依赖是否就绪
docker-compose ps etcd minio

# 等待更长时间（Milvus 启动较慢）
sleep 30

# 查看日志
docker-compose logs milvus
```

### 端口冲突

编辑 `.env` 修改端口：

```bash
KAFKA_PORT=9093
MILVUS_PORT=19531
```

## 下一步

- 阅读 [数据库参考文档](database-reference.md)
- 查看 [ACL-RAG 对接契约](acl-rag-contract.md)
- 探索 Kafka 和 Milvus 官方文档

