# AI Coding Agent 指南（Novagate）

## 项目大图
- Novagate 是基于 TCP 长连接的轻量 RPC 网关骨架：TCP 字节流 → Frame → Message → Command 路由（见 docs/protocol.md、docs/decision.md）。
- `protocol/` 是跨语言可复用的纯协议层（Frame/Message/Flags/Command 映射）；Go 运行时在根包 `novagate`（accept loop + conn handler + Router）。
- 与 Kitex 的对接点在 `internal/codec/MessageCodec`：把 Kitex RPC 的 `Service.Method` 映射为协议 `Command(uint16)`（见 protocol/mapper.go）。

## 关键目录/入口
- `cmd/server/`：示例网关服务端入口；在 `setup()` 里注册 command 映射 + 路由（见 cmd/server/main.go）。
- `cmd/client/`：最小客户端，手工组包/解包用于联调（见 cmd/client/main.go）。
- `protocol/`：Frame(`protocol/frame.go`) + Message(`protocol/message.go`) + Flags/gzip(`protocol/compress.go`) + Cmd 常量(`protocol/commands.go`)。
- `services/acl/`：独立 Go module 的 HTTP ACL 子服务（Hertz），按配置选择 InMemory/Redis store（见 services/acl/main.go）。

## 开发/测试命令（macOS）
- Go 版本：本仓库用 `mise` 对齐 `go 1.25.5`（见 README.md、go.mod）：`mise install`。
- 根模块测试：`mise exec -- go test ./...`（注意：会跳过含独立 go.mod 的子模块，如 services/acl）。
- 注意：`cmd/server/main.go` 对 `.test` 二进制会直接 `return`，避免某些环境下 `go test ./...` 误启动监听。
- 启动网关示例：`mise exec -- go run ./cmd/server -config ./novagate.yaml`。
- 客户端验证：`mise exec -- go run ./cmd/client -addr 127.0.0.1:9000 -cmd 0x0001 -payload ping`。
- ACL 子模块：`cd services/acl && go test ./... && go run . -config ./config.example.yaml`。

## 项目内约定（写代码时优先遵循）
- Command 映射：生产建议开启 strict（见 protocol/mapper.go、cmd/server/main.go）。新增命令时：
  - 在 `protocol/commands.go` 增加 `CmdXXX`；
  - 在 `setup()` 调 `protocol.RegisterFullMethodCommand("Service.Method", CmdXXX)` 并 `protocol.SetStrictCommandMapping(true)`；
  - 通过 `Router.Register(cmd, novagate.BridgeProtocolHandler(...))` 绑定处理（桥接示例见 cmd/server/main.go；业务示例 handler 注册见 internal/service/registry.go）。

### 新增命令（3 步最小示例）
1) 在 `protocol/commands.go` 添加常量（示例：`const CmdFoo uint16 = 0x0301`），把 `Command` 当成稳定 ABI 管理。
2) 在 `cmd/server/main.go` 的 `setup()` 里注册映射并桥接：`protocol.RegisterFullMethodCommand("FooService.Bar", protocol.CmdFoo)` + `r.Register(protocol.CmdFoo, novagate.BridgeProtocolHandler(...))`。
3) 在 `internal/service/registry.go`（或你的业务模块）里 `dispatcher.Register(protocol.CmdFoo, ...)`，供网关侧转发后落到业务实现。

### 命令一致性校验（推荐在改动后跑）
- `mise exec -- go run ./cmd/validate-commands`（只校验约定的 3 个文件：`protocol/commands.go`、`cmd/server/main.go`、`internal/service/registry.go`）
- 可选更严格：`mise exec -- go run ./cmd/validate-commands -require-all`（要求每个定义的 `Cmd*` 都被桥接并有 dispatcher handler）
- Flags 语义：`FlagEncrypted` 当前会被拒绝；`FlagOneWay` 不回写响应；响应会继承请求的 `RequestID`，并仅透传压缩位（见 protocol/compress.go、conn_handler.go）。
- 连接资源控制：每连接有 buffer quota（默认 256KiB）+ token bucket 限速（见 conn_ctx.go），`handleConn` 通过 Read/Write deadline 实现 idle/write timeout（见 conn_handler.go）。
- 配置优先级：`flag > env > yaml > default`；默认读取 `novagate.yaml`（不存在也允许）并加载本地 `.env`（见 cmd/server/config.go）。
- Kitex 编解码：`internal/codec/MessageCodec` 读取 `msg.Tags()["novagate.flags"]` 写入 Frame flags，并在 Decode 时回填 tags：`novagate.command/request_id/flags`（便于上层观测/路由）。
