# 🚀 Novagate - 5分钟快速开始

## 前置要求

- ✅ Docker & Docker Compose（19+）
- ✅ 2GB+ 可用内存

## 一键启动

```bash
# 方式 1️⃣：直接使用 Docker Compose（推荐）
docker-compose up -d

# 或使用交互式菜单
./scripts/docker-compose-launcher.sh
```

## 📊 访问服务

启动成功后，打开浏览器：

- **🌐 管理后台**：http://localhost:8888
  - 用户/文档/权限管理
  - 默认用户：user-001（Alice）、user-002（Bob）
  
- **🔌 RPC 网关**：127.0.0.1:9000
  - 用于应用对接

- **💾 Redis**：localhost:6379
  - 权限数据存储

## 🎯 常见操作

```bash
# 查看服务状态
docker-compose ps

# 查看日志
docker-compose logs -f

# 进入管理后台
docker-compose exec admin sh

# 进入 Redis CLI
docker-compose exec redis redis-cli

# 停止服务（保留数据）
docker-compose stop

# 完全清理
docker-compose down -v
```

## 📚 详细指南

- **完整启动指南**：[docker-compose-guide.md](docs/docker-compose-guide.md)
- **管理后台使用**：[admin-guide.md](docs/admin-guide.md)
- **协议文档**：[docs/protocol.md](docs/protocol.md)

## 🧪 测试演示

```bash
# RAG 演示（需要启动 Milvus）
docker-compose --profile milvus up -d
python scripts/rag-demo.py

# 网关测试
docker-compose exec gateway wget -q -O- http://localhost:9000/health
```

## 🆘 故障排查

### 服务无法启动？
```bash
# 查看详细错误
docker-compose logs admin
docker-compose logs gateway
docker-compose logs redis
```

### 端口已被占用？
```bash
# 修改 .env 文件的端口配置
echo "ADMIN_PORT=9999" >> .env
docker-compose up -d
```

### 需要重新初始化？
```bash
# 删除所有数据
docker-compose down -v

# 重新启动
docker-compose up -d
```

## 🎓 下一步

1. 打开管理后台探索 UI
2. 创建新用户和文档
3. 配置权限规则
4. 集成到你的应用

---

💡 **需要帮助？** 查看 [docs/docker-compose-guide.md](docs/docker-compose-guide.md) 了解更多高级配置。
