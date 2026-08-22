# Gateway Adapter

本文记录硬件端如何接入云端 `gateway-adapter`，以及云端 Adapter 的内部运行方式。硬件协议仍以硬件端最终文档和真实 raw 包为准。

硬件端联调只需要阅读“硬件端联调指南”。`gateway-adapter-mock`、raw journal、replay、运行命令属于云端内部自测和排障工具，硬件端不需要理解或运行。

硬件报文、时隙、CRC、ACK/清缓存语义的当前摘要见 `docs/hardware_protocol_current.md`。

## 硬件端联调指南

本节可直接给硬件端作为第一轮联调说明。当前云端对硬件端暴露的是 **TCP 字节流入口**，不是 HTTP JSON 接口。

### 1. 双方分工

硬件端负责：

- 作为 TCP client 主动连接云端 `43.143.208.153:8081`。
- 在 TCP 连接上直接写入真实硬件原始 bytes。
- 首包或上线阶段发送可识别的设备网关名称，建议使用 `FP_` 开头，例如 `FP_NB114`。
- 按约定发送心跳包，保持设备网关在线状态。
- 发送真实工作包、告警包、ACK 包样例，并说明每个包的预期语义。
- 保持 TCP 连接，用于接收云端写回的下行 `command_hex`。

云端负责：

- 提供 TCP 接入地址并接收所有原始 bytes。
- 把收到的原始包展示在管理后台“原始报文排查”。
- 识别注册和心跳，刷新设备网关在线状态。
- 在协议未冻结前，不误解析工作包和告警包，只做 raw 留痕。
- 管理农户、设备网关绑定、联调点位和自定义 Hex 下行命令。
- 在 Admin/Mini 创建控制命令后，把 `command_hex` 写回硬件端当前 TCP 连接。

云端不会要求硬件端：

- 调用 HTTP API。
- 组装 JSON。
- 理解或运行 `gateway-adapter-mock`。
- 理解 Go 服务、数据库、raw journal 文件或 replay 工具。

### 2. 连接地址

联调环境：

```text
TCP Host: 43.143.208.153
TCP Port: 8081
```

硬件网关需要作为 TCP client 主动连接该地址。云端不会主动去连接硬件网关。

### 3. 硬件端最小动作

连接建立后，硬件端直接向 TCP socket 写入原始 bytes。云端不要求硬件端组 JSON，也不要求硬件端调用 HTTP。

第一轮建议按以下顺序测试：

1. 注册包：发送一个以 `FP_` 开头的 ASCII 名称，例如 `FP_NB114`。
2. 心跳包：发送 `keepalive` 或 `keepalive message`。
3. mock telemetry 文本联调：发送 `NH_TELEMETRY|...` 文本帧（见下方示例）。
4. 真实工作包：发送 MCU 透传出来的原始二进制包。
5. 真实告警包：发送 MCU 透传出来的原始告警包。
6. 等待下行：保持 TCP 连接不断开，云端在 Admin 创建下行命令后会把 `command_hex` 写回同一条 TCP 连接。

注意：

- `FP_` 前缀用于云端识别设备网关名称；不带 `FP_` 的公网扫描包会进入原始报文排查，但不会被当作正式设备网关注册。
- 心跳包会刷新设备网关在线时间；当前离线扫描默认 35 秒未上报后标记离线。
- MCU1 BB=01/02/03 工作包可做展示型协议解码；温度换算规则未确认前不承诺写入温度、告警或曲线业务数据。
- MCU1 上行前两字节按采集执行器 ID 展示，不再假定为固定 `AA AA` 帧头；BB、报文长度和 CRC 共同确定 MCU1 上行类型。
- mock telemetry 文本帧示例：

  ```text
  NH_TELEMETRY|frequency=1|device_external_id=1001|function_type=1|channel_no=1|value=26.5|unit=C
  ```

### 4. 平台侧查看位置

硬件端发包后，在管理后台查看：

```text
Admin: http://43.143.208.153:8080/admin/
页面：原始报文排查
```

应能看到：

