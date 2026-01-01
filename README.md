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
- `cmd/server/`：示例服务端启动入口（注册 command + handler）
- `cmd/client/`：示例客户端（手工组包/发包/收包）
- `internal/`：Go 侧默认实现的内部组件（dispatcher/codec/limits/transport 等）
- `docs/`：协议与架构决策文档

## 快速开始

### 使用 mise 管理 Go 版本

本项目使用 `mise` 管理 Go 版本（与 `go.mod` 的 `go 1.25.5` 对齐）。

```bash
mise install
```

后续命令建议通过 `mise exec -- ...` 执行，确保使用一致的 Go 版本。

### 其他常用命令也建议用 mise

凡是依赖 Go 工具链的命令，都可以统一用 `mise exec -- go ...` 来跑，例如：

```bash
# 格式化
mise exec -- gofmt -w .

# 只跑 protocol 包测试
mise exec -- go test ./protocol -run TestFrameEncodeDecodeRoundTrip -count=1

# 基础静态检查
mise exec -- go vet ./...

# 依赖整理
mise exec -- go mod tidy
```

### 运行测试

```bash
mise exec -- go test ./...
```

### 启动服务端

```bash
mise exec -- go run ./cmd/server
```

默认会尝试读取当前目录下的 `novagate.yaml`（如果文件不存在会忽略）；支持的 YAML 结构（kitex 风格分组）：

你可以直接复制示例配置文件：[`novagate.yaml.example`](novagate.yaml.example) → `novagate.yaml`。

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

### One-way（不等响应）

```bash
mise exec -- go run ./cmd/client -addr 127.0.0.1:9000 -cmd 0x0001 -payload ping -flags 0x04
```

### 启用 gzip 压缩

```bash
mise exec -- go run ./cmd/client -addr 127.0.0.1:9000 -cmd 0x0001 -payload ping -flags 0x01
```

> 注：压缩/解压由 `protocol.EncodeFrameBody` / `protocol.DecodeFrameBody` 统一处理。

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
