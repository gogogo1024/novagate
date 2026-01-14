#!/bin/bash
# End-to-End Demo: 完整流程演示
# 包含：启动服务 → 初始化数据 → 测试调用 → 验证结果

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[✓]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[!]${NC} $1"
}

log_error() {
    echo -e "${RED}[✗]${NC} $1"
}

# ============================================================
# Step 1: 启动核心服务
# ============================================================
log_info "======================================="
log_info "Step 1: 启动所有服务"
log_info "======================================="

log_info "启动 Redis..."
docker-compose up -d redis redis-insights 2>/dev/null || true

log_info "启动 Kafka..."
docker-compose --profile kafka up -d zookeeper kafka kafka-ui 2>/dev/null || true

log_info "启动 Milvus..."
docker-compose --profile milvus up -d etcd minio milvus attu 2>/dev/null || true

log_info "等待服务就绪..."
sleep 15

# 检查服务健康
if ! docker-compose ps | grep -q "redis.*Up"; then
    log_error "Redis 未启动"
    exit 1
fi
log_success "Redis 就绪"

if ! docker-compose ps | grep -q "kafka.*Up"; then
    log_warning "Kafka 未启动，跳过"
else
    log_success "Kafka 就绪"
fi

if ! docker-compose ps | grep -q "milvus.*Up"; then
    log_warning "Milvus 未启动，跳过"
else
    log_success "Milvus 就绪"
fi

# ============================================================
# Step 2: 初始化 Redis ACL 数据
# ============================================================
log_info "======================================="
log_info "Step 2: 初始化 ACL 数据到 Redis"
log_info "======================================="

# 创建用户和权限
docker-compose exec -T redis redis-cli << 'EOF'
# 创建用户
HSET user:user-001 id user-001 name "Alice" email "alice@example.com" created_at "2024-01-01"
HSET user:user-002 id user-002 name "Bob" email "bob@example.com" created_at "2024-01-02"

# 创建租户
HSET tenant:tenant-001 id tenant-001 name "Acme Corp" plan "premium"
HSET tenant:tenant-002 id tenant-002 name "Startup Inc" plan "free"

# 创建权限规则 (租户-用户-资源)
# Alice 可以访问文档 doc-001, doc-002
SADD acl:tenant-001:user-001 doc-001 doc-002

# Bob 只能访问 doc-001
SADD acl:tenant-001:user-002 doc-001

# 创建文档元数据
HSET doc:doc-001 id doc-001 title "Python 最佳实践" category "programming" owner_id user-001 created_at "2024-01-10"
HSET doc:doc-002 id doc-002 title "Go 并发编程" category "programming" owner_id user-001 created_at "2024-01-11"
HSET doc:doc-003 id doc-003 title "JavaScript 框架对比" category "frontend" owner_id user-002 created_at "2024-01-12"

# 创建中文停用词过滤规则
SADD stopwords 的 了 在 是 有 和 人 这 中 大

PING
EOF

log_success "ACL 数据已初始化"

# ============================================================
# Step 3: 初始化 Milvus 集合和向量数据
# ============================================================
log_info "======================================="
log_info "Step 3: 初始化 Milvus 向量数据"
log_info "======================================="

# 检查 Python 环境
if ! command -v python3 &> /dev/null; then
    log_warning "Python3 未安装，跳过 Milvus 初始化"
else
    # 安装 pymilvus（如果未安装）
    pip install -q pymilvus 2>/dev/null || log_warning "pymilvus 安装失败"
    
    python3 << 'PYTHON_EOF'
import sys
import time
import numpy as np
from pymilvus import connections, Collection, CollectionSchema, FieldSchema, DataType
import json

