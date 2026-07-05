# Gateway Adapter

本文记录当前 Go `gateway-adapter` 的工程边界。硬件协议仍以甲方最终文档和真实 raw 包为准。

硬件报文、时隙、CRC、ACK/清缓存语义的当前摘要见 `docs/hardware_protocol_current.md`。本文只写 Adapter 如何运行、如何收包、如何留痕、如何联调。

## 职责

- 接收硬件网关 TCP 连接，默认监听 `NEHUANG_GATEWAY_TCP_ADDR=:8081`。
- 收到任意 bytes 后立即写 raw frame journal。
- 将每次 TCP read 到的原始 bytes 上报到 `POST /api/v1/adapter/raw-frames`。
- 保留旧网关 `FP_xxx` 注册包和显式心跳包的最小分类，用于连接维度 `gateway_name` 和 debug 日志；业务解析由 Go 服务完成。
- 识别出设备网关后，按 `gateway_external_id` 拉取 Server 中的 pending control command。
- 将 pending command 的 `command_hex` 写回当前 TCP 连接，写成功后回调 Server 标记 `sent`。
- 提供 `/healthz` 和 `/readyz`，默认监听 `NEHUANG_GATEWAY_HEALTH_ADDR=:18081`。
- 提供 CRC-16/MODBUS 和协议分类骨架，真实工作包解析等硬件样例包稳定后再补。

## 日志分工

- runtime log：结构化 JSON，记录启动、连接、断开、转发失败、raw 写入失败。
- raw frame journal：JSONL，默认目录 `data/gateway-adapter/raw-frames`，字段包含 `connection_id`、`gateway_name`、`remote_addr`、`direction`、`payload_hex`、`received_at`、`raw_seq`。
- debug/protocol log：通过 `NEHUANG_GATEWAY_DEBUG=true` 开启，记录协议分类和 CRC 等调试字段。
- command log：结构化 JSON，记录 pending 拉取、TCP 写回、`sent` 回调和失败原因。

## 命令

```bash
go build ./cmd/gateway-adapter
go build ./cmd/gateway-adapter-mock
go build ./cmd/gateway-adapter-replay
```

本地启动示例：

```bash
NEHUANG_GATEWAY_TCP_ADDR=:8081 \
NEHUANG_GATEWAY_HEALTH_ADDR=:18081 \
NEHUANG_GATEWAY_API_BASE_URL=http://43.143.208.153:8080 \
NEHUANG_ADAPTER_TOKEN=nehuang-adapter-dev-token \
go run ./cmd/gateway-adapter
```

下行命令相关配置：

| 环境变量 | 默认值 | 含义 |
|----------|--------|------|
| `NEHUANG_GATEWAY_COMMAND_POLL_INTERVAL_SECONDS` | `2` | 每个已识别设备网关连接拉取 pending command 的间隔 |
| `NEHUANG_GATEWAY_COMMAND_POLL_LIMIT` | `20` | 每次拉取命令数量上限 |
| `NEHUANG_GATEWAY_COMMAND_WRITE_TIMEOUT_SECONDS` | `5` | 写回 TCP 连接的超时时间 |

模拟硬件网关发包：

```bash
go run ./cmd/gateway-adapter-mock -addr 127.0.0.1:8081 -frame all
```

回放 raw journal：

```bash
go run ./cmd/gateway-adapter-replay -raw-log data/gateway-adapter/raw-frames/raw-frames-YYYYMMDD.jsonl
```

## 硬件侧交互口径

当前第一版 Adapter 对甲方暴露的是 **TCP 字节流入口**，不是 HTTP JSON 接口。

```text
硬件网关 TCP client -> gateway-adapter TCP listener -> raw journal -> /api/v1/adapter/raw-frames -> Go 服务解析
```

甲方需要做的事情：

1. 硬件网关作为 TCP client 连接云端 `IP:PORT`，例如 `43.143.208.153:8081`。
2. 连接建立后，直接向 socket 写入原始 bytes。可以是 ASCII 注册包，也可以是 MCU 透传出来的二进制包。
3. 第一版不要求硬件网关先组 JSON，也不要求硬件网关调用 HTTP。
4. 每次写入的 bytes 都会被 Adapter 立即写入 raw frame journal，用于后续确认真实协议和解析规则。
5. 当前所有 bytes 都会被 Adapter 上报到 Go 服务的 `/api/v1/adapter/raw-frames`；Go 服务第一版只解析注册和心跳，工作包/告警包保持 pending。

业务解析边界：

- TCP 接收循环只负责连接、读写、raw journal、全量 raw 上报和最小分类。
- 工作包、告警包、ACK 的业务字段解析必须基于真实 raw 样例和 replay 回归。
- 不把 MCU 温控算法、时隙策略、告警去重规则写死在 TCP 接收循环中。

### 当前可识别报文

