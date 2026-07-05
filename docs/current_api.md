# 涅凰智农 IoT 当前 API

本文件只记录当前代码中真实注册的接口。历史方案文档中的未实现接口不以此为准。

## Public

| Method | Path | Auth | Status |
|--------|------|------|--------|
| GET | `/healthz` | 无 | 正式 |
| GET | `/readyz` | 无 | 正式 |
| POST | `/api/v1/admin/login` | 无 | 正式 |
| POST | `/api/v1/mini/login` | 无 | 正式，需要微信配置 |
| POST | `/api/v1/mini/mock-login` | 无 | 调试，仅 `NEHUANG_ENABLE_MOCK_LOGIN=true` 可用 |

## Adapter

HTTP Adapter API 是 Go `gateway-adapter` 和 Go `nehuang-server` 之间的内部接口。甲方硬件网关第一版不直接调用这些 HTTP 接口，而是作为 TCP client 连接 `gateway-adapter`。硬件 TCP 交互见 `docs/gateway_adapter.md`。

联调阶段新增口径：

- 统一优先使用自定义 `command_hex` 下行，避免在联调期先把标准下行语义写死。
- 标准 4 小时工作包按硬件主动上行保留；临时读取采用“云端下发读取 hex 后返回 raw”。
- ACK/清缓存/完整工作包字段语义未最终冻结，当前只做原始留存与状态追踪。

| Method | Path | Auth | Status |
|--------|------|------|--------|
| POST | `/api/v1/adapter/raw-frames` | `X-Adapter-Token` | 正式，硬件 TCP raw frame 主入口 |
| POST | `/api/v1/adapter/device-events` | `X-Adapter-Token` | 正式 |
| GET | `/api/v1/adapter/commands/pending?gateway_external_id=` | `X-Adapter-Token` | 新增，Adapter 拉取待下发命令 |
| POST | `/api/v1/adapter/commands/{command_id}/sent` | `X-Adapter-Token` | 新增，Adapter 标记已下发 |
| POST | `/api/v1/adapter/commands/{command_id}/result` | `X-Adapter-Token` | 新增，Adapter 回写 ACK/成功/失败/超时 |

说明：`gateway-adapter` 的 TCP 主链路会把每次收到的 bytes 上报到 `/api/v1/adapter/raw-frames`，Server 先写 `raw_device_events`。第一版只解析 `FP_` 注册包和 `HEARTBEAT` 心跳包；工作包、告警包、未知包会先按 `event_type=3/6/7` 分开展示，但业务解析仍保持 pending，等真实协议稳定后补 parser。`/api/v1/adapter/device-events` 保留给已结构化 JSON 上报和旧联调脚本。

### POST `/api/v1/adapter/raw-frames`

请求字段：

- `protocol_version`
- `source_type`
- `message_id`
- `connection_id`
- `gateway_name`
- `remote_ip`
- `remote_port`
- `direction`
- `payload_hex`
- `received_at`
- `raw_seq`

返回 `raw_event_id` 和 `parse_status`。该接口不要求同步完成业务解析。

## Admin

所有 Admin 接口需要 `Authorization: Bearer <admin token>`，JWT `aud=admin`，角色为 super_admin/admin。

| Method | Path | Status |
|--------|------|--------|
| GET | `/api/v1/admin/me` | 正式 |
| GET | `/api/v1/admin/dashboard?range=1d` | 正式；支持 `1d/3d/7d/30d`，兼容 `1h/6h/24h` |
| GET | `/api/v1/admin/users` | 正式 |
| GET | `/api/v1/admin/farmers?keyword=` | 正式 |
| GET | `/api/v1/admin/farmers/{user_id}` | 正式 |
| PATCH | `/api/v1/admin/farmers/{user_id}` | 正式 |
| GET | `/api/v1/admin/farmers/{user_id}/bindings` | 正式 |
| GET | `/api/v1/admin/collectors` | 正式 |
| GET | `/api/v1/admin/bindings` | 正式 |
| POST | `/api/v1/admin/bindings` | 正式；支持 `binding_type=gateway/point` |
| DELETE | `/api/v1/admin/bindings/{bind_id}` | 正式 |
| GET | `/api/v1/admin/gateway-bind-candidates?keyword=` | 正式；返回未 active 绑定的设备网关候选 |
| GET | `/api/v1/admin/raw-events?limit=` | 正式 |
| GET | `/api/v1/admin/control-logs?limit=` | 正式；读取 `control_commands`，展示 command hex 和命令状态 |
| GET | `/api/v1/admin/device-tree` | 正式 |
| GET | `/api/v1/admin/device-points?keyword=&bind_status=` | 正式 |
| PATCH | `/api/v1/admin/device-points/{point_id}` | 正式，更新点位配置 |
| POST | `/api/v1/admin/device-points/{point_id}/control` | 正式创建异步控制命令，最终下发和 ACK 由 Adapter 完成 |
| GET | `/api/v1/admin/payload-events` | 正式 |
| GET | `/api/v1/admin/payload-events/{raw_event_id}` | 正式 |

### POST `/api/v1/admin/bindings`

请求字段：

- 网关级绑定：`binding_type=gateway`、`user_id`、`gateway_id`
- 兼容功能点绑定：`binding_type=point`、`user_id`、`point_id`

默认产品口径是农户绑定设备网关：一个农户可绑定多个设备网关；同一设备网关同时只能有一个 active 农户绑定。旧功能点绑定继续可读、可解绑、可创建，用于兼容历史数据和精细授权场景。

### POST `/api/v1/admin/device-points/{point_id}/control`

请求字段：

- `action`: `open`、`close`、`stop`
- `command_hex`: 可选；传入后按自定义 hex 创建命令，优先于 `action`
- `command_hex` 可承载“临时读取 hex”；临时读取返回仍先按 raw 上行入库。

返回 `command_id`、`status=pending`、`command_hex`。该接口只创建异步控制命令，不等待真实硬件 ACK；临时读取命令返回仍走 raw 回流链路，待联调确认字段语义后再入解析。

### PATCH `/api/v1/admin/device-points/{point_id}`

支持字段：

- `point_name`
- `shed_no`
- `unit`
- `best_min`
- `best_max`
- `run_mode`
- `controllable_status`
- `poll_interval`
- `poll_cmd`
- `point_status`
- `remark`

`shed_no` 写入 `collector_nodes`；兼容字段会同步到 `device_realtime_maps`。

## Mini

所有 Mini 接口需要 `Authorization: Bearer <mini token>`，JWT `aud=mini`，农户角色可访问。

| Method | Path | Status |
|--------|------|--------|
| GET | `/api/v1/mini/me` | 正式 |
| PATCH | `/api/v1/mini/me` | 正式 |
| GET | `/api/v1/mini/devices` | 正式 |
| GET | `/api/v1/mini/devices/{point_id}/realtime` | 正式 |
| GET | `/api/v1/mini/devices/{point_id}/history?range=1d` | 正式；支持 `1d/3d/7d/30d`，兼容 `1h/6h/24h` |
| POST | `/api/v1/mini/devices/{point_id}/control` | 正式创建异步控制命令，最终下发和 ACK 由 Adapter 完成 |

## Shared

| Method | Path | Auth | Status |
|--------|------|------|--------|
| GET | `/api/v1/gateways` | Admin token | 正式 |