try:
    # 连接 Milvus
    connections.connect("default", host="localhost", port="19530", pool_name="default")
    print("[INFO] 连接 Milvus...")
    
    # 检查并删除已有集合
    try:
        from pymilvus import utility
        if utility.has_collection("novagate_rag_documents"):
            utility.drop_collection("novagate_rag_documents")
            print("[INFO] 删除已有集合...")
    except:
        pass
    
    # 创建集合 Schema
    fields = [
        FieldSchema(name="id", dtype=DataType.INT64, is_primary=True, auto_id=True),
        FieldSchema(name="doc_id", dtype=DataType.VARCHAR, max_length=100),
        FieldSchema(name="chunk_id", dtype=DataType.VARCHAR, max_length=100),
        FieldSchema(name="tenant_id", dtype=DataType.VARCHAR, max_length=100),
        FieldSchema(name="embedding", dtype=DataType.FLOAT_VECTOR, dim=1536),
        FieldSchema(name="metadata", dtype=DataType.JSON),
    ]
    schema = CollectionSchema(fields=fields, description="Novagate RAG Documents")
    
    # 创建集合
    collection = Collection(name="novagate_rag_documents", schema=schema)
    print("[✓] 集合已创建")
    
    # 准备向量数据（模拟 OpenAI embedding）
    doc_chunks = [
        {
            "doc_id": "doc-001",
            "title": "Python 最佳实践",
            "chunks": [
                "Python 是一门易于学习的编程语言，具有简洁的语法和强大的库生态。",
                "在 Python 中应该优先使用列表推导式而不是循环来提高代码简洁性和性能。",
                "异常处理是编写健壮 Python 代码的关键，应该捕获具体异常而非所有异常。",
            ]
        },
        {
            "doc_id": "doc-002",
            "title": "Go 并发编程",
            "chunks": [
                "Goroutine 是 Go 语言的核心特性，是轻量级的并发单元。",
                "Channel 用于在 Goroutine 之间安全地传递数据和同步。",
                "使用 sync.Mutex 保护共享资源可以防止数据竞态条件。",
            ]
        },
        {
            "doc_id": "doc-003",
            "title": "JavaScript 框架对比",
            "chunks": [
                "React 是一个用于构建用户界面的 JavaScript 库，强调组件化和函数式编程。",
                "Vue.js 提供了更温和的学习曲线，适合中小型项目快速开发。",
                "Angular 是一个完整的框架，适合大型企业级应用的开发。",
            ]
        }
    ]
    
    # 生成向量数据（模拟 OpenAI ada-002 1536维向量）
    entities = []
    np.random.seed(42)
    
    for doc in doc_chunks:
        for chunk_idx, chunk_text in enumerate(doc["chunks"]):
            # 模拟真实 embedding：基于文本长度的伪随机向量
            embedding = np.random.randn(1536).astype(np.float32)
            # 加入一些文本相关的"特征"
            for i, char in enumerate(chunk_text[:20]):
                embedding[i % 1536] += ord(char) / 256.0
            embedding = (embedding / np.linalg.norm(embedding)).tolist()  # 归一化
            
            entities.append({
                "doc_id": doc["doc_id"],
                "chunk_id": f"{doc['doc_id']}-chunk-{chunk_idx}",
                "tenant_id": "tenant-001",
                "embedding": embedding,
                "metadata": {
                    "title": doc["title"],
                    "chunk_idx": chunk_idx,
                    "text": chunk_text,
                    "length": len(chunk_text),
                    "category": "demo"
                }
            })
    
    # 插入数据
    insert_result = collection.insert(entities)
    print(f"[✓] 已插入 {len(entities)} 条向量数据")
    
    # 创建索引
    collection.create_index(
        field_name="embedding",
        index_params={
            "metric_type": "COSINE",
            "index_type": "HNSW",
            "params": {"M": 8, "efConstruction": 200}
        }
    )
    print("[✓] 索引已创建")
    
    # 加载集合到内存
    collection.load()
    print("[✓] 集合已加载到内存")
    
    connections.disconnect("default")
    print("[✓] Milvus 数据初始化完成")

except Exception as e:
    print(f"[✗] Milvus 初始化失败: {e}")
    sys.exit(1)

PYTHON_EOF
    
    log_success "Milvus 向量数据已初始化"
fi

# ============================================================
# Step 4: 启动网关服务
# ============================================================
log_info "======================================="
log_info "Step 4: 启动 Novagate 网关"
log_info "======================================="

# 启动网关（后台）
log_info "启动网关服务..."
mise exec -- go run ./cmd/server -config ./novagate.yaml &
GATEWAY_PID=$!
sleep 2

if ps -p $GATEWAY_PID > /dev/null; then
    log_success "网关已启动 (PID: $GATEWAY_PID)"
else
    log_error "网关启动失败"
    exit 1
fi

# ============================================================
# Step 5: 执行测试用例
# ============================================================
log_info "======================================="
log_info "Step 5: 执行测试调用"
log_info "======================================="

# Test 1: Ping（验证网关连接）
log_info "Test 1: 网关 Ping 测试..."
PING_OUTPUT=$(mise exec -- go run ./cmd/client -addr 127.0.0.1:9000 -cmd 0x0001 -payload "ping" 2>&1)
if echo "$PING_OUTPUT" | grep -q "pong"; then
    log_success "Ping 成功"
else
    log_error "Ping 失败"
    log_error "$PING_OUTPUT"
