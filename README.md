# Novagate

Novagate 是一个基于 TCP 长连接的轻量协议网关骨架：

定位：**纯 RPC 网关（以 request/response 为主，长连接仅用于复用与降低开销）**。当前不提供订阅/服务端主动推送等语义（未来如需推送，建议引入单写者模型与会话/背压治理）。

- `protocol`：纯协议定义与编解码（可跨语言复用）
- `novagate`：Go 侧默认运行时实现（listener/conn loop/router）

本仓库的协议规范见：[`docs/protocol.md`](docs/protocol.md)

## 特性

- **明确的 Frame / Message 分层**：解决粘包/拆包与语义路由
- **Command 路由**：以 `uint16` 的 Command 作为协议级路由键
- **Flags 扩展位**：支持 gzip 压缩、one-way（单向消息）；加密位保留但当前拒绝
- **连接级资源控制**：内置简单的内存配额控制（防止异常流量导致内存膨胀）
- **可控的运行时行为**：支持 `context` 取消优雅停机；Accept 遇到可恢复错误会指数退避重试；连接的正常断开不刷 error 日志
- **示例可运行**：`cmd/server` + `cmd/client` 可以直接验证协议收发

## 协议概览（稳定版）

> 完整定义以 [`docs/protocol.md`](docs/protocol.md) 为准。

### Frame

- Header：8 字节
  - `Magic`：`0xCAFE`（2B）
  - `Version`：当前为 `1`（1B）
  - `Flags`：位标志（1B）
  - `Length`：Body 长度（4B，大端）
- Body：`MessageBytes`

相关实现：[`protocol/frame.go`](protocol/frame.go)

### Message

- `Command`：`uint16`（2B，大端）
- `RequestID`：`uint64`（8B，大端）
- `Payload`：bytes（可选，N 字节）

相关实现：[`protocol/message.go`](protocol/message.go)

### Flags

- Bit0：压缩（gzip）
- Bit1：加密（预留；当前实现会拒绝此位）
- Bit2：单向消息（one-way；不返回响应）

相关实现：[`protocol/compress.go`](protocol/compress.go)

## 目录结构

- `protocol/`：纯协议（Frame/Message/Flags/Command 映射）
- `cmd/server/`：**示例网关服务端** - 展示如何注册 Command、关联业务 handler、配置超时等
  - 包含完整配置加载流程（YAML + 环境变量 + flag 优先级）
  - 展示 strict command mapping 与 dispatcher 桥接的最佳实践
  - **用途**：作为实际部署的参考；或直接修改后作为生产网关启动入口
- `cmd/client/`：**协议调试工具** - TCP 层手动组包/发包/收包，用于联调与验证
  - 支持 flags（one-way、gzip）、自定义 payload、Request ID
  - **用途**：不依赖 SDK 直接测试服务端；快速验证协议实现是否正确
- `internal/`：Go 侧默认实现的内部组件（dispatcher/codec/limits/transport 等）
- `docs/`：协议与架构决策文档

## 子服务

- ACL HTTP 子服务（用于 RAG/检索场景的逐用户权限判定）：[services/acl/README.md](services/acl/README.md)
- **管理后台**（用户/权限/文档管理 Web UI）：[cmd/admin/](cmd/admin/) 和 [docs/admin-guide.md](docs/admin-guide.md)

## 完整系统（Docker Compose）

本项目支持以下服务的容器化部署：

| 服务 | 用途 | 默认 | 可选 |
|------|------|------|------|
| **Redis** | ACL 权限数据存储 | ✅ | |
| **Admin（管理后台）** | Web UI 管理用户/文档/权限 | ✅ | |
| **Gateway（网关）** | RPC 入口，TCP 长连接 | ✅ | |
| **Kafka + Zookeeper** | 消息队列 | | 📦 |
| **Milvus** | 向量数据库（RAG 检索） | | 📦 |
| PostgreSQL | 关系型数据库 | | 📦 |
| MySQL | 关系型数据库 | | 📦 |

