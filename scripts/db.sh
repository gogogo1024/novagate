#!/usr/bin/env bash
# Database management utility for Novagate

set -e

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Print colored message
info() { echo -e "${BLUE}ℹ${NC} $*"; }
success() { echo -e "${GREEN}✓${NC} $*"; }
warn() { echo -e "${YELLOW}⚠${NC} $*"; }
error() { echo -e "${RED}✗${NC} $*"; }

# Check Docker
check_docker() {
    if ! command -v docker &> /dev/null || ! docker ps &> /dev/null; then
        error "Docker is not running"
        return 1
    fi
    return 0
}

# Database status
db_status() {
    info "Database container status:"
    docker-compose ps
    echo ""
    
    if docker-compose ps redis 2>/dev/null | grep -q "Up"; then
        success "Redis: Running"
        docker-compose exec redis redis-cli INFO server | grep redis_version || true
    else
        warn "Redis: Stopped"
    fi
    
    if docker-compose ps postgres 2>/dev/null | grep -q "Up"; then
        success "PostgreSQL: Running"
        docker-compose exec postgres psql -U novagate -d novagate -c "SELECT version();" -t || true
    else
        warn "PostgreSQL: Not started (use: docker-compose --profile postgres up -d)"
    fi
    
    if docker-compose ps mysql 2>/dev/null | grep -q "Up"; then
        success "MySQL: Running"
        docker-compose exec mysql mysql -u novagate -pnovagate_dev -e "SELECT VERSION();" || true
    else
        warn "MySQL: Not started (use: docker-compose --profile mysql up -d)"
    fi
    
    echo ""
    
    if docker-compose ps kafka 2>/dev/null | grep -q "Up"; then
        success "Kafka: Running"
        docker-compose exec kafka kafka-broker-api-versions --bootstrap-server localhost:9092 2>/dev/null | head -1 || true
    else
        warn "Kafka: Not started (use: docker-compose --profile kafka up -d)"
    fi
    
    if docker-compose ps milvus 2>/dev/null | grep -q "Up"; then
        success "Milvus: Running"
        echo "  Version: 2.3.3"
        echo "  Endpoint: localhost:19530"
    else
        warn "Milvus: Not started (use: docker-compose --profile milvus up -d)"
    fi
}

# Start databases
db_start() {
    local profiles="${1:-redis}"
    
    info "Starting databases: $profiles"
    
    case "$profiles" in
        all)
            docker-compose --profile postgres --profile mysql --profile kafka --profile milvus --profile tools up -d
            ;;
        redis)
            docker-compose up -d redis
            ;;
        postgres)
            docker-compose --profile postgres up -d
            ;;
        mysql)
            docker-compose --profile mysql up -d
            ;;
        kafka)
            info "Starting Kafka + Zookeeper..."
            docker-compose --profile kafka up -d
            ;;
        milvus)
            info "Starting Milvus + dependencies (etcd, MinIO)..."
            docker-compose --profile milvus up -d
            ;;
        *)
            error "Unknown profile: $profiles"
            error "Available: redis, postgres, mysql, kafka, milvus, all"
            return 1
            ;;
    esac
    
    success "Databases started"
    sleep 3
    db_status
}

# Stop databases
db_stop() {
    info "Stopping all databases..."
    docker-compose down
    success "Databases stopped"
}

# Clean databases
db_clean() {
    warn "This will DELETE all database data!"
    read -p "Are you sure? (yes/no): " confirm
    
    if [[ "$confirm" == "yes" ]]; then
        info "Cleaning up..."
        docker-compose down -v
        success "All data deleted"
    else
        info "Cancelled"
    fi
}