fi

# Test 2: 模拟向量检索 + ACL 过滤
log_info ""
log_info "Test 2: RAG 流程测试（向量检索 + ACL 过滤）..."
python3 << 'PYTHON_EOF'
import json
import numpy as np
from pymilvus import connections, Collection

try:
    # 连接 Milvus
    connections.connect("default", host="localhost", port="19530", pool_name="default")
    
    # 加载集合
    collection = Collection("novagate_rag_documents")
    collection.load()
    
    # 生成查询向量（关于 Python 的问题）
    query_text = "Python 编程最佳实践是什么"
    query_embedding = np.random.randn(1536).astype(np.float32)
    for i, char in enumerate(query_text[:20]):
        query_embedding[i % 1536] += ord(char) / 256.0
    query_embedding = (query_embedding / np.linalg.norm(query_embedding)).tolist()
    
    # 向量检索
    search_params = {
        "metric_type": "COSINE",
        "params": {"ef": 64}
    }
    
    results = collection.search(
        data=[query_embedding],
        anns_field="embedding",
        param=search_params,
        limit=5,
        expr="tenant_id == 'tenant-001'",
        output_fields=["doc_id", "metadata"]
    )
    
    print("[✓] 向量检索完成，找到 {} 条结果".format(len(results[0])))
    
    # 模拟 ACL 过滤（user-001 可以访问 doc-001, doc-002）
    import redis
    r = redis.Redis(host='localhost', port=6379, decode_responses=True)
    
    user_id = "user-001"
    allowed_docs = set(r.smembers(f"acl:tenant-001:{user_id}"))
    
    print(f"[INFO] 用户 {user_id} 的权限: {allowed_docs}")
    
    filtered_results = []
    for hit in results[0]:
        doc_id = hit.entity.get('doc_id')
        if doc_id in allowed_docs:
            filtered_results.append(hit)
            print(f"  ✓ {doc_id}: {hit.entity.get('metadata', {}).get('title')} (距离: {hit.distance:.4f})")
        else:
            print(f"  ✗ {doc_id}: 无权限访问")
    
    print(f"\n[✓] ACL 过滤后: {len(filtered_results)} 条可访问结果")
    connections.disconnect("default")
    
except Exception as e:
    print(f"[✗] RAG 测试失败: {e}")
    import traceback
    traceback.print_exc()

PYTHON_EOF

# Test 3: Redis ACL 查询
log_info ""
log_info "Test 3: ACL 权限查询..."
redis_output=$(docker-compose exec -T redis redis-cli << 'EOF'
HGETALL user:user-001
SMEMBERS acl:tenant-001:user-001
EOF
)

if echo "$redis_output" | grep -q "Alice"; then
    log_success "ACL 数据可访问"
else
    log_error "ACL 数据访问失败"
fi

# ============================================================
# Step 6: 显示服务访问方式
# ============================================================
log_info "======================================="
log_info "Step 6: 服务访问方式"
log_info "======================================="

echo ""
echo -e "${BLUE}网关服务:${NC}"
echo "  地址: 127.0.0.1:9000"
echo "  测试: mise exec -- go run ./cmd/client -addr 127.0.0.1:9000 -cmd 0x0001 -payload ping"
echo ""
echo -e "${BLUE}Redis (ACL 存储):${NC}"
echo "  地址: localhost:6379"
echo "  UI:   http://localhost:8081"
echo "  CLI:  docker-compose exec redis redis-cli"
echo ""
echo -e "${BLUE}Kafka (消息队列):${NC}"
echo "  地址: localhost:9092"
echo "  UI:   http://localhost:8080"
echo ""
echo -e "${BLUE}Milvus (向量数据库):${NC}"
echo "  地址: localhost:19530"
echo "  UI:   http://localhost:8000 (Attu)"
echo "  MinIO: http://localhost:9001 (minioadmin/minioadmin)"
echo ""

# ============================================================
# Step 7: 清理说明
# ============================================================
log_info "======================================="
log_info "清理资源"
log_info "======================================="

echo -e "${YELLOW}关闭网关:${NC}"
echo "  kill $GATEWAY_PID"
echo ""
echo -e "${YELLOW}停止服务:${NC}"
echo "  docker-compose down"
echo ""
echo -e "${YELLOW}完全清理（包括数据）:${NC}"
echo "  docker-compose down -v"
echo ""

log_success "演示流程完成！"
echo ""
echo "按 Ctrl+C 停止网关和服务"
echo ""

# 保持网关运行
wait $GATEWAY_PID