### 🚀 一键启动（三种模式）

#### 1️⃣ 快速启动（仅核心服务）
```bash
docker-compose up -d
```
包含：Redis、管理后台、网关  
访问：http://localhost:8888

#### 2️⃣ 完整启动（加入 Kafka）
```bash
docker-compose --profile kafka up -d
```
新增：Kafka、Zookeeper、Kafka UI  
消息队列地址：localhost:9092

#### 3️⃣ 全功能启动（加入 Milvus）
```bash
docker-compose --profile kafka --profile milvus up -d
```
新增：Milvus、etcd、MinIO、Milvus Attu  
向量数据库地址：localhost:19530

### 📊 服务状态与日志

```bash
# 查看所有服务运行状态
docker-compose ps

# 实时查看日志
docker-compose logs -f

# 查看特定服务日志
docker-compose logs -f admin   # 管理后台
docker-compose logs -f gateway # 网关
```

### 🎛️ 交互式启动工具

```bash
# 使用图形菜单选择启动模式
./scripts/docker-compose-launcher.sh
```

提供的功能：
- 选择启动模式（快速/完整/全功能）
- 自动等待服务就绪并显示地址
- 查看服务状态
- 实时日志查看
- 服务启停和清理

### 📚 更多信息

详见 [Docker Compose 完整指南](docs/docker-compose-guide.md)

### 详细指南

- **完整配置**：[docker-compose.yml](docker-compose.yml)
- **管理工具**：[scripts/db.sh](scripts/db.sh)
- **快速上手**：[docs/kafka-milvus-quickstart.md](docs/kafka-milvus-quickstart.md)
- **数据库参考**：[docs/database-reference.md](docs/database-reference.md)

### 管理界面（启动后访问）

| 服务 | 地址 | 默认凭证 |
|------|------|--------|
| Kafka UI | http://localhost:8080 | - |
| Milvus Attu | http://localhost:8000 | - |
| MinIO Console | http://localhost:9001 | minioadmin/minioadmin |
| Redis Commander | http://localhost:8081 | - |

## 快速开始

### ⚡ 最快上手（5分钟）

1️⃣ **启动系统**
```bash
docker-compose up -d
```

2️⃣ **打开管理后台**
```
http://localhost:8888
```

3️⃣ **查看日志**
```bash
docker-compose logs -f admin gateway
```

详见：[QUICK_START.md](QUICK_START.md) | [docker-compose-guide.md](docs/docker-compose-guide.md)

### 🎯 三种启动模式

| 命令 | 包含服务 | 场景 | 资源 |
|------|--------|------|------|
| `docker-compose up -d` | Redis + Admin + Gateway | 💻 开发/测试 | 500MB |
| `docker-compose --profile kafka up -d` | + Kafka + Zookeeper | 📨 消息队列 | 1.5GB |
| `docker-compose --profile kafka --profile milvus up -d` | + Milvus + etcd + MinIO | 🤖 RAG 演示 | 3GB |

### 📚 文档导航

| 文档 | 说明 |
|------|------|
| [QUICK_START.md](QUICK_START.md) | ⚡ 5分钟快速开始 |
| [docs/docker-compose-guide.md](docs/docker-compose-guide.md) | 📖 完整启动指南 |
| [docs/docker-compose-cheatsheet.md](docs/docker-compose-cheatsheet.md) | 🔍 命令速查表 |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | 🏗️ 系统架构 |
| [docs/admin-guide.md](docs/admin-guide.md) | 🎛️ 管理后台使用 |
| [docs/quick-start.md](docs/quick-start.md) | 🧪 端到端演示 |
| [docs/protocol.md](docs/protocol.md) | 📋 协议文档 |

### 🚀 完整端到端演示（可选）

需要完整的自动化演示脚本：

```bash
# 启动所有服务、初始化数据、运行演示
./scripts/e2e-demo.sh

# 在另一个终端运行 RAG 演示
python3 scripts/rag-demo.py --demo-mode
```

