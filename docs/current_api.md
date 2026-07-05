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

HTTP Adapter API 是 Go `gateway-adapter` 和 Go `nehuang-server` 之间的内部接口。硬件网关第一版不直接调用这些 HTTP 接口，而是作为 TCP client 连接 `gateway-adapter`。硬件 TCP 交互见 `docs/gateway_adapter.md`。

联调阶段新增口径：

- 统一优先使用自定义 `command_hex` 下行，避免在联调期先把标准下行语义写死。
- 最新沟通口径倾向于云端点名读取实时数据和周期累计记录；此前“硬件主动上行标准工作包”的口径需以最终协议样例统一。
- 临时读取采用“云端下发读取 hex 后返回 raw”。
- 工作/告警/ACK/清缓存/4 小时读取字段语义未最终冻结，当前只做原始留存与状态追踪。

API 与展示口径（当前）：

- 雪花 ID 与身份字段按字符串返回，避免 JS Number 精度丢失；典型包含：`user_id`、`gateway_id`、`point_id`、`bind_id`、`command_id`、`raw_event_id` 等。
- 传感器值与状态在内部保持数值类型，继续按数字入库、聚合与计算。
- 网关口径字段新增 `access_mode`（`shared` / `exclusive`）；共享/独享会影响网关绑定后的数据展开范围。

| Method | Path | Auth | Status |
|--------|------|------|--------|
| POST | `/api/v1/adapter/raw-frames` | `X-Adapter-Token` | 正式，硬件 TCP raw frame 主入口 |
| POST | `/api/v1/adapter/device-events` | `X-Adapter-Token` | 正式 |
| GET | `/api/v1/adapter/commands/pending?gateway_external_id=` | `X-Adapter-Token` | 新增，Adapter 拉取待下发命令 |
| POST | `/api/v1/adapter/commands/{command_id}/sent` | `X-Adapter-Token` | 新增，Adapter 标记已下发 |
| POST | `/api/v1/adapter/commands/{command_id}/result` | `X-Adapter-Token` | 新增，Adapter 回写 ACK/成功/失败/超时 |

说明：`gateway-adapter` 的 TCP 主链路会把每次收到的 bytes 上报到 `/api/v1/adapter/raw-frames`，Server 先写 `raw_device_events`。第一版解析 `FP_` 注册包，以及 `keepalive/keepalive message/HEARTBEAT/HB/PING/HEART` 心跳包；工作包、告警包、未知包会先按 `event_type=3/6/7` 分开展示，但业务解析仍保持 pending，等真实协议稳定后补 parser。`/api/v1/adapter/device-events` 保留给已结构化 JSON 上报和旧联调脚本。

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
| POST | `/api/v1/admin/bindings` | 正式；支持 `binding_type=gateway/point/collector` |
| DELETE | `/api/v1/admin/bindings/{bind_id}` | 正式 |
| GET | `/api/v1/admin/gateway-bind-candidates?keyword=` | 正式；返回设备网关候选（含 access_mode 与已绑定农户） |
| PATCH | `/api/v1/admin/gateways/{gateway_id}` | 正式；更新 `access_mode=shared/exclusive` |
| POST | `/api/v1/admin/gateways/{gateway_id}/collectors` | 正式建档；给已注册设备网关创建/更新采集执行器和默认功能点 |
| POST | `/api/v1/admin/gateways/{gateway_id}/seed-points` | 兼容旧路径；内部等同 `/collectors` |
| GET | `/api/v1/admin/raw-events?limit=` | 正式 |
| GET | `/api/v1/admin/control-logs?limit=` | 正式；读取 `control_commands`，展示来源、command hex、命令状态、下发/完成时间 |
| GET | `/api/v1/admin/operation-logs?limit=` | 正式；读取 `op_records`，展示绑定、解绑、配置、控制等操作审计 |
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
- 新增采集执行器绑定：`binding_type=collector`、`user_id`、`node_id`（或由 `point_id` 补全）

默认产品口径是农户绑定采集执行器优先；网关绑定作为共享接入关系保留，按 `access_mode` 控制可见范围：
- `exclusive`：可按网关展开全部采集执行器。
- `shared`：只按绑定到该农户的 `collector`/`point` 授权展开。
- 旧功能点绑定继续可读、可解绑、可创建，用于兼容历史数据和精细授权场景。

- `DELETE /api/v1/admin/bindings/{bind_id}` 按解绑语义落地。
- `op_records` 已作为操作审计表，绑定/解绑在服务端链路中会写入 `operation-logs`。

### PATCH `/api/v1/admin/gateways/{gateway_id}`

请求字段：

- `access_mode`: `shared` 或 `exclusive`

含义：

- `shared`: 设备网关是共享接入设施，农户端只展示直接绑定给该农户的采集执行器/功能点。
- `exclusive`: 设备网关独享，农户绑定该网关后可展开该网关下全部采集执行器/功能点。

### POST `/api/v1/admin/gateways/{gateway_id}/collectors`

正式建档接口。用途是在设备网关已注册但真实工作包 parser 尚未可稳定补齐时，管理员显式创建或更新采集执行器和默认功能点，方便验证后台设备管理、农户绑定、C 端设备列表和下行 Hex。
正式口径下，协议未冻结前采集执行器以网关上下文手工建档为先，暂不承诺硬件包自动同步新增/更新采集执行器。

请求字段：

- `frequency`: 采样频点，默认前端填 `1`
- `device_external_id`: 采集执行器 DevID，必填
- `node_name`: 采集执行器名称，可空
- `shed_no`: 棚号，可空
- `remark`: 备注，可空
- `points`: 可选；不传时后端默认创建温度、墒情、水泵、风机 4 个联调功能点

重复调用同一 `gateway_id + frequency + device_external_id` 会更新已有采集执行器；同一 `node_id + function_type + channel_no` 会更新已有功能点，不重复制造点位。

`POST /api/v1/admin/gateways/{gateway_id}/seed-points` 作为旧联调路径保留，行为与 `/collectors` 一致。

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