# Backup databases
db_backup() {
    local backup_dir="./backup"
    mkdir -p "$backup_dir"
    local timestamp=$(date +%Y%m%d_%H%M%S)
    
    info "Backing up databases to $backup_dir..."
    
    # Redis backup
    if docker-compose ps redis 2>/dev/null | grep -q "Up"; then
        info "Backing up Redis..."
        docker-compose exec redis redis-cli SAVE
        docker cp novagate-redis:/data/dump.rdb "$backup_dir/redis-$timestamp.rdb"
        success "Redis backed up: redis-$timestamp.rdb"
    fi
    
    # PostgreSQL backup
    if docker-compose ps postgres 2>/dev/null | grep -q "Up"; then
        info "Backing up PostgreSQL..."
        docker-compose exec postgres pg_dump -U novagate novagate > "$backup_dir/postgres-$timestamp.sql"
        success "PostgreSQL backed up: postgres-$timestamp.sql"
    fi
    
    # MySQL backup
    if docker-compose ps mysql 2>/dev/null | grep -q "Up"; then
        info "Backing up MySQL..."
        docker-compose exec mysql mysqldump -u novagate -pnovagate_dev novagate > "$backup_dir/mysql-$timestamp.sql"
        success "MySQL backed up: mysql-$timestamp.sql"
    fi
    
    success "Backup complete in $backup_dir"
}

# Redis CLI
redis_cli() {
    if ! docker-compose ps redis 2>/dev/null | grep -q "Up"; then
        error "Redis is not running"
        return 1
    fi
    
    info "Connecting to Redis CLI..."
    docker-compose exec redis redis-cli "$@"
}

# PostgreSQL CLI
postgres_cli() {
    if ! docker-compose ps postgres 2>/dev/null | grep -q "Up"; then
        error "PostgreSQL is not running"
        return 1
    fi
    
    info "Connecting to PostgreSQL CLI..."
    docker-compose exec postgres psql -U novagate -d novagate "$@"
}

# MySQL CLI
mysql_cli() {
    if ! docker-compose ps mysql 2>/dev/null | grep -q "Up"; then
        error "MySQL is not running"
        return 1
    fi
    
    info "Connecting to MySQL CLI..."
    docker-compose exec mysql mysql -u novagate -pnovagate_dev novagate "$@"
}

# Kafka CLI
kafka_cli() {
    if ! docker-compose ps kafka 2>/dev/null | grep -q "Up"; then
        error "Kafka is not running"
        return 1
    fi
    
    local cmd="${1:-topics}"
    shift || true
    
    case "$cmd" in
        topics)
            info "Listing Kafka topics..."
            docker-compose exec kafka kafka-topics --bootstrap-server localhost:9092 --list
            ;;
        create)
            if [[ -z "$1" ]]; then
                error "Usage: kafka-cli create <topic-name> [partitions] [replication-factor]"
                return 1
            fi
            local topic="$1"
            local partitions="${2:-1}"
            local replication="${3:-1}"
            info "Creating topic: $topic"
            docker-compose exec kafka kafka-topics --bootstrap-server localhost:9092 \
                --create --topic "$topic" --partitions "$partitions" --replication-factor "$replication"
            ;;
        produce)
            if [[ -z "$1" ]]; then
                error "Usage: kafka-cli produce <topic-name>"
                return 1
            fi
            info "Producing to topic: $1 (type messages, Ctrl+C to stop)"
            docker-compose exec -T kafka kafka-console-producer --bootstrap-server localhost:9092 --topic "$1"
            ;;
        consume)
            if [[ -z "$1" ]]; then
                error "Usage: kafka-cli consume <topic-name>"
                return 1
            fi
            info "Consuming from topic: $1 (Ctrl+C to stop)"
            docker-compose exec kafka kafka-console-consumer --bootstrap-server localhost:9092 \
                --topic "$1" --from-beginning
            ;;
        *)
            error "Unknown kafka command: $cmd"
            error "Available: topics, create, produce, consume"
            return 1
            ;;
    esac
}

# Milvus info
milvus_info() {
    if ! docker-compose ps milvus 2>/dev/null | grep -q "Up"; then
        error "Milvus is not running"
        return 1
    fi
    
    cat << 'EOF'
Milvus 向量数据库信息:

  Endpoint: localhost:19530
  Metric:   localhost:9091
  管理界面: http://localhost:8000 (Attu)
  
连接示例（Python）:
  from pymilvus import connections
  connections.connect("default", host="localhost", port="19530")

MinIO（Milvus 存储）:
  API:      http://localhost:9000
  Console:  http://localhost:9001
  User:     minioadmin
  Password: minioadmin

EOF
}