### 📊 服务访问地址

启动后各服务访问地址：

- **🌐 管理后台**：http://localhost:8888
- **🔌 RPC 网关**：127.0.0.1:9000
- **💾 Redis**：localhost:6379
- **📨 Kafka**（可选）：localhost:9092
- **🔎 Kafka UI**（可选）：http://localhost:8080
- **🤖 Milvus**（可选）：localhost:19530
- **🎨 Milvus UI**（可选）：http://localhost:8000

### 🛠️ 使用 mise 管理 Go 版本

```bash
mise install
```

凡是依赖 Go 工具链的命令，都可以统一用 `mise exec -- go ...` 来跑：

```bash
# 基础静态检查
mise exec -- go vet ./...
mise exec -- go mod tidy

# 运行测试
mise exec -- go test ./...
```

详见：[端到端演示指南](docs/e2e-demo-guide.md)

### 运行测试

```bash
mise exec -- go test ./...
```

### 命令一致性校验（可选）

默认只校验 3 个文件：`protocol/commands.go`、`cmd/server/main.go`、`internal/service/registry.go`。

要求 `protocol/commands.go` 里的 `Cmd* uint16` 常量使用十六进制（`0x....`）以便稳定维护 ABI。

```bash
mise exec -- go run ./cmd/validate-commands
mise exec -- go run ./cmd/validate-commands -require-all
```

### Git hooks（pre-commit，可选）

本仓库提供基于 `mise` 的 `pre-commit` hook：仅当 staged 里包含 Go 相关改动时，自动运行：

- `mise exec -- go run ./cmd/validate-commands`
- `mise exec -- go test ./...`

安装：

```bash
./scripts/install-githooks.sh
```

### 启动服务端

```bash
mise exec -- go run ./cmd/server
```

默认会尝试读取当前目录下的 `novagate.yaml`（如果文件不存在会忽略）；支持的 YAML 结构（kitex 风格分组）：

你可以直接复制示例配置文件：[`novagate.yaml.example`](novagate.yaml.example) → `novagate.yaml`。

建议不要把 `novagate.yaml` 提交到 git（通常是本地/环境配置；本仓库默认也会忽略它），只提交 `novagate.yaml.example` 作为模板。

```yaml
server:
    addr: ":9000"

timeouts:
    idle: "5m"
    write: "10s"
```

如果 YAML 或环境变量里提供了非法的 duration（例如 `idle: "5x"`），服务端会直接启动失败并报错（fail-fast）。

也可以显式指定配置文件：

```bash
mise exec -- go run ./cmd/server -config ./novagate.yaml
```

也可以通过环境变量（或本地 `.env` 文件）覆盖 YAML 默认值；命令行 flag 优先级更高。

优先级：`flag > env > yaml > default`。

- `NOVAGATE_ADDR`：监听地址（默认 `:9000`）
- `NOVAGATE_IDLE_TIMEOUT`：连接空闲超时（例如 `60s`、`5m`；默认 `5m`）
- `NOVAGATE_WRITE_TIMEOUT`：响应写超时（例如 `10s`；默认 `10s`）

示例 `.env`：

```dotenv
NOVAGATE_ADDR=:9000
NOVAGATE_IDLE_TIMEOUT=60s
NOVAGATE_WRITE_TIMEOUT=10s
```

#### 远程配置与热更新（当前策略）

`cmd/server` 当前只支持**本地 YAML 配置文件**（加上 env/flag 覆盖），不内置 Consul/etcd/Nacos 等远程配置中心的读取，也不支持运行中动态 reload 立即生效。

推荐做法：

- 在部署/启动层把远程配置渲染/同步到本地文件（例如 `/etc/novagate/novagate.yaml`）。
- 启动时用 `-config` 显式指定该文件路径。
- 需要变更配置时，通过滚动重启/灰度发布生效（比“在线热更新”更可控、更易排障）。