- 来源 IP 和端口。
- 设备网关名称，注册成功后显示 `FP_xxx`。
- 事件类型：注册、心跳、工作数据、告警或未知包。
- 原始 Hex 和 payload 快照。
- 业务处理状态：注册/心跳可处理；MCU1 工作包可显示协议字段，但在设备映射和温度换算完成前仍不代表已写入业务数据。
- 平台原始报文排查默认取最近 24 小时的 `source_type=1` TCP 原始报文，支持切换查看结构化 Adapter 事件，并支持按网关、`message_id`、`payload_hex` 过滤定位。
- `/api/v1/adapter/device-events` 的结构化 JSON 事件没有 `payload_hex` 是正常契约，不代表设备发送了空心跳或空 MCU1 报文。

持续发送 PPT MCU1 样例（测试采集器 ID `03 E9`）可使用：

```bash
./gateway-adapter-mock \
  -addr 127.0.0.1:8081 \
  -gateway FP_TEST_ADMIN_002 \
  -collector-id-hex 03E9 \
  -frame register,heartbeat,mcu1-auto-history,mcu1-manual,mcu1-fault \
  -loop -heartbeat-interval 30s -work-interval 10s
```

这里的 `03 E9` 仅是显式测试 ID；生产设备的字节序及其与平台采集执行器编号的映射仍需甲方确认。

如果硬件端发了包但平台没有显示，优先排查：

1. TCP 是否连到 `43.143.208.153:8081`，不是 HTTP `8080`。
2. 防火墙/安全组是否允许硬件端出口访问 `8081`。
3. 硬件端是否真的写入了 bytes，而不是只建立连接。
4. 注册包是否以 `FP_` 开头；不以 `FP_` 开头的包不会显示为正式设备网关。

### 5. 从注册到 C 端展示的临时流程

真实工作包 parser 未冻结前，平台提供一个联调辅助流程，用来先跑通“设备展示、绑定、C 端查看、下行命令”：

1. 硬件端发送 `FP_xxx` 注册包。
2. Admin 进入“设备管理”，看到设备网关。
3. Admin 选中设备网关，点击“添加联调点位”。
4. 平台在该设备网关下创建一个采集执行器和默认 4 个功能点。
5. Admin 在“用户管理”或“绑定管理”把农户绑定到该设备网关。
6. 农户端进入“我的设备”，可看到该设备网关下的功能点。
7. Admin 或农户端对可控功能点发送自定义 Hex 指令。
8. Adapter 把该 `command_hex` 写回硬件端当前 TCP 连接。

这个流程只用于联调平台能力，不代表真实硬件工作包字段已经冻结。

### 6. 下行命令联调

当前下行按异步命令模型执行：

```text
Admin/Mini 创建控制命令
  -> Server 写 control_commands(pending)
  -> gateway-adapter 按 gateway_external_id 拉取 pending command
  -> gateway-adapter 将 command_hex 写回当前 TCP 连接
  -> Server 标记 sent
```

联调阶段推荐由 Admin 手动输入自定义 `command_hex`，不要提前把未确认协议写死成业务按钮。硬件端收到后，请记录实际收到的 hex，并返回真实 ACK 样例给云端确认。

当前已经可验证：

- 云端能创建 pending 命令。
- Adapter 能拉取 pending 命令。
- Adapter 能把 `command_hex` 写回当前 TCP 连接。
- 控制日志能看到 pending/sent 状态。

当前不能承诺：

- MCU 是否执行成功。
- ACK 是否能解析成 succeeded/failed。
- 清缓存命令是否正确。
- 3 字节 `HH MM SS` 的最终编码和时区（最新 MCU1 文档已取代旧“6 字节时间”口径）。

### 7. 硬件端需要回传给云端的联调材料

每次联调失败或字段不匹配时，请硬件端提供：

- 发送时间。
- 设备网关名称，例如 `FP_NB114`。
- 原始上行 hex。
- 串口调试助手截图或日志。
- 如果收到云端下行，提供收到的下行 hex。
- 如果 MCU 有 ACK，提供 ACK 原始 hex。
- 说明该包预期语义：注册、心跳、工作数据、告警、读取结果、ACK 或清缓存确认。

