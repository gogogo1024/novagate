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
    echo -e "${YELLOW}📦 Starting Docker Redis...${NC}"
    
    if ! check_docker; then
        echo -e "${RED}❌ Docker is not running. Please start Docker Desktop.${NC}"
        return 1
    fi
    
    if docker-compose ps redis 2>/dev/null | grep -q "Up"; then
        echo -e "${GREEN}✓ Redis already running in Docker${NC}"
    else
        docker-compose up -d redis
        # Wait for healthcheck
        echo -e "${YELLOW}⏳ Waiting for Redis healthcheck...${NC}"
        for i in {1..30}; do
            if docker-compose exec redis redis-cli ping &> /dev/null; then
                echo -e "${GREEN}✓ Redis is ready${NC}"
                return 0
            fi
            sleep 1
        done
        echo -e "${RED}❌ Redis healthcheck timeout${NC}"
        return 1
    fi
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
  docker-up        Start Redis in Docker
  docker-down      Stop Docker containers (keep data)
  docker-clean     Stop and remove all Docker data
  redis-test       Test Redis connectivity
  test [target]    Run tests (requires Redis running)
                   - test all: full suite
                   - test acl: ACL module only
                   - test protocol: protocol module only

Environment Variables:
  REDIS_HOST       Redis host (default: 127.0.0.1)
  REDIS_PORT       Redis port (default: 6379)

Examples:
  # Start Docker and run all tests
  ./scripts/test.sh docker-up
  ./scripts/test.sh test

  # Just run ACL tests
  ./scripts/test.sh test acl

  # Use local Redis (already running)
  ./scripts/test.sh test all
EOF
            ;;
    esac
}

main "$@"
