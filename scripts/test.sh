#!/usr/bin/env bash
# Test runner script - supports both Docker and local Redis

set -e

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Config
REDIS_HOST="${REDIS_HOST:-127.0.0.1}"
REDIS_PORT="${REDIS_PORT:-6379}"
USE_DOCKER="${USE_DOCKER:-false}"

echo -e "${YELLOW}🔧 Novagate Test Runner${NC}"
echo ""

# Check if Docker is available
check_docker() {
    if command -v docker &> /dev/null && docker ps &> /dev/null; then
        return 0
    else
        return 1
    fi
}

# Start Docker Redis if needed
start_docker_redis() {
    echo -e "${YELLOW}📦 Starting Docker databases...${NC}"
    
    if ! check_docker; then
        echo -e "${RED}❌ Docker is not running. Please start Docker Desktop.${NC}"
        return 1
    fi
    
    # Check if using test compose file
    local compose_file="${COMPOSE_FILE:-docker-compose.yml}"
    local compose_cmd="docker-compose"
    
    if [[ -n "$USE_TEST_COMPOSE" ]]; then
        compose_file="docker-compose.test.yml"
        echo -e "${YELLOW}Using test configuration (no persistence)${NC}"
    fi
    
    if [[ "$compose_file" != "docker-compose.yml" ]]; then
        compose_cmd="docker-compose -f $compose_file"
    fi
    
    if $compose_cmd ps redis 2>/dev/null | grep -q "Up"; then
        echo -e "${GREEN}✓ Redis already running${NC}"
    else
        echo -e "${YELLOW}Starting Redis...${NC}"
        $compose_cmd up -d redis
        
        # Wait for healthcheck
        echo -e "${YELLOW}⏳ Waiting for Redis healthcheck...${NC}"
        for i in {1..30}; do
            if $compose_cmd exec redis redis-cli ping &> /dev/null; then
                echo -e "${GREEN}✓ Redis is ready${NC}"
                return 0
            fi
            sleep 1
        done
        echo -e "${RED}❌ Redis healthcheck timeout${NC}"
        return 1
    fi
    
    # Show optional databases
    echo -e "${YELLOW}💡 Tip: Start optional databases with:${NC}"
    echo -e "  docker-compose --profile postgres up -d  # PostgreSQL"
    echo -e "  docker-compose --profile mysql up -d     # MySQL"
}

# Test Redis connectivity
test_redis() {
    echo -e "${YELLOW}🔍 Testing Redis connectivity...${NC}"
    
    if redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" ping &> /dev/null; then
        echo -e "${GREEN}✓ Redis is accessible at $REDIS_HOST:$REDIS_PORT${NC}"
        return 0
    else
        echo -e "${RED}❌ Cannot connect to Redis at $REDIS_HOST:$REDIS_PORT${NC}"
        return 1
    fi
}

# Run tests
run_tests() {
    local target="${1:-.}"
    local pattern="${2:---run=.*}"
    
    echo ""
    echo -e "${YELLOW}🧪 Running tests: $target${NC}"
    
    cd "$(dirname "$0")"
    
    if [[ "$target" == "all" ]]; then
        # Run root module tests
        echo -e "${YELLOW}📍 Root module tests...${NC}"
        mise exec -- go test ./...
        
        # Run ACL module tests
        echo -e "${YELLOW}📍 ACL module tests...${NC}"
        cd services/acl
        go test ./...
    elif [[ "$target" == "acl" ]]; then
        cd services/acl
        go test $pattern ./...
    elif [[ "$target" == "protocol" ]]; then
        go test $pattern ./protocol
    else
        go test $pattern "$target"
    fi
}

# Main
main() {
    case "${1:-all}" in
        docker-up)
            start_docker_redis
            ;;
        docker-down)
            echo -e "${YELLOW}🛑 Stopping Docker containers...${NC}"
            docker-compose down
            echo -e "${GREEN}✓ Done${NC}"
            ;;
        docker-clean)
            echo -e "${YELLOW}🗑️  Cleaning up Docker volumes...${NC}"
            docker-compose down -v
            echo -e "${GREEN}✓ Done${NC}"
            ;;
        docker-status)
            echo -e "${YELLOW}📊 Docker container status:${NC}"
            docker-compose ps
            ;;
        db-info)
            cat << 'EOF'
📊 Database Connection Information:

Redis (默认启动):
  Host: 127.0.0.1:6379
  UI:   http://localhost:5540 (Redis Insights)

PostgreSQL (可选):
  Host: 127.0.0.1:5432
  DB:   novagate
  User: novagate
  启动: docker-compose --profile postgres up -d

MySQL (可选):
  Host: 127.0.0.1:3306
  DB:   novagate
  User: novagate
  启动: docker-compose --profile mysql up -d
EOF
            ;;
        redis-test)
            test_redis
            ;;
        test)
            start_docker_redis && run_tests "all"
            ;;
        *)
            cat << 'EOF'
Usage: ./scripts/test.sh <command> [options]

Commands:
  docker-up        启动 Redis（默认）
  docker-down      停止所有容器（保留数据）
  docker-clean     停止并删除所有数据
  docker-status    查看容器状态
  db-info          显示数据库连接信息
  redis-test       测试 Redis 连接
  test [target]    运行测试（需要 Redis）
                   - test all: 完整测试套件
                   - test acl: 仅 ACL 模块
                   - test protocol: 仅协议模块

Environment Variables:
  REDIS_HOST       Redis host (default: 127.0.0.1)
  REDIS_PORT       Redis port (default: 6379)
  USE_TEST_COMPOSE 使用 docker-compose.test.yml（无持久化）

Examples:
  # 启动 Redis 并运行所有测试
  ./scripts/test.sh docker-up
  ./scripts/test.sh test

  # 使用测试配置（更快，无持久化）
  USE_TEST_COMPOSE=1 ./scripts/test.sh docker-up
  ./scripts/test.sh test

  # 查看数据库连接信息
  ./scripts/test.sh db-info

  # 启动额外数据库
  docker-compose --profile postgres up -d
  docker-compose --profile mysql up -d
EOF
            ;;
    esac
}

main "$@"
