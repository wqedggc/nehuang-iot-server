# 涅凰智农 IoT 云平台 V1.0 架构说明

> 更新日期：2026-07-05  
> 注意：本文记录当前架构边界和模块划分。最终物理表名、字段类型、索引和迁移顺序以 `schema/mysql/*.sql` 与 `docs/table_structure_v1.md` 为准。当前 API 以 `docs/current_api.md` 为准。

## 1. 当前架构结论

V1 当前采用“双进程、四层边界”：

```text
采集执行器 MCU
  -> 云边/LoRa/TCP 硬件网关
  -> Go gateway-adapter
  -> Go nehuang-server + MySQL + Admin/Mini
```

两个 Go 进程：

| 进程 | 职责 |
|------|------|
| `cmd/gateway-adapter` | TCP 接收硬件网关连接，先写 raw frame journal，再做最小报文分类和事件转发 |
| `cmd/server` | HTTP API、Admin/Mini 页面、MySQL 业务读写、绑定、点位配置、控制命令状态 |

核心原则：

- Adapter 尽量薄，只做连接、留痕、最小分类和转发。
- 业务事实以 MySQL 为准，不让页面或小程序直接读 raw 文件、TCP 连接或 `devices.json`。
- 真实硬件协议未稳定前，工作包/告警包只保留 raw，不误解析。
- 控制链路默认异步：用户操作先创建 `control_commands`，不长时间阻塞等待 MCU。

## 2. 与旧 Python 的关系

旧代码中较可信的历史链路：

```text
003rukou.py
  -> tcp_gateway.py
  -> 01app_core/data_cache_huancun/devices.json
  -> 01app_core/routes/*
```

旧 `tcp_gateway.py` 的价值：

- 证明硬件 TCP 连接、`FP` 注册、在线状态缓存曾经跑通过。
- 可作为联调对照和历史行为参考。

当前架构不再把旧 Python 作为正式实现继续扩展：

- 不继续让 `devices.json` 作为业务事实来源。
- 不继续把控制状态、协议解析和页面逻辑混在 Python 网关里。
- 新的硬件入口是 Go `gateway-adapter`。

## 3. 数据主模型

当前主模型：

```text
cloud_gateways
  -> collector_nodes
  -> device_points
```

含义：

- `cloud_gateways`：云边/LoRa/TCP 通信接入设备，注册包和心跳进入这里。
- `collector_nodes`：采集执行器节点，唯一身份为 `gateway_id + frequency + device_external_id`。
- `device_points`：节点下的功能点/通道，Admin/Mini 展示、绑定、曲线、控制都围绕 `point_id`。
- `device_realtime_maps`：保留甲方旧字段语义的兼容投影。

设备身份原则：

- 服务端完整硬件身份为 `gateway_id + frequency + device_external_id`。
- 功能点身份为 `node_id + function_type + channel_no`。
- 农户、棚号、设备名称、备注、绑定关系、阈值和控制策略都是服务端业务数据，不参与硬件唯一身份。

## 4. 上行数据流

### 4.1 注册和心跳

```text
硬件网关 TCP client
  -> gateway-adapter TCP listener
  -> raw frame journal
  -> register/heartbeat 分类
  -> POST /api/v1/adapter/device-events
  -> cloud_gateways
```

当前实现：

- `FP_xxx` ASCII 注册包已识别。
- `HEARTBEAT/HB/PING/HEART` 已识别。
- Go 服务可按 register/heartbeat 更新网关在线状态。
- 离线状态由 server offline scanner 兜底。

### 4.2 工作数据

甲方最新硬件文档给出的方向：

```text
MCU 每 5 分钟缓存温度
  -> 4 小时累计 48 组
  -> 工作包上报温度序列 + R/S/T/U/V/W + 长度 + CRC
  -> 云端确认后 MCU 清缓存并重新计时
```

当前架构处理方式：

```text
工作包 bytes
  -> gateway-adapter raw journal
  -> replay/parser
  -> raw_device_events
  -> sensor_data
  -> device_points / device_realtime_maps
```

当前实现状态：

- raw journal 已实现。
- CRC-16/MODBUS 已实现。
- 真实工作包字段 parser 未实现，等待真实样例和最终字段定义。

### 4.3 告警数据

甲方最新硬件文档给出的方向：

