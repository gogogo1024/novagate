# Protocol Specification (稳定版)

## 1. 设计目标

本协议主要用于 客户端 ↔ Gate 或 边缘节点 ↔ Gate 的长连接通信，
不直接暴露给后端业务服务，目标是：

- 明确定义 Frame / Message 边界
- 支持多路复用与扩展
- 可对接 Kitex / 自定义 RPC
- 具备连接级资源控制能力

本协议 **不依赖 HTTP / gRPC**，适用于高性能网关、游戏服、内网 RPC。

---

## 2. 分层模型

```
+-----------------------------+
|        Application          |
|   (Command / RPC Handler)   |
+-----------------------------+
|          Message            |
|   (Command + Payload)       |
+-----------------------------+
|           Frame             |
|   (Length + Meta + Body)    |
+-----------------------------+
|            TCP              |
+-----------------------------+
```

---

## 3. Frame 定义（网络最小传输单元）

### 3.1 Frame Header

| 字段名 | 长度(Byte) | 说明 |
|------|-----------|------|
| Magic | 2 | 固定值 `0xCAFE` |
| Version | 1 | 协议版本 |
| Flags | 1 | 位标志 |
| Length | 4 | Body 总长度（不含 Header） |

Header 总长度：**8 字节**

---

### 3.2 Frame Body

```
+------------------+
|   MessageBytes   |
+------------------+
```

Body 为完整 Message 的二进制表示。

---

## 4. Message 定义（语义单元）

### 4.1 Message 结构

| 字段名 | 类型 | 说明 |
|------|------|------|
| Command | uint16 | 语义指令 |
| RequestID | uint64 | 请求唯一标识 |
| Payload | bytes | 编码后的业务数据 |

---

### 4.2 Message 二进制布局

```
+---------+------------+-----------+
| Command | RequestID  | Payload   |
|  2B     |   8B       |  N bytes  |
+---------+------------+-----------+
```

---

## 5. Command 设计

Command 是 **协议级路由键**，用于在 Gate 内部做快速分发。

### 5.1 示例

| Command | 含义 |
|-------|------|
| 0x0001 | Ping |
| 0x0101 | UserLogin |
| 0x0201 | OrderCreate |

---

## 6. 编码流程（发送）

```
Payload (业务结构体)
    ↓ Codec (JSON / Protobuf)
Message (Command + ReqID + Payload)
    ↓
Frame (Header + Message)
    ↓
TCP Write
```

---

## 7. 解码流程（接收）

```
TCP Read
  ↓
Frame Decode（按 Length 拆包）
  ↓
Message Decode
  ↓
Command Router
  ↓
Handler / RPC
```

---

## 8. 粘包 / 拆包规则

- 使用 Length-Field Based Frame
- Length 表示 Body 总长度
- 不允许半包 Message

---

## 9. 扩展能力

### 9.1 Flags 位定义（预留）

| Bit | 含义 |
|----|------|
| 0 | 是否压缩 |
| 1 | 是否加密 |
| 2 | 是否单向消息 |

---

## 10. 与 Kitex 的关系

- Gate 负责：协议、连接、限流、路由
- Kitex 负责：RPC、IDL、服务治理

```
Client → Gate → Kitex RPC → Service
```

Gate **不等价于 Kitex**，而是位于 Kitex 之前。

---

## 11. 非目标（明确不做的事情）

- 不定义业务字段
- 不绑定具体序列化协议
- 不处理服务治理逻辑

---

## 12. 总结

> Frame 解决 **怎么收**
> Message 解决 **是什么**
> Command 解决 **往哪走**
> Gate 解决 **谁能进**
