#!/usr/bin/env python3
"""
End-to-End RAG Demo - 向量检索 + ACL 过滤演示

演示完整的 RAG 流程：
1. 用户提交查询
2. 向量化查询
3. Milvus 中向量检索
4. Redis 中 ACL 过滤
5. 返回用户可访问的结果
"""

import sys
import time
import json
import argparse
import numpy as np
from typing import List, Dict, Tuple

# 颜色输出
class Colors:
    BLUE = '\033[0;34m'
    GREEN = '\033[0;32m'
    YELLOW = '\033[1;33m'
    RED = '\033[0;31m'
    NC = '\033[0m'

def log_info(msg: str):
    print(f"{Colors.BLUE}[INFO]{Colors.NC} {msg}")

def log_success(msg: str):
    print(f"{Colors.GREEN}[✓]{Colors.NC} {msg}")

def log_warning(msg: str):
    print(f"{Colors.YELLOW}[!]{Colors.NC} {msg}")

def log_error(msg: str):
    print(f"{Colors.RED}[✗]{Colors.NC} {msg}")

def connect_redis():
    """连接 Redis"""
    try:
        import redis
        r = redis.Redis(host='localhost', port=6379, decode_responses=True)
        r.ping()
        return r
    except Exception as e:
        log_error(f"连接 Redis 失败: {e}")
        return None

def connect_milvus():
    """连接 Milvus"""
    try:
        from pymilvus import connections, Collection
        connections.connect("default", host="localhost", port="19530", pool_name="default")
        return connections, Collection
    except Exception as e:
        log_error(f"连接 Milvus 失败: {e}")
        return None, None

def embed_text(text: str) -> np.ndarray:
    """模拟文本向量化（实际应使用 OpenAI API 或其他模型）
    
    这里使用简单的伪向量化：基于文本内容生成一致的 1536 维向量
    """
    np.random.seed(hash(text) % 2**32)
    embedding = np.random.randn(1536).astype(np.float32)
    
    # 加入文本特征以提高相关性
    for i, char in enumerate(text):
        embedding[i % 1536] += ord(char) / 256.0
    
    # 归一化
    norm = np.linalg.norm(embedding)
    if norm > 0:
        embedding = embedding / norm
    
    return embedding.tolist()

def retrieve_documents(
    collections: tuple, 
    query: str, 
    tenant_id: str,
    top_k: int = 5
) -> List[Dict]:
    """从 Milvus 中检索相关文档"""
    try:
        _, Collection = collections
        collection = Collection("novagate_rag_documents")
        collection.load()
        
        # 向量化查询
        log_info(f"向量化查询: '{query}'")
        query_embedding = embed_text(query)
        
        # 执行向量检索
        log_info(f"在 Milvus 中检索 (top_{top_k})...")
        search_params = {
            "metric_type": "COSINE",
            "params": {"ef": 64}
        }
        
        results = collection.search(
            data=[query_embedding],
            anns_field="embedding",
            param=search_params,
            limit=top_k,
            expr=f"tenant_id == '{tenant_id}'",
            output_fields=["doc_id", "metadata"]
        )
        
        retrieved = []
        for hit in results[0]:
            retrieved.append({
                "doc_id": hit.entity.get('doc_id'),
                "metadata": hit.entity.get('metadata', {}),
                "distance": float(hit.distance)
            })
        
        log_success(f"检索到 {len(retrieved)} 条相关文档")
        return retrieved
    
    except Exception as e:
        log_error(f"向量检索失败: {e}")
        return []

def filter_by_acl(
    redis_conn,
    tenant_id: str,
    user_id: str,
    documents: List[Dict]
) -> Tuple[List[Dict], List[Dict]]:
    """通过 ACL 过滤用户可访问的文档"""
    try:
        # 获取用户权限
        acl_key = f"acl:{tenant_id}:{user_id}"
        allowed_docs = set(redis_conn.smembers(acl_key))
        
        if not allowed_docs:
            log_warning(f"用户 {user_id} 没有任何文档访问权限")
            return [], documents
        
        log_info(f"用户 {user_id} 的权限: {allowed_docs}")
        
        # 过滤文档
        filtered = []
        denied = []
        
        for doc in documents:
            doc_id = doc["doc_id"]
            if doc_id in allowed_docs:
                filtered.append(doc)
                log_info(f"  ✓ {doc_id}: {doc['metadata'].get('title', 'N/A')} (相似度: {doc['distance']:.4f})")
            else:
                denied.append(doc)
                log_warning(f"  ✗ {doc_id}: 无权限访问")
        
        log_success(f"ACL 过滤后: {len(filtered)} 条可访问 / {len(denied)} 条拒绝")
        return filtered, denied
    
    except Exception as e:
        log_error(f"ACL 过滤失败: {e}")
        return [], documents