# Show connection info
db_info() {
    cat << 'EOF'
╔══════════════════════════════════════════════════════════╗
║           Novagate Database Connections                  ║
╚══════════════════════════════════════════════════════════╝

Redis (默认启动):
  Host:     127.0.0.1:6379
  Password: (none)
  DB:       0
  UI:       http://localhost:5540 (Redis Insights)
  启动:     docker-compose up -d redis
  CLI:      ./scripts/db.sh redis-cli

PostgreSQL (可选):
  Host:     127.0.0.1:5432
  Database: novagate
  User:     novagate
  Password: novagate_dev
  启动:     docker-compose --profile postgres up -d
  CLI:      ./scripts/db.sh postgres-cli

MySQL (可选):
  Host:     127.0.0.1:3306
  Database: novagate
  User:     novagate
  Password: novagate_dev
  启动:     docker-compose --profile mysql up -d
  CLI:      ./scripts/db.sh mysql-cli

Kafka (消息队列):
  Bootstrap: localhost:9092
  UI:        http://localhost:8080 (Kafka UI)
  启动:      docker-compose --profile kafka up -d
  管理:      ./scripts/db.sh kafka-cli topics
  
Milvus (向量数据库):
  Endpoint:  localhost:19530
  UI:        http://localhost:8000 (Attu)
  MinIO API: http://localhost:9000
  MinIO UI:  http://localhost:9001
  启动:      docker-compose --profile milvus up -d
  信息:      ./scripts/db.sh milvus-info

⚠️  生产环境需修改默认密码！参考 .env.example

EOF
}

# Logs
db_logs() {
    local service="${1:-redis}"
    info "Showing logs for $service..."
    docker-compose logs -f "$service"
}

# Main
main() {
    # Commands that don't need Docker
    case "${1:-help}" in
        info|help|"")
            if [[ "${1:-help}" == "info" ]]; then
                db_info
            else
                cat << 'EOF'
Usage: ./scripts/db.sh <command> [options]

Commands:
  status              显示所有数据库状态
  start [profile]     启动数据库（redis|postgres|mysql|kafka|milvus|all）
  stop                停止所有数据库（保留数据）
  clean               停止并删除所有数据（需确认）
  backup              备份所有运行中的数据库
  redis-cli [args]    连接到 Redis CLI
  postgres-cli [args] 连接到 PostgreSQL CLI
  mysql-cli [args]    连接到 MySQL CLI
  kafka-cli <cmd>     Kafka 管理（topics|create|produce|consume）
  milvus-info         显示 Milvus 连接信息
  info                显示连接信息
  logs [service]      查看日志（默认 redis）

Examples:
  # 启动 Redis（默认）
  ./scripts/db.sh start

  # 启动 Kafka
  ./scripts/db.sh start kafka

  # 启动 Milvus（含依赖）
  ./scripts/db.sh start milvus

  # 启动所有数据库
  ./scripts/db.sh start all

  # 查看状态
  ./scripts/db.sh status

  # Kafka 操作
  ./scripts/db.sh kafka-cli topics
  ./scripts/db.sh kafka-cli create my-topic 3 1
  ./scripts/db.sh kafka-cli produce my-topic
  ./scripts/db.sh kafka-cli consume my-topic

  # Milvus 信息
  ./scripts/db.sh milvus-info

  # 备份
  ./scripts/db.sh backup

  # 查看连接信息
  ./scripts/db.sh info
EOF
            fi
            return 0
            ;;
    esac
    
    # All other commands need Docker
    if ! check_docker; then
        error "Please start Docker Desktop first"
        exit 1
    fi
    
    case "${1}" in
        status)
            db_status
            ;;
        start)
            db_start "${2:-redis}"
            ;;
        stop)
            db_stop
            ;;
        clean)
            db_clean
            ;;
        backup)
            db_backup
            ;;
        redis-cli)
            shift
            redis_cli "$@"
            ;;
        postgres-cli)
            shift
            postgres_cli "$@"
            ;;
        mysql-cli)
            shift
            mysql_cli "$@"
            ;;
        kafka-cli)
            shift
            kafka_cli "$@"
            ;;
        milvus-info)
            milvus_info
            ;;
        logs)
            db_logs "${2:-redis}"
            ;;
        *)
            error "Unknown command: $1"
            error "Run './scripts/db.sh help' for usage"
            exit 1
            ;;
    esac
}

main "$@"