可选：配置连接空闲超时（IdleTimeout）。连接在指定时长内没有任何读写数据时，会被服务端主动关闭：

```bash
mise exec -- go run ./cmd/server -addr :9000 -idle-timeout 60s
```

可选：配置响应写超时（WriteTimeout）。用于防止对端不读/网络卡死导致 `Write` 长时间阻塞：

```bash
mise exec -- go run ./cmd/server -addr :9000 -write-timeout 10s
```

### 运行客户端（Ping）

```bash
mise exec -- go run ./cmd/client -addr 127.0.0.1:9000 -cmd 0x0001 -payload ping
```

预期输出类似：

```text
resp: cmd=0x0001 request_id=1 payload="pong"
```

### 管理后台（可选）

```bash
# 启动管理后台（需要 Redis 运行）
./scripts/admin.sh

# 或直接运行
mise exec -- go run ./cmd/admin -addr :8888 -redis localhost:6379
```

访问：**http://localhost:8888**

功能：
- 👥 用户管理（新增、删除）
- 📄 文档管理（新增、删除）
- 🔒 权限管理（授予、撤销）
- 📋 审计日志（操作记录）

详见：[管理后台指南](docs/admin-guide.md)

### 运行客户端（Ping）

#### 1. 测试 One-way 消息（不等响应）

```bash
mise exec -- go run ./cmd/client -addr 127.0.0.1:9000 -cmd 0x0001 -payload ping -flags 0x04
```

#### 2. 测试 gzip 压缩

```bash
mise exec -- go run ./cmd/client -addr 127.0.0.1:9000 -cmd 0x0001 -payload "hello world" -flags 0x01
```

#### 3. 快速验证服务端是否启动

```bash
mise exec -- go run ./cmd/client -addr your-server:9000 -cmd 0x0001 -payload ping
```

#### 4. 与其他客户端库交互测试

当你在 Java/Python/Node.js 等其他语言实现了 Novagate 客户端后，可以用 `cmd/client` 验证跨语言协议兼容性：

```bash
# 1. 启动 Go 网关
mise exec -- go run ./cmd/server

# 2. 用 Go 客户端验证
mise exec -- go run ./cmd/client -addr 127.0.0.1:9000 -cmd 0x0001 -payload test

# 3. 用其他语言的客户端测试同样的命令
python3 my_client.py --addr 127.0.0.1:9000 --cmd 0x0001 --payload test
```

#### 5. 调试包格式问题

如果自己实现的客户端无法与服务端通信，可以：

1. 启动服务端：`mise exec -- go run ./cmd/server`
2. 用 Go 客户端测试：`mise exec -- go run ./cmd/client -addr 127.0.0.1:9000 -cmd 0x0001 -payload test`
3. 如果 Go 客户端成功，说明服务端协议实现无问题，问题在自己的客户端实现
4. 用 Wireshark/tcpdump 抓包对比 Go 客户端的字节流

**完整客户端选项**：

```bash
mise exec -- go run ./cmd/client -h
```

| 选项 | 说明 | 示例 |
|------|------|------|
| `-addr` | 服务端地址 | `127.0.0.1:9000` |
| `-cmd` | 命令（十六进制） | `0x0001`（Ping） |
| `-payload` | 请求内容 | `"hello"` |
| `-flags` | Frame flags（十六进制） | `0x01`（gzip）、`0x04`（one-way） |
| `-id` | Request ID | `42` |



## 作为库使用（Go）

### 启动一个默认网关

`novagate.ListenAndServe` 需要注入一个 `setup`，用于注册 Command 表与路由 handler：

```go
package main

import (
    "context"

    "github.com/gogogo1024/novagate"
    "github.com/gogogo1024/novagate/protocol"
)

func setup(r *novagate.Router) error {
    protocol.RegisterFullMethodCommand("NovaService.Ping", protocol.CmdPing)
    protocol.SetStrictCommandMapping(true)

    r.Register(protocol.CmdPing, novagate.BridgeProtocolHandler(protocol.CmdPing,
        func(ctx context.Context, payload []byte) ([]byte, error) {
            return []byte("pong"), nil
        }))
    return nil
}

func main() {
    _ = novagate.ListenAndServe(":9000", setup)
}
```