def get_document_details(redis_conn: any, doc_id: str) -> Dict:
    """从 Redis 获取文档元数据"""
    try:
        doc_data = redis_conn.hgetall(f"doc:{doc_id}")
        return dict(doc_data) if doc_data else {}
    except Exception as e:
        log_error(f"获取文档详情失败: {e}")
        return {}

def rag_demo(query: str, user_id: str = "user-001", tenant_id: str = "tenant-001"):
    """完整 RAG 演示"""
    print("\n" + "=" * 70)
    print(f"RAG 演示: 查询用户权限下的文档")
    print("=" * 70)
    
    log_info(f"用户: {user_id} | 租户: {tenant_id} | 查询: '{query}'")
    print()
    
    # 第一步：连接数据库
    log_info("连接数据库...")
    redis_conn = connect_redis()
    if not redis_conn:
        return
    log_success("Redis 已连接")
    
    milvus_conns = connect_milvus()
    if milvus_conns[0] is None:
        return
    log_success("Milvus 已连接")
    print()
    
    # 第二步：向量检索
    print(f"{Colors.BLUE}--- 步骤 1: 向量检索 ---{Colors.NC}")
    retrieved_docs = retrieve_documents(milvus_conns, query, tenant_id, top_k=5)
    if not retrieved_docs:
        log_error("检索失败或无结果")
        return
    print()
    
    # 第三步：ACL 过滤
    print(f"{Colors.BLUE}--- 步骤 2: ACL 权限过滤 ---{Colors.NC}")
    filtered_docs, denied_docs = filter_by_acl(redis_conn, tenant_id, user_id, retrieved_docs)
    print()
    
    # 第四步：返回结果
    if filtered_docs:
        print(f"{Colors.BLUE}--- 步骤 3: 返回结果 ---{Colors.NC}")
        print(f"{Colors.GREEN}用户 {user_id} 可访问的结果:{Colors.NC}\n")
        
        for i, doc in enumerate(filtered_docs, 1):
            details = get_document_details(redis_conn, doc["doc_id"])
            print(f"{i}. {doc['doc_id']}")
            print(f"   标题: {details.get('title', 'N/A')}")
            print(f"   分类: {details.get('category', 'N/A')}")
            print(f"   所有者: {details.get('owner_id', 'N/A')}")
            print(f"   创建时间: {details.get('created_at', 'N/A')}")
            print(f"   相似度: {doc['distance']:.4f}")
            print()
    else:
        log_warning("在用户权限范围内没有找到相关文档")
        if denied_docs:
            print(f"\n{Colors.YELLOW}注: 发现 {len(denied_docs)} 条无权限访问的文档{Colors.NC}")
    
    print("=" * 70)
    print()

def main():
    parser = argparse.ArgumentParser(description='Novagate RAG 端到端演示')
    parser.add_argument('--query', default='Python 编程最佳实践', help='查询文本')
    parser.add_argument('--user', default='user-001', help='用户 ID')
    parser.add_argument('--tenant', default='tenant-001', help='租户 ID')
    parser.add_argument('--demo-mode', action='store_true', help='运行多个演示场景')
    
    args = parser.parse_args()
    
    if args.demo_mode:
        # 多个演示场景
        scenarios = [
            {
                "query": "Python 编程最佳实践",
                "user": "user-001",
                "description": "Alice 查询 Python 相关文档"
            },
            {
                "query": "Go 并发编程",
                "user": "user-001",
                "description": "Alice 查询 Go 相关文档"
            },
            {
                "query": "JavaScript 框架",
                "user": "user-002",
                "description": "Bob 查询 JavaScript 相关文档（权限受限）"
            },
        ]
        
        for scenario in scenarios:
            print(f"\n{Colors.BLUE}场景: {scenario['description']}{Colors.NC}")
            rag_demo(scenario["query"], scenario["user"])
            time.sleep(1)
    else:
        # 单个查询
        rag_demo(args.query, args.user, args.tenant)

if __name__ == "__main__":
    main()
