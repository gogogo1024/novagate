# Admin 管理后台开发指南

## 📋 项目结构

```
cmd/admin/
├── main.go           # HTTP 服务器入口，API 路由定义
├── service.go        # 业务逻辑（CRUD 操作）
└── config.go         # 配置管理

web/
├── index.html        # Web UI 前端（完整 SPA）
└── (可扩展：JS、CSS)
```

## 🚀 快速开始

### 1. 本地开发启动

```bash
# 方式 1：使用 Docker（推荐）
docker-compose up -d redis admin

# 方式 2：直接运行（需要本地 Redis）
go run ./cmd/admin -addr :8888 -redis localhost:6379
```

### 2. 访问管理后台

打开浏览器：http://localhost:8888

### 3. 开发工作流

```bash
# 1. 修改代码
# 编辑 cmd/admin/main.go 或 web/index.html

# 2. 本地测试（Docker 方式）
docker-compose down
docker-compose build --no-cache admin
docker-compose up -d admin

# 3. 查看日志
docker-compose logs -f admin

# 4. 测试 API
curl http://localhost:8888/api/users
```

---

## 🛠️ API 说明

### 基础信息

- **服务器地址**：http://localhost:8888
- **默认 Redis**：localhost:6379
- **可配置**：`-addr` 和 `-redis` 标志

### 数据模型

#### User（用户）
```json
{
  "id": "user-001",
  "name": "Alice",
  "email": "alice@example.com",
  "created_at": "2025-01-14T10:00:00Z",
  "tenant_id": "tenant-001"
}
```

#### Document（文档）
```json
{
  "id": "doc-001",
  "title": "Python 最佳实践",
  "content": "...",
  "created_by": "user-001",
  "created_at": "2025-01-14T10:00:00Z"
}
```

#### Permission（权限）
```json
{
  "user_id": "user-001",
  "doc_id": "doc-001",
  "granted_at": "2025-01-14T10:00:00Z"
}
```

### API 端点

#### 用户管理

##### 获取用户列表
```bash
curl http://localhost:8888/api/users
```

**响应**：
```json
[
  {
    "id": "user-001",
    "name": "Alice",
    "email": "alice@example.com",
    "created_at": "2025-01-14T10:00:00Z"
  }
]
```

##### 创建用户
```bash
curl -X POST http://localhost:8888/api/users/create \
  -H "Content-Type: application/json" \
  -d '{
    "id": "user-003",
    "name": "Charlie",
    "email": "charlie@example.com"
  }'
```

**响应**：
```json
{
  "success": true,
  "message": "User created successfully"
}
```

##### 删除用户
```bash
curl -X POST http://localhost:8888/api/users/delete \
  -H "Content-Type: application/json" \
  -d '{"id": "user-003"}'
```

#### 文档管理

##### 获取文档列表
```bash
curl http://localhost:8888/api/documents
```

##### 创建文档
```bash
curl -X POST http://localhost:8888/api/documents/create \
  -H "Content-Type: application/json" \
  -d '{
    "id": "doc-004",
    "title": "Rust 安全编程",
    "content": "...",
    "created_by": "user-001"
  }'
```

##### 删除文档
```bash
curl -X POST http://localhost:8888/api/documents/delete \
  -H "Content-Type: application/json" \
  -d '{"id": "doc-004"}'
```

#### 权限管理

##### 获取权限列表
```bash
curl http://localhost:8888/api/permissions
```

##### 授予权限
```bash
curl -X POST http://localhost:8888/api/permissions/grant \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user-001",
    "doc_id": "doc-001"
  }'
```

##### 撤销权限
```bash
curl -X POST http://localhost:8888/api/permissions/revoke \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user-001",
    "doc_id": "doc-001"
  }'
```

#### 审计日志

##### 获取审计日志
```bash
curl http://localhost:8888/api/audit-logs
```

**响应**：
```json
[
  {
    "action": "create_user",
    "resource_id": "user-001",
    "timestamp": "2025-01-14T10:00:00Z"
  }
]
```

