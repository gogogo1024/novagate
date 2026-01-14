#!/bin/bash
# 初始化数据脚本 - 在 Docker Compose 启动时被调用

set -e

REDIS_ADDR="${REDIS_ADDR:-localhost:6379}"

# 等待 Redis 就绪
echo "等待 Redis 就绪..."
max_attempts=30
attempt=0
while ! nc -z $(echo $REDIS_ADDR | cut -d: -f1) $(echo $REDIS_ADDR | cut -d: -f2); do
    attempt=$((attempt+1))
    if [ $attempt -ge $max_attempts ]; then
        echo "Redis 连接失败"
        exit 1
    fi
    sleep 1
done

echo "初始化数据..."

# 初始化 Redis 数据
redis-cli -h $(echo $REDIS_ADDR | cut -d: -f1) -p $(echo $REDIS_ADDR | cut -d: -f2) << 'EOF'
# 创建用户
HSET user:user-001 id user-001 name "Alice" email "alice@example.com" created_at "2024-01-01"
HSET user:user-002 id user-002 name "Bob" email "bob@example.com" created_at "2024-01-02"

# 创建租户
HSET tenant:tenant-001 id tenant-001 name "Acme Corp" plan "premium"

# 创建权限
SADD acl:tenant-001:user-001 doc-001 doc-002
SADD acl:tenant-001:user-002 doc-001

# 创建文档
HSET doc:doc-001 id doc-001 title "Python 最佳实践" category "programming" owner_id user-001 created_at "2024-01-10"
HSET doc:doc-002 id doc-002 title "Go 并发编程" category "programming" owner_id user-001 created_at "2024-01-11"
HSET doc:doc-003 id doc-003 title "JavaScript 框架对比" category "frontend" owner_id user-002 created_at "2024-01-12"

PING
EOF

echo "数据初始化完成"