```text
R/S 故障
  -> MCU 在后续 3 个自有下摆时隙重复上报告警包
  -> 云端去重
  -> 告警记录
  -> Admin/Mini 通知
```

当前实现状态：

- Adapter 可保留 alarm-looking raw。
- 告警字段解析、重复去重和通知未实现。

## 5. 下行控制流

当前架构选择异步控制：

```text
Admin/Mini
  -> POST control API
  -> control_commands(created/pending)
  -> Adapter 拉取 pending command
  -> Adapter 在可用策略下发到硬件
  -> MCU ACK / result
  -> Adapter 回写 command result
  -> control_results
  -> control_commands(ack/succeeded/failed/timeout)
```

已经实现：

- Mini 控制接口创建 pending command。
- Adapter 内部接口可拉取 pending、标记 sent、回写 result。

未实现：

- Adapter 真实下行 hex 写回 TCP 连接。
- 时隙调度。
- MCU ACK 解析。
- 超时扫描完整接入前端状态。

当前下行协议理解：

| 指令 | 含义 |
|------|------|
| `AA AA BB XX YY` | 阈值同步/读取/握手起点，具体复用语义待确认 |
| `AA AA BB F0` | 手动开启 |
| `AA AA BB 0F` | 手动关闭 |
| `AA AA BB 00` | 手动停止 |
| `AA AA BB ZZ` | ACK/执行完成/清缓存确认之一，具体语义待确认 |

## 6. 时隙和网络延迟边界

甲方当前硬件方案：

- 单个云边集群最多按 100 台采集执行器规划。
- 每台采集执行器 2 秒专有时序。
- 满配时完整周期 200 秒。
- 上摆周期用于云端下发；下摆周期用于设备上报。
- 前后各 100ms 作为缓冲保护。

云端工程判断：

- 公网服务端不适合强依赖毫秒级窗口直接卡点下发。
- 更稳妥的设计是：server 创建 pending command，Adapter/硬件网关/MCU 侧在可执行窗口取走或下发。
- 如果甲方坚持云端主动卡窗口，需要提供明确的时钟同步、slot 分配、网络延迟容错和重试规则。

因此当前代码先实现异步命令状态机，不承诺实时同步控制。

## 7. API 边界

当前真实 API 见：

- `docs/current_api.md`

关键边界：

- 硬件网关不直接调 HTTP JSON，第一版是 TCP client 连接 `gateway-adapter`。
- `gateway-adapter` 转发 register/heartbeat 到 `POST /api/v1/adapter/device-events`。
- Admin/Mini 只调用 `cmd/server` 暴露的 HTTP API。
- Adapter 内部命令接口使用 `X-Adapter-Token`。

## 8. 页面数据投影

### 8.1 Admin

Admin 当前围绕以下业务对象：

- 用户/农户。
- 网关。
- 采集执行器节点。
- 功能点。
- 绑定关系。
- 原始事件。
- Payload 排查。
- 控制日志。

### 8.2 Mini / 农户端

Mini 当前围绕以下业务对象：

- 当前农户资料。
- 已绑定功能点。
- 实时数据。
- 历史曲线。
- 异步控制命令。

农户端数据权限必须通过 `bind_relations` 约束，不能直接按 gateway 或 device_external_id 查询全量设备。

## 9. 发布和运行边界

当前部署分两类进程：

```text
HTTP/API:
  nehuang-server :8080

Hardware TCP:
  gateway-adapter :8081
  gateway-adapter health :18081
```

发布原则：

- `nehuang-server` 升级不应影响 Adapter 接收 raw bytes。
- `gateway-adapter` 升级前应确保 raw journal、health、ready 检查可用。
- 真实硬件切流前保留旧 Python 网关或旧端口作为回退方案。
- 单机部署可以做到低停机和可回滚，但不承诺大型集群意义上的绝对零中断。

## 10. 不纳入当前已实现范围

- MCU 固件开发。
- 硬件端口接线和本地温控算法实现。
- 真实工作包 parser。
- 真实告警 parser 和通知。
- 完整时隙调度。
- 完整控制闭环。
- 多机高可用。
- OTA。
- AI 分析。
- 完整报表中心。

这些能力需要在真实硬件 raw 包、ACK 语义和时隙策略确认后再进入实现计划。