云端会以管理后台原始报文排查和 Adapter 收包日志为准做对照。

## 职责

- 接收硬件网关 TCP 连接，默认监听 `NEHUANG_GATEWAY_TCP_ADDR=:8081`。
- 收到任意 bytes 后立即写 raw frame journal。
- 联调默认不透传 unknown raw 包到 Server，仅保存本地 journal；如需排障可通过 `NEHUANG_GATEWAY_FORWARD_UNKNOWN_FRAMES=true` 开关透传 unknown。
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

## 云端内部运行命令

本节只给云端开发和运维使用，硬件端不需要阅读。

```bash
go build ./cmd/gateway-adapter
go build ./cmd/gateway-adapter-mock
go build ./cmd/gateway-adapter-replay
```

本地启动示例：

```bash
NEHUANG_GATEWAY_TCP_ADDR=:8081 \
NEHUANG_GATEWAY_HEALTH_ADDR=:18081 \
NEHUANG_GATEWAY_FORWARD_UNKNOWN_FRAMES=false \
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
| `NEHUANG_GATEWAY_FORWARD_UNKNOWN_FRAMES` | `false` | unknown 包是否透传到 `/api/v1/adapter/raw-frames`；默认 `false` 时仅保留本地 journal |

回放 raw journal：

```bash
go run ./cmd/gateway-adapter-replay -raw-log data/gateway-adapter/raw-frames/raw-frames-YYYYMMDD.jsonl
```

## 云端内部实现口径

当前第一版 Adapter 对外暴露的是 **TCP 字节流入口**，不是 HTTP JSON 接口。

```text
硬件网关 TCP client -> gateway-adapter TCP listener -> raw journal -> /api/v1/adapter/raw-frames -> Go 服务解析
```

硬件端需要做的事情：

1. 硬件网关作为 TCP client 连接云端 `IP:PORT`，例如 `43.143.208.153:8081`。
2. 连接建立后，直接向 socket 写入原始 bytes。可以是 ASCII 注册包，也可以是 MCU 透传出来的二进制包。
3. 第一版不要求硬件网关先组 JSON，也不要求硬件网关调用 HTTP。
4. 每次写入的 bytes 都会被 Adapter 立即写入 raw frame journal，用于后续确认真实协议和解析规则。
5. 除已识别注册/心跳外，unknown 包默认仅保留本地 journal；如需排障可通过 `NEHUANG_GATEWAY_FORWARD_UNKNOWN_FRAMES=true` 将 unknown 透传到 Go 服务。MCU1 BB=01/02/03 可生成展示型协议摘要；业务温度和告警投影仍待正式规则确认。

业务解析边界：

- TCP 接收循环只负责连接、读写、raw journal、raw 上传策略（unknown 默认仅本地保留 + 可选透传）和最小分类。
- 工作包、告警包、ACK 的业务字段解析必须基于真实 raw 样例和 replay 回归。
- 不把 MCU 温控算法、时隙策略、告警去重规则写死在 TCP 接收循环中。

### 当前可识别报文

| 报文 | 示例 | Adapter 行为 |
|------|------|--------------|
| 注册 | ASCII `FP_TEST_ADMIN_001\n` | Adapter 上报 raw；Go 服务解析为 `register` 并刷新 gateway |
| 心跳 | ASCII `keepalive`、`keepalive message`、`HEARTBEAT\n`、`HB`、`PING`、`HEART` | Adapter 上报 raw；Go 服务解析为 `heartbeat` 并刷新 gateway |
| Mock 工作数据 | ASCII `NH_TELEMETRY|frequency=1|device_external_id=1001|function_type=1|channel_no=1|value=26.5|unit=C` | Adapter 上报 raw；Go 服务解析并投影采集执行器、功能点、实时值和时序数据 |
| MCU1 工作包 | 二进制 `AA AA BB HH MM SS ... Tag CRC_LO CRC_HI` | Adapter 上报 raw；Go 服务展示 ID、BB、时间原字节、Tag、CRC 和温度记录边界，不代表已写入温度业务 |
| 告警包 | 二进制 `AA AA ... FF FF ... CRC_LO CRC_HI` | Adapter 上报 raw；Go 服务保持 pending，去重和告警入库等真实样例确认后补 |
| 未知包 | 任意无法识别 bytes | 默认仅写 local journal；`NEHUANG_GATEWAY_FORWARD_UNKNOWN_FRAMES=true` 时上报 raw；不中断连接 |

### 云端内部调试工具：gateway-adapter-mock

`gateway-adapter-mock` 是云端开发自测工具，用来在没有真实硬件时模拟 TCP client。硬件端联调时不需要使用该工具，直接用真实硬件网关连接 TCP 地址即可。

`gateway-adapter-mock` 的 `-frame` 支持逗号分隔组合（例如：`register`、`heartbeat`、`heartbeat-message`、`modbus-read`、`mcu1`、`work`、`alarm`、`unknown`）。

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

只发送调试变体心跳：

```bash
go run ./cmd/gateway-adapter-mock -addr 127.0.0.1:8081 -frame heartbeat-message
```

只发送 Modbus 读：

```bash
go run ./cmd/gateway-adapter-mock -addr 127.0.0.1:8081 -frame modbus-read
```

只发送 CRC 合法的占位工作包：

```bash
go run ./cmd/gateway-adapter-mock -addr 127.0.0.1:8081 -frame work
```

按 MCU1 说明书第 14–15 页发送三类完整上行样例（`BB=01/02/03`，均带合法 Tag 和 CRC16）：

```bash
go run ./cmd/gateway-adapter-mock \
  -addr 43.143.208.153:8081 \
  -gateway FP_TEST_ADMIN_002 \
  -frame register,mcu1 \
  -delay 500ms