如果你希望启用连接空闲超时：

```go
func main() {
    _ = novagate.ListenAndServeWithOptions(":9000", setup, novagate.WithIdleTimeout(60*time.Second))
}
```

如果你希望同时启用响应写超时：

```go
func main() {
    _ = novagate.ListenAndServeWithOptions(
        ":9000",
        setup,
        novagate.WithIdleTimeout(60*time.Second),
        novagate.WithWriteTimeout(10*time.Second),
    )
}
```

如果你希望支持优雅停机（例如接收 SIGINT/SIGTERM 时退出）：

```go
func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    _ = novagate.ListenAndServeWithContext(
        ctx,
        ":9000",
        setup,
        novagate.WithIdleTimeout(60*time.Second),
        novagate.WithWriteTimeout(10*time.Second),
    )
}
```

> 注：`ListenAndServeWithContext/ServeWithContext` 会在 `ctx` 取消时关闭 listener 并退出；连接上 `handleConn` 返回 `net.ErrClosed` / `ECONNRESET` / `EPIPE` 等常见正常断开错误时不会打印 `conn error`。

### 仅使用纯协议库

如果你只想在其他项目/其他语言实现同一协议：

- Frame：`protocol.Encode` / `protocol.Decode`
- Message：`protocol.EncodeMessage` / `protocol.DecodeMessage`
- Flags 处理：`protocol.EncodeFrameBody` / `protocol.DecodeFrameBody`

## Command 映射与 strict 模式

在网关场景里，Command 是协议级路由键（`uint16`），需要在“协议端”和“业务端”保持一致。

- `protocol.RegisterFullMethodCommand(fullMethod, cmd)`：显式注册“方法名 → Command”的映射
- `protocol.SetStrictCommandMapping(true)`：开启 strict 模式
    - strict 模式下，如果没有显式注册映射，会直接报错（不做任何隐式回退）
    - 目的：避免不同语言/不同实现里使用 hash/隐式规则导致不一致或碰撞

建议：生产环境开启 strict，并把 Command 当成稳定 ABI 维护。

## 跨语言实现要点（对齐清单）

如果你要在 Java/Rust/C++/Python 等语言里实现相同协议，建议按下面清单逐项对齐：

- **字节序**：所有整数字段使用大端（Big Endian）
- **Frame Header**：固定 8 字节；`Length` 表示 Body 长度（不含 Header）
- **拆包逻辑**：必须支持半包/多包（TCP 字节流无消息边界）
- **Flags 语义**：
    - Bit0 压缩：gzip
    - Bit2 one-way：客户端不等响应；服务端也不应回写响应
    - Bit1 加密：预留；当前实现会拒绝该位
- **压缩上限**：解压后输出需要有上限（防解压炸弹）。本实现上限与 `MaxFrameBody` 一致（默认 1MB）

相关 Go 参考实现入口：`protocol.Encode/Decode`、`protocol.EncodeMessage/DecodeMessage`、`protocol.EncodeFrameBody/DecodeFrameBody`。

## FAQ

### 1) 为什么 `git push -u origin main` 会报 `src refspec main does not match any`？

通常是因为本地还没有任何 commit（`No commits yet on main`）。先 `git commit -m "init"` 再 push。

### 2) 为什么服务端 `cmd/server` 在 `go test ./...`（或 `mise exec -- go test ./...`）时不会启动？

`cmd/server` 的 `main` 做了防御：如果当前进程名以 `.test` 结尾会直接返回，避免测试时意外启动长监听。

### 3) one-way 消息为什么客户端收不到响应？

这是设计使然：one-way 表示单向投递，客户端不应等待响应；服务端也不会回写响应。

### 4) 设置了压缩位但解码失败怎么办？

