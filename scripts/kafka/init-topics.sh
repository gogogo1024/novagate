#!/usr/bin/env bash
# Kafka 初始化脚本 - 创建默认 topics

set -e

# 等待 Kafka 就绪
echo "Waiting for Kafka to be ready..."
sleep 10

# 创建默认 topics
echo "Creating default topics..."

# 网关事件 topic
kafka-topics --bootstrap-server localhost:9092 \
  --create --if-not-exists \
  --topic novagate.gateway.events \
  --partitions 3 \
  --replication-factor 1 \
  --config retention.ms=604800000 \
  --config segment.ms=86400000

# ACL 审计日志 topic
kafka-topics --bootstrap-server localhost:9092 \
  --create --if-not-exists \
  --topic novagate.acl.audit \
  --partitions 3 \
  --replication-factor 1 \
  --config retention.ms=2592000000 \
  --config cleanup.policy=compact

# 向量索引更新 topic（用于 Milvus 数据同步）
kafka-topics --bootstrap-server localhost:9092 \
  --create --if-not-exists \
  --topic novagate.vector.updates \
  --partitions 6 \
  --replication-factor 1 \
  --config retention.ms=86400000

# RAG 查询事件
kafka-topics --bootstrap-server localhost:9092 \
  --create --if-not-exists \
  --topic novagate.rag.queries \
  --partitions 3 \
  --replication-factor 1 \
  --config retention.ms=604800000

echo "Topics created successfully!"

# 列出所有 topics
echo ""
echo "Available topics:"
kafka-topics --bootstrap-server localhost:9092 --list