---

## 💻 代码开发

### 项目入口：cmd/admin/main.go

```go
package main

import (
	"flag"
	"fmt"
	"net/http"
	"github.com/redis/go-redis/v9"
)

func main() {
	addr := flag.String("addr", ":8888", "HTTP service address")
	redisAddr := flag.String("redis", "localhost:6379", "Redis address")
	flag.Parse()

	// 连接 Redis
	rdb := redis.NewClient(&redis.Options{
		Addr: *redisAddr,
	})

	// 创建服务
	svc := NewService(rdb)

	// 注册路由
	http.HandleFunc("/api/users", svc.GetUsers)
	http.HandleFunc("/api/users/create", svc.CreateUser)
	// ... 更多路由 ...

	// 启动服务器
	fmt.Printf("Admin service listening on %s\n", *addr)
	http.ListenAndServe(*addr, nil)
}
```

### 业务逻辑：cmd/admin/service.go

```go
type Service struct {
	rdb *redis.Client
}

// GetUsers 获取所有用户
func (s *Service) GetUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 从 Redis 获取所有用户
	keys := s.rdb.Keys(r.Context(), "user:*").Val()
	users := []User{}
	
	for _, key := range keys {
		user := User{}
		s.rdb.HGetAll(r.Context(), key).Scan(&user)
		users = append(users, user)
	}

	// 返回 JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

// CreateUser 创建用户
func (s *Service) CreateUser(w http.ResponseWriter, r *http.Request) {
	var user User
	json.NewDecoder(r.Body).Decode(&user)

	// 保存到 Redis
	s.rdb.HSet(r.Context(), "user:"+user.ID, 
		"id", user.ID,
		"name", user.Name,
		"email", user.Email,
	)

	// 记录审计日志
	s.LogAudit(r.Context(), "create_user", user.ID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "User created successfully",
	})
}
```

### Web UI：web/index.html

管理后台前端是一个完整的单页应用（SPA），包含 5 个模块：

#### 1. 仪表板（Dashboard）
- 统计数据展示
- 系统概览

#### 2. 用户管理（Users）
- 用户列表
- 创建/删除用户
- 用户信息编辑

#### 3. 文档管理（Documents）
- 文档列表
- 创建/删除文档
- 文档内容预览

#### 4. 权限管理（Permissions）
- 权限规则配置
- 可视化授权矩阵
- 批量权限操作

#### 5. 审计日志（Audit Logs）
- 操作日志查看
- 时间戳过滤
- 操作详情

---

## 🔧 常见扩展

### 场景 1：添加新的 API 端点

```go
// 1. 在 main.go 中添加路由
http.HandleFunc("/api/roles", svc.GetRoles)

// 2. 在 service.go 中实现处理函数
func (s *Service) GetRoles(w http.ResponseWriter, r *http.Request) {
	// 实现逻辑
	roles := []Role{}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(roles)
}

// 3. 在 web/index.html 中更新前端调用
fetch('/api/roles')
	.then(res => res.json())
	.then(data => {
		// 更新 UI
	})
```

### 场景 2：从 HTTP 改为 gRPC

```go
// 1. 定义 proto（services/admin/proto/admin.proto）
service AdminService {
	rpc GetUsers(Empty) returns (UserList) {}
	rpc CreateUser(User) returns (CreateResponse) {}
}

// 2. 生成代码
protoc --go_out=. --go-grpc_out=. services/admin/proto/*.proto

// 3. 实现 gRPC 服务
type AdminServer struct {
	pb.UnimplementedAdminServiceServer
	svc *Service
}

func (s *AdminServer) GetUsers(ctx context.Context, _ *pb.Empty) (*pb.UserList, error) {
	// 实现逻辑
	return &pb.UserList{}, nil
}

// 4. 启动 gRPC 服务器
listener, _ := net.Listen("tcp", ":9001")
grpcServer := grpc.NewServer()
pb.RegisterAdminServiceServer(grpcServer, &AdminServer{svc: svc})
grpcServer.Serve(listener)
```

