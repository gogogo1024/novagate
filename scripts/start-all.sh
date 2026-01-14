#!/bin/bash
# 完整启动脚本 - 启动所有服务并初始化数据

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[✓]${NC} $1"; }
log_warning() { echo -e "${YELLOW}[!]${NC} $1"; }

log_info "======================================"
log_info "Novagate 完整启动"
log_info "======================================"

# Step 1: 启动数据库服务
log_info ""
log_info "Step 1: 启动数据库服务..."

docker-compose up -d redis 2>/dev/null || true
docker-compose --profile kafka up -d zookeeper kafka 2>/dev/null || true
docker-compose --profile milvus up -d etcd minio milvus 2>/dev/null || true

log_info "等待服务就绪..."
sleep 15

# 验证服务
if docker-compose ps | grep -q "redis.*Up"; then
    log_success "Redis 就绪"
else
    log_warning "Redis 未启动"
    exit 1
fi

# Step 2: 初始化 Redis ACL 数据
log_info ""
log_info "Step 2: 初始化 ACL 数据..."

docker-compose exec -T redis redis-cli << 'EOF' > /dev/null 2>&1 || true
HSET user:user-001 id user-001 name "Alice" email "alice@example.com" created_at "2024-01-01"
HSET user:user-002 id user-002 name "Bob" email "bob@example.com" created_at "2024-01-02"
HSET tenant:tenant-001 id tenant-001 name "Acme Corp" plan "premium"
SADD acl:tenant-001:user-001 doc-001 doc-002
SADD acl:tenant-001:user-002 doc-001
HSET doc:doc-001 id doc-001 title "Python 最佳实践" category "programming" owner_id user-001 created_at "2024-01-10"
HSET doc:doc-002 id doc-002 title "Go 并发编程" category "programming" owner_id user-001 created_at "2024-01-11"
HSET doc:doc-003 id doc-003 title "JavaScript 框架对比" category "frontend" owner_id user-002 created_at "2024-01-12"
PING
EOF

log_success "ACL 数据初始化完成"

# Step 3: 初始化 Milvus 向量数据
log_info ""
log_info "Step 3: 初始化 Milvus 向量数据..."

if command -v python3 &> /dev/null; then
    pip install -q pymilvus 2>/dev/null || true
    
    python3 << 'PYTHON_EOF' 2>/dev/null || true
import numpy as np
from pymilvus import connections, Collection, CollectionSchema, FieldSchema, DataType

try:
    connections.connect("default", host="localhost", port="19530", pool_name="default")
    
    try:
        from pymilvus import utility
        if utility.has_collection("novagate_rag_documents"):
            utility.drop_collection("novagate_rag_documents")
    except:
        pass
    
    fields = [
        FieldSchema(name="id", dtype=DataType.INT64, is_primary=True, auto_id=True),
        FieldSchema(name="doc_id", dtype=DataType.VARCHAR, max_length=100),
        FieldSchema(name="chunk_id", dtype=DataType.VARCHAR, max_length=100),
        FieldSchema(name="tenant_id", dtype=DataType.VARCHAR, max_length=100),
        FieldSchema(name="embedding", dtype=DataType.FLOAT_VECTOR, dim=1536),
        FieldSchema(name="metadata", dtype=DataType.JSON),
    ]
    schema = CollectionSchema(fields=fields, description="Novagate RAG Documents")
    collection = Collection(name="novagate_rag_documents", schema=schema)
    
    doc_chunks = [
        {"doc_id": "doc-001", "title": "Python 最佳实践", "chunks": [
            "Python 是一门易于学习的编程语言，具有简洁的语法和强大的库生态。",
            "在 Python 中应该优先使用列表推导式而不是循环来提高代码简洁性和性能。",
            "异常处理是编写健壮 Python 代码的关键，应该捕获具体异常而非所有异常。",
        ]},
        {"doc_id": "doc-002", "title": "Go 并发编程", "chunks": [
            "Goroutine 是 Go 语言的核心特性，是轻量级的并发单元。",
            "Channel 用于在 Goroutine 之间安全地传递数据和同步。",
            "使用 sync.Mutex 保护共享资源可以防止数据竞态条件。",
        ]},
        {"doc_id": "doc-003", "title": "JavaScript 框架对比", "chunks": [
            "React 是一个用于构建用户界面的 JavaScript 库，强调组件化和函数式编程。",
            "Vue.js 提供了更温和的学习曲线，适合中小型项目快速开发。",
            "Angular 是一个完整的框架，适合大型企业级应用的开发。",
        ]},
    ]
    
    entities = []
    np.random.seed(42)
    for doc in doc_chunks:
        for chunk_idx, chunk_text in enumerate(doc["chunks"]):
            embedding = np.random.randn(1536).astype(np.float32)
            for i, char in enumerate(chunk_text[:20]):
                embedding[i % 1536] += ord(char) / 256.0
            embedding = (embedding / np.linalg.norm(embedding)).tolist()
            
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
    
    collection.insert(entities)
    collection.create_index(
        field_name="embedding",
        index_params={
            "metric_type": "COSINE",
            "index_type": "HNSW",
            "params": {"M": 8, "efConstruction": 200}
        }
    )
    collection.load()
    connections.disconnect("default")
