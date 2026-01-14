#!/bin/bash
# 启动管理后台服务

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT"

# 检查 go-redis 依赖
if ! grep -q "github.com/redis/go-redis" go.mod; then
    echo "[INFO] 添加 redis 依赖..."
    go get github.com/redis/go-redis/v9
    go mod tidy
fi

# 启动管理后台
echo "[INFO] 启动管理后台服务..."
mise exec -- go run ./cmd/admin -addr :8888 -redis localhost:6379

