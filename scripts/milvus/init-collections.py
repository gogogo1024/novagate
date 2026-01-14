#!/usr/bin/env python3
"""
Milvus 初始化脚本 - 创建 RAG 向量集合

依赖：pip install pymilvus
"""

from pymilvus import connections, Collection, CollectionSchema, FieldSchema, DataType, utility
import sys

def create_rag_collection():
    """创建 RAG 文档向量集合"""
    
    # 连接 Milvus
    print("Connecting to Milvus...")
    connections.connect(
        alias="default",
        host="localhost",
        port="19530"
    )
    
    # 检查集合是否已存在
    collection_name = "novagate_rag_documents"
    if utility.has_collection(collection_name):
        print(f"Collection '{collection_name}' already exists, dropping...")
        utility.drop_collection(collection_name)
    
    # 定义字段
    fields = [
        FieldSchema(name="id", dtype=DataType.INT64, is_primary=True, auto_id=True),
        FieldSchema(name="doc_id", dtype=DataType.VARCHAR, max_length=36),  # UUID
        FieldSchema(name="chunk_id", dtype=DataType.VARCHAR, max_length=36),
        FieldSchema(name="tenant_id", dtype=DataType.VARCHAR, max_length=36),
        FieldSchema(name="embedding", dtype=DataType.FLOAT_VECTOR, dim=1536),  # OpenAI ada-002
        FieldSchema(name="metadata", dtype=DataType.JSON),  # 额外元数据
    ]
    
    # 创建 schema
    schema = CollectionSchema(
        fields=fields,
        description="Novagate RAG document embeddings",
        enable_dynamic_field=True
    )
    
    # 创建集合
    print(f"Creating collection '{collection_name}'...")
    collection = Collection(
        name=collection_name,
        schema=schema,
        using='default',
        shards_num=2
    )
    
    # 创建索引（HNSW - 高性能近似检索）
    print("Creating index...")
    index_params = {
        "metric_type": "COSINE",  # 余弦相似度
        "index_type": "HNSW",
        "params": {"M": 16, "efConstruction": 256}
    }
    collection.create_index(
        field_name="embedding",
        index_params=index_params
    )
    
    # 加载到内存
    print("Loading collection...")
    collection.load()
    
    print(f"✓ Collection '{collection_name}' created successfully!")
    print(f"  - Primary key: id (auto)")
    print(f"  - Vector dimension: 1536")
    print(f"  - Index: HNSW (COSINE)")
    print(f"  - Shards: 2")
    
    # 列出所有集合
    print("\nAvailable collections:")
    for coll in utility.list_collections():
        print(f"  - {coll}")
    
    connections.disconnect("default")

def create_sentence_collection():
    """创建句子级向量集合（可选）"""
    
    connections.connect(alias="default", host="localhost", port="19530")
    
    collection_name = "novagate_rag_sentences"
    if utility.has_collection(collection_name):
        print(f"Collection '{collection_name}' already exists, dropping...")
        utility.drop_collection(collection_name)
    
    fields = [
        FieldSchema(name="id", dtype=DataType.INT64, is_primary=True, auto_id=True),
        FieldSchema(name="sentence_id", dtype=DataType.VARCHAR, max_length=36),
        FieldSchema(name="doc_id", dtype=DataType.VARCHAR, max_length=36),
        FieldSchema(name="tenant_id", dtype=DataType.VARCHAR, max_length=36),
        FieldSchema(name="embedding", dtype=DataType.FLOAT_VECTOR, dim=768),  # sentence-transformers
        FieldSchema(name="position", dtype=DataType.INT32),  # 句子在文档中的位置
    ]
    
    schema = CollectionSchema(fields=fields, description="Sentence-level embeddings")
    collection = Collection(name=collection_name, schema=schema, shards_num=2)
    
    index_params = {
        "metric_type": "IP",  # 内积
        "index_type": "IVF_FLAT",
        "params": {"nlist": 1024}
    }
    collection.create_index(field_name="embedding", index_params=index_params)
    collection.load()
    
    print(f"✓ Collection '{collection_name}' created successfully!")
    connections.disconnect("default")

if __name__ == "__main__":
    try:
        print("=" * 60)
        print("Milvus Initialization for Novagate RAG")
        print("=" * 60)
        print()
        
        # 创建文档级集合
        create_rag_collection()
        
        print()
        
        # 可选：创建句子级集合
        # create_sentence_collection()
        
        print()
        print("✓ Initialization complete!")
        print("  Access Attu UI: http://localhost:8000")
        
    except Exception as e:
        print(f"✗ Error: {e}", file=sys.stderr)
        sys.exit(1)
