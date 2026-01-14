#!/bin/bash

# Novagate Docker Compose 快速启动脚本
# 提供交互式菜单来启动不同配置的系统

set -e

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# 脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo -e "${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║         Novagate Docker Compose 快速启动工具                   ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════════════╝${NC}"

show_menu() {
    echo ""
    echo -e "${YELLOW}请选择启动模式：${NC}"
    echo ""
    echo "  1. 🚀 快速启动（Redis + 管理后台 + 网关）- 仅核心服务"
    echo "  2. 📊 完整启动（加入 Kafka 消息队列）"
    echo "  3. 🤖 RAG 启动（加入 Milvus 向量数据库）"
    echo "  4. 🎯 全功能启动（包含所有可选服务）"
    echo "  5. 🛑 停止所有服务"
    echo "  6. 🧹 清理所有服务和数据"
    echo "  7. 📋 查看服务状态"
    echo "  8. 📜 查看实时日志"
    echo "  0. ❌ 退出"
    echo ""
    read -p "请输入选项 [0-8]: " choice
}

wait_for_service() {
    local service=$1
    local timeout=${2:-60}
    local start_time=$(date +%s)
    
    echo -e "${YELLOW}⏳ 等待 $service 服务就绪...${NC}"
    
    while true; do
        if docker-compose ps "$service" 2>/dev/null | grep -q "healthy"; then
            echo -e "${GREEN}✅ $service 已就绪${NC}"
            return 0
        fi
        
        local current_time=$(date +%s)
        local elapsed=$((current_time - start_time))
        
        if [ $elapsed -gt $timeout ]; then
            echo -e "${RED}❌ $service 启动超时${NC}"
            return 1
        fi
        
        sleep 2
    done
}

launch_mode() {
    local mode=$1
    local profile=$2
    
    echo -e "${GREEN}📦 启动 Novagate ($mode)...${NC}"
    echo ""
    
    cd "$PROJECT_ROOT"
    
    case $mode in
        "quick")
            docker-compose up -d redis admin gateway
            ;;
        "kafka")
            docker-compose --profile kafka up -d
            ;;
        "milvus")
            docker-compose --profile milvus up -d
            ;;
        "all")
            docker-compose --profile kafka --profile milvus up -d
            ;;
    esac
    
    echo -e "${GREEN}✅ 服务启动中...${NC}"
    
    # 等待关键服务就绪
    wait_for_service "redis" 30
    wait_for_service "admin" 60
    
    show_summary "$mode"
}

show_summary() {
    local mode=$1
    
    echo ""
    echo -e "${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║                       🎉 启动完成                              ║${NC}"
    echo -e "${BLUE}╚════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    
    echo -e "${GREEN}📍 服务地址：${NC}"
    echo "  🌐 管理后台  http://localhost:8888"
    echo "  🔌 网关      127.0.0.1:9000"
    echo "  💾 Redis     localhost:6379"
    
    if [[ "$mode" == "kafka" ]] || [[ "$mode" == "all" ]]; then
        echo "  📨 Kafka     localhost:9092"
        echo "  🎛️  Kafka UI  http://localhost:8080"
    fi
    
    if [[ "$mode" == "milvus" ]] || [[ "$mode" == "all" ]]; then
        echo "  🤖 Milvus    localhost:19530"
        echo "  🛠️  Milvus UI http://localhost:8000"
        echo "  📦 MinIO      http://localhost:9001"
    fi
    
    echo ""
    echo -e "${GREEN}🔧 常用命令：${NC}"
    echo "  查看状态：docker-compose ps"
    echo "  查看日志：docker-compose logs -f"
    echo "  进入管理后台：docker-compose exec admin sh"
    echo "  进入 Redis：docker-compose exec redis redis-cli"
    echo ""
    
    echo -e "${YELLOW}💡 下一步操作：${NC}"
    echo "  1. 打开浏览器访问 http://localhost:8888 管理后台"
    echo "  2. 使用默认用户：user-001（Alice）、user-002（Bob）"
    echo "  3. 尝试 RAG 演示：python scripts/rag-demo.py"
    echo "  4. 查看详细指南：docs/docker-compose-guide.md"
    echo ""
}

stop_services() {
    echo -e "${YELLOW}🛑 停止所有服务...${NC}"
    cd "$PROJECT_ROOT"
    docker-compose stop
    echo -e "${GREEN}✅ 所有服务已停止${NC}"
    echo -e "${BLUE}💡 数据已保留，运行 'docker-compose up -d' 即可恢复${NC}"
}

cleanup_services() {
    echo -e "${RED}⚠️  警告：将删除所有容器和数据（不可恢复）${NC}"
    read -p "确认删除？输入 'yes' 继续: " confirm
    
    if [ "$confirm" != "yes" ]; then
        echo "✓ 已取消"
        return
    fi
    
    echo -e "${YELLOW}🧹 清理所有服务和数据...${NC}"
    cd "$PROJECT_ROOT"
    docker-compose down -v
    echo -e "${GREEN}✅ 清理完成${NC}"
}

show_status() {
    cd "$PROJECT_ROOT"
    echo ""
    docker-compose ps
    echo ""
    
    if command -v docker &> /dev/null; then
        echo -e "${GREEN}📊 容器统计：${NC}"
        docker ps --filter label=com.docker.compose.project=novagate --format "table {{.Names}}\t{{.Status}}" || echo "暂无运行的 Novagate 容器"
    fi
}

show_logs() {
    cd "$PROJECT_ROOT"
    echo -e "${YELLOW}📜 实时日志（按 Ctrl+C 退出）...${NC}"
    docker-compose logs -f
}

# 主循环
while true; do
    show_menu
    
    case $choice in
        1)
            launch_mode "quick"
            ;;
        2)
            launch_mode "kafka"
            ;;
        3)
            launch_mode "milvus"
            ;;
        4)
            launch_mode "all"
            ;;
        5)
            stop_services
            ;;
        6)
            cleanup_services
            ;;
        7)
            show_status
            ;;
        8)
            show_logs
            ;;
        0)
            echo -e "${BLUE}👋 再见！${NC}"
            exit 0
            ;;
        *)
            echo -e "${RED}❌ 无效选项，请重试${NC}"
            ;;
    esac
done