确认两端都使用同一套规则处理 flags：

- 发送：先 `EncodeMessage`，再 `EncodeFrameBody(flags, msgBytes)`，最后 `Encode(Frame)`
- 接收：先 `Decode(Frame)`，再 `DecodeFrameBody(frame)`，最后 `DecodeMessage(body)`

## 设计与决策

- 协议规范：[`docs/protocol.md`](docs/protocol.md)
- 架构决策记录（ADR）：[`docs/decision.md`](docs/decision.md)
- Thrift（示例 IDL）：[`api/idl/nova.thrift`](api/idl/nova.thrift)

## 约束与安全性提示

- Frame Body 最大值：`1MB`（见 `protocol.MaxFrameBody`）
- gzip 解压有输出上限（防止解压炸弹）
- `FlagEncrypted`（加密位）当前会被拒绝，返回 `protocol.ErrUnsupportedFrameFlags`

## 贡献

欢迎以 PR / Issue 的方式提交改进：

- 新增命令：在 `protocol/commands.go` 定义 `CmdXXX`，并在 `setup` 中注册 handler
- 扩展 flags：优先在 `protocol` 包集中实现编码/解码规则，保持跨语言一致性

## CI/CD

### GitHub Actions Workflows

本项目配置了完整的 CI/CD 流水线：

#### 1. **CI 测试** ([.github/workflows/ci.yml](.github/workflows/ci.yml))

自动触发：每次 push 到 `main` 分支或 pull request

- ✅ 启动 Redis 服务容器（7-alpine）
- ✅ 运行根模块测试（race detector + coverage）
- ✅ 运行 ACL 模块测试（独立 go.mod）
- ✅ 命令映射一致性校验（`cmd/validate-commands`）
- ✅ 上传测试覆盖率到 Codecov（可选）

#### 2. **Pre-commit 检查** ([.github/workflows/pre-commit.yml](.github/workflows/pre-commit.yml))

自动触发：pull request 或 push

- ✅ `go fmt` 格式化检查（未格式化会失败）
- ✅ `go vet` 静态分析
- ✅ 命令映射一致性校验
- ⚠️ TODO/FIXME 警告（无 issue 引用时提示）
- ❌ 硬编码凭证检查（发现时失败）

#### 3. **Docker 镜像构建** ([.github/workflows/docker-build.yml](.github/workflows/docker-build.yml))

自动触发：push 到 `main`、打 tag 或 pull request

- 🐳 构建 `novagate-server` 镜像（[Dockerfile.server](Dockerfile.server)）
- 🐳 构建 `novagate-acl` 镜像（[services/acl/Dockerfile](services/acl/Dockerfile)）
- 📦 推送到 GitHub Container Registry (`ghcr.io`)
- 🏷️ 自动标记：`main`、PR 号、版本号、commit SHA

#### 4. **发布自动化** ([.github/workflows/release.yml](.github/workflows/release.yml))

自动触发：打 tag（`v*.*.*`）

- 📦 交叉编译多平台二进制（Linux/macOS，amd64/arm64）
- 🏷️ 生成 GitHub Release + Changelog
- ⬆️ 上传发布包（`.tar.gz`）

### 本地测试（推荐）

```bash
# 使用 Docker Redis 运行完整测试
./scripts/test.sh docker-up
./scripts/test.sh test

# 或手动启动 Redis
docker-compose up -d
mise exec -- go test ./...
cd services/acl && go test ./...
```

详见：[DOCKER_TESTING.md](DOCKER_TESTING.md)

### 状态徽章（可选）

在仓库中添加：

```markdown
[![CI](https://github.com/gogogo1024/novagate/actions/workflows/ci.yml/badge.svg)](https://github.com/gogogo1024/novagate/actions/workflows/ci.yml)
[![Docker](https://github.com/gogogo1024/novagate/actions/workflows/docker-build.yml/badge.svg)](https://github.com/gogogo1024/novagate/actions/workflows/docker-build.yml)
```