| 报文 | 示例 | Adapter 行为 |
|------|------|--------------|
| 注册 | ASCII `FP_TEST_ADMIN_001\n` | Adapter 上报 raw；Go 服务解析为 `register` 并刷新 gateway |
| 心跳 | ASCII `HEARTBEAT\n`、`HB`、`PING`、`HEART` | Adapter 上报 raw；Go 服务第一版解析 `HEARTBEAT` 并刷新 gateway |
| 工作包 | 二进制 `AA AA ... CRC_LO CRC_HI` | Adapter 上报 raw；Go 服务保持 pending，字段解析等真实样例确认后补 |
| 告警包 | 二进制 `AA AA ... FF FF ... CRC_LO CRC_HI` | Adapter 上报 raw；Go 服务保持 pending，去重和告警入库等真实样例确认后补 |
| 未知包 | 任意无法识别 bytes | Adapter 上报 raw，不中断连接 |

### 本地模拟报文

`gateway-adapter-mock` 是硬件 TCP client 的本地替身，用于演示“甲方怎么发、我们怎么收”。

发送全部样例：

```bash
go run ./cmd/gateway-adapter-mock -addr 127.0.0.1:8081 -frame all
```

只发送注册：

```bash
go run ./cmd/gateway-adapter-mock -addr 127.0.0.1:8081 -frame register
```

只发送心跳：

```bash
go run ./cmd/gateway-adapter-mock -addr 127.0.0.1:8081 -frame heartbeat
```

只发送 CRC 合法的占位工作包：

```bash
go run ./cmd/gateway-adapter-mock -addr 127.0.0.1:8081 -frame work
```

只发送未知包：

```bash
go run ./cmd/gateway-adapter-mock -addr 127.0.0.1:8081 -frame unknown
```

重复发送 3 轮：

```bash
go run ./cmd/gateway-adapter-mock -addr 127.0.0.1:8081 -frame all -repeat 3 -delay 1s
```

模拟硬件等待云端下行命令：

```bash
go run ./cmd/gateway-adapter-mock -addr 127.0.0.1:8081 -wait-downlink -wait-timeout 30s
```

该模式会先发送注册和心跳，使 Adapter 能识别设备网关；随后阻塞读取 TCP 下行 bytes，并把收到的命令打印为 hex。

## 如何看日志

### runtime log

Adapter 标准输出是结构化 JSON。重点看这些事件：

| event | 含义 |
|-------|------|
| `gateway_adapter_started` | Adapter 启动完成 |
| `gateway_connection_opened` | 硬件网关建立 TCP 连接 |
| `gateway_protocol_classified` | debug 开启时输出协议识别结果 |
| `gateway_raw_frame_forward_failed` | raw frame 上报 Go 服务失败 |
| `gateway_pending_commands_poll_failed` | 拉取 pending command 失败 |
| `gateway_pending_command_decode_failed` | pending command 的 `command_hex` 非法 |
| `gateway_pending_command_write_failed` | 写回 TCP 连接失败 |
| `gateway_pending_command_mark_sent_failed` | TCP 写回成功后，回调 Server 标记 sent 失败 |
| `gateway_pending_command_written` | 命令已写回 TCP |
| `gateway_connection_closed` | TCP 连接断开 |

开启协议调试：

```bash
NEHUANG_GATEWAY_DEBUG=true go run ./cmd/gateway-adapter
```

### raw frame journal

默认文件：

```text
data/gateway-adapter/raw-frames/raw-frames-YYYYMMDD.jsonl
```

每一行是一条原始收包记录，示例：

```json
{"connection_id":"...","gateway_name":"FP_TEST_ADMIN_001","remote_addr":"127.0.0.1:54321","direction":"inbound","payload_hex":"46505F544553545F41444D494E5F3030310A","received_at":"2026-07-05T10:00:00Z","raw_seq":1}
```

字段说明：

| 字段 | 含义 |
|------|------|
| `connection_id` | Adapter 为每个 TCP 连接生成的 ID |
| `gateway_name` | 已识别出来的网关名；注册前可能为空 |
| `remote_addr` | TCP client 地址 |
| `direction` | 当前第一版主要是 `inbound` |
| `payload_hex` | 收到的原始 bytes 的十六进制表示 |
| `received_at` | Adapter 收到 bytes 的时间 |
| `raw_seq` | 当前连接内的收包序号 |

回放统计：

```bash
go run ./cmd/gateway-adapter-replay -raw-log data/gateway-adapter/raw-frames/raw-frames-YYYYMMDD.jsonl
```

## 联调顺序

1. 发送 `FP_TEST_ADMIN_001`，确认 raw journal 有记录，Go 服务 `raw_device_events` 有 raw frame，gateway 被刷新。
2. 发送心跳包，确认 gateway 在线时间刷新。
3. 发送真实工作包，检查 raw journal 和 Go 服务 `raw_device_events`，不要求第一版立刻解析。
4. 拿 raw journal 跑 replay，补协议 parser 和 CRC 校验。
5. 在 Admin 或 Mini 创建控制命令，确认 Adapter 能拉取 pending command 并写回当前 TCP 连接。
6. 等 MCU ACK 协议确认后，再启用真实 ACK 解析、清缓存和完整闭环。

## Nginx

第一版默认硬件 TCP 直连 Go Adapter。Nginx 继续只代理 HTTP 管理端和 API。

如果后续需要统一 TCP 入口，可用 Nginx stream 代理，但这会增加一层排障面，联调稳定前不作为默认方案。