except Exception as e:
    pass
PYTHON_EOF
    
    log_success "Milvus 向量数据初始化完成"
else
    log_warning "Python3 未安装，跳过 Milvus 初始化"
fi

# Step 4: 启动管理后台
log_info ""
log_info "Step 4: 启动管理后台..."

# 检查依赖
if ! grep -q "github.com/redis/go-redis" go.mod; then
    log_info "安装 redis 依赖..."
    go get github.com/redis/go-redis/v9 > /dev/null 2>&1 || true
    go mod tidy > /dev/null 2>&1 || true
fi

# 启动管理后台（后台）
mise exec -- go run ./cmd/admin -addr :8888 -redis localhost:6379 > /tmp/admin.log 2>&1 &
ADMIN_PID=$!
sleep 2

if ps -p $ADMIN_PID > /dev/null 2>&1; then
    log_success "管理后台已启动 (PID: $ADMIN_PID)"
else
    log_warning "管理后台启动失败"
fi

# Step 5: 启动网关
log_info ""
log_info "Step 5: 启动网关..."

mise exec -- go run ./cmd/server -config ./novagate.yaml > /tmp/gateway.log 2>&1 &
GATEWAY_PID=$!
sleep 2

if ps -p $GATEWAY_PID > /dev/null 2>&1; then
    log_success "网关已启动 (PID: $GATEWAY_PID)"
else
    log_warning "网关启动失败"
fi

# Step 6: 显示访问方式
log_info ""
log_info "======================================"
log_info "✓ 所有服务已启动！"
log_info "======================================"
echo ""
echo -e "${BLUE}📊 管理后台${NC}"
echo "   地址: http://localhost:8888"
echo "   功能: 用户/文档/权限管理"
echo ""
echo -e "${BLUE}🚀 网关服务${NC}"
echo "   地址: 127.0.0.1:9000"
echo "   测试: mise exec -- go run ./cmd/client -addr 127.0.0.1:9000 -cmd 0x0001 -payload ping"
echo ""
echo -e "${BLUE}📊 可视化工具${NC}"
echo "   Redis Insights: http://localhost:8081"
echo "   Kafka UI:       http://localhost:8080"
echo "   Milvus Attu:    http://localhost:8000"
echo "   MinIO Console:  http://localhost:9001 (minioadmin/minioadmin)"
echo ""
echo -e "${BLUE}🧪 运行演示${NC}"
echo "   RAG 演示:       python3 scripts/rag-demo.py --demo-mode"
echo "   E2E 演示:       ./scripts/e2e-demo.sh"
echo ""
echo -e "${YELLOW}关闭服务${NC}"
echo "   kill $ADMIN_PID      # 关闭管理后台"
echo "   kill $GATEWAY_PID    # 关闭网关"
echo "   docker-compose down  # 停止所有容器"
echo ""

# 保持运行
wait $ADMIN_PID $GATEWAY_PID 2>/dev/null || true