```

其中时间字段使用原始测试字节 `01 02 03`，只验证字段定位，不代表时间编码已经冻结。也可以分别使用 `mcu1-auto`、`mcu1-manual`、`mcu1-fault`。

只发送告警包：

```bash
go run ./cmd/gateway-adapter-mock -addr 127.0.0.1:8081 -frame alarm
```

只发送未知包：

```bash
go run ./cmd/gateway-adapter-mock -addr 127.0.0.1:8081 -frame unknown
```

重复发送 3 轮：

```bash
go run ./cmd/gateway-adapter-mock -addr 127.0.0.1:8081 -frame all -repeat 3 -delay 1s
```

持续验证 TCP 收包（只验证 raw/协议透视，不生成可换算温度曲线）：

```bash
go run ./cmd/gateway-adapter-mock -addr 127.0.0.1:8081 -loop -duration 30m -heartbeat-interval 10s
```

持续生成有波动的结构化测试曲线（`--rounds 0` 直到 Ctrl+C）：

```bash
scripts/mock_device_events.sh --rounds 0 --interval 60
```

两者用途不同：前者验证真实 TCP/raw 链路，后者在 MCU1 温度编码尚未冻结时验证业务入库和曲线展示。

模拟硬件等待云端下行命令：

```bash
go run ./cmd/gateway-adapter-mock -addr 127.0.0.1:8081 -wait-downlink -wait-timeout 30s
```

该模式会先发送注册和心跳，使 Adapter 能识别设备网关；随后阻塞读取 TCP 下行 bytes，并把收到的命令打印为 hex。

已可执行的 mock 循环联调口径：

- 注册一次后可进入 keepalive 轮询：`-frame heartbeat -repeat N -delay 10s`。
- 可选触发 `work`、`alarm`、`unknown`、`modbus-read` 包。
- 支持 `-wait-downlink` 在超时前持续等待并输出下行 command hex。
- `-loop` 为下轮规划项，当前采用 `-repeat/-delay` 实现持续性联调。

```bash
# 持续 keepalive + 等待下行（可用于联调）
go run ./cmd/gateway-adapter-mock \
  -addr 43.143.208.153:8081 \
  -gateway FP_TEST_ADMIN_002 \
  -frame heartbeat \
  -repeat 200 \
  -delay 10s \
  -wait-downlink \
  -wait-timeout 120s
```

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