### 场景 3：添加数据库支持

```go
// 1. 引入数据库库
import (
	"database/sql"
	_ "github.com/lib/pq" // PostgreSQL
)

// 2. 修改 Service 结构
type Service struct {
	rdb *redis.Client
	db  *sql.DB  // 新增数据库连接
}

// 3. 同时从 Redis 和数据库读取数据
func (s *Service) GetUsers(w http.ResponseWriter, r *http.Request) {
	// 先从 Redis 缓存读取
	// 如果不存在，从数据库查询
	// 结果写入 Redis 缓存
}
```

### 场景 4：添加认证和授权

```go
// 1. 中间件
func (s *Service) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if !s.ValidateToken(token) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// 2. 注册中间件
http.Handle("/api/", s.AuthMiddleware(http.HandlerFunc(s.HandleAPI)))
```

---

## 🧪 测试

### 单元测试

```bash
# 运行所有测试
go test ./cmd/admin/...

# 运行特定测试
go test ./cmd/admin/... -run TestGetUsers

# 生成覆盖率报告
go test -cover ./cmd/admin/...
```

### 集成测试

```bash
# 启动 Redis 容器
docker-compose up -d redis

# 运行集成测试
go test -tags=integration ./cmd/admin/...
```

### API 测试

```bash
# 启动服务
docker-compose up -d admin

# 测试 API
curl http://localhost:8888/api/users
curl -X POST http://localhost:8888/api/users/create \
  -H "Content-Type: application/json" \
  -d '{"id":"test-user","name":"Test"}'
```

---

## 📦 Docker 部署

### Dockerfile 说明

```dockerfile
# 构建阶段
FROM golang:1.25.5-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o admin ./cmd/admin

# 运行阶段
FROM alpine:latest
WORKDIR /app
COPY web/ ./web/
COPY --from=builder /app/admin /app/admin
EXPOSE 8888
CMD ["/app/admin", "-addr", ":8888", "-redis", "${REDIS_ADDR:-redis:6379}"]
```

### 构建和运行

```bash
# 构建镜像
docker build -f Dockerfile.admin -t novagate-admin:latest .

# 运行容器
docker run -d \
  -p 8888:8888 \
  -e REDIS_ADDR=redis:6379 \
  --name admin \
  novagate-admin:latest

# 或使用 Docker Compose
docker-compose build admin
docker-compose up -d admin
```

---

## 🔗 与其他服务的集成

### 与 Gateway（网关）的交互

```
Web UI → Admin HTTP API → Redis
  ↓
Gateway (RPC) → Redis (权限查询)
  ↓
后端业务逻辑
```

### 与 ACL（权限）服务的交互

```
Admin 授予权限 → Redis
  ↓
Gateway 查询权限 → Redis
  ↓
ACL 服务验证权限
```

---

## 📚 参考资源

- [Go Web 开发](https://golang.org/doc/articles/wiki/)
- [Redis 客户端库](https://github.com/redis/go-redis)
- [Docker 官方文档](https://docs.docker.com/)
- [协议文档](docs/protocol.md)

---

## 🆘 常见问题

### Q: 如何连接到远程 Redis？
```bash
go run ./cmd/admin -redis remote-redis-host:6379
```

### Q: 如何修改默认端口？
```bash
go run ./cmd/admin -addr :9999
```

### Q: Web 前端如何修改？
编辑 `web/index.html`，刷新浏览器即可（不需要重启服务）

### Q: 如何添加新的数据库表？
在 Redis 或 PostgreSQL/MySQL 中创建对应的键或表，然后在 `service.go` 中添加处理逻辑。

### Q: 如何监控服务健康？
```bash
curl http://localhost:8888/api/users
# 如果返回 200，说明服务正常
```

---

**版本**：v1.0 | **最后更新**：2025年1月
