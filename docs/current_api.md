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
- `mock telemetry` 联调文本帧示例：
  `NH_TELEMETRY|frequency=1|device_external_id=1001|function_type=1|channel_no=1|value=26.5|unit=C`。
- raw 入口对注册、心跳、工作、告警、CRC 错误和 unknown 一视同仁：先写本地 durable journal，再上报 Server；分类结果不决定是否留存。
- 在线口径：心跳只刷新设备网关在线状态；只有工作/telemetry 才会刷新采集执行器与功能点状态、实时值。

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

说明：`gateway-adapter` 把每次 TCP `Read` 得到的 bytes 作为 `tcp_chunk` 写入本地分段 journal，然后全部上报 `/api/v1/adapter/raw-frames`。上报失败时保留未确认记录并自动补传；Server 使用 `source_type + capture_id` 幂等落 `raw_device_events`。一次 `Read` 不等于逻辑包，外层分帧规则确认前管理端展示的是原始捕获块。第一版只稳定解析 `FP_` 注册包和显式心跳包；MCU1 候选包可展示协议摘要，但工作/告警/unknown 的业务投影仍保持 pending。

`GET /api/v1/admin/payload-events` 支持 `source_type` 筛选：`1` TCP Gateway、`2` 结构化 Adapter、`3` 手动录入、`4` Mock 脚本。管理端原始报文排查默认使用 `source_type=1`。

### POST `/api/v1/adapter/raw-frames`

请求字段：

- `protocol_version`
- `source_type`
- `message_id`
- `capture_id`（新 Adapter 必传；旧 Adapter 缺省时兼容使用 `message_id`）
- `capture_kind`（`tcp_read` 或 `tcp_write_attempt`；旧 Adapter 可省略）
- `connection_id`
- `gateway_name`
- `remote_ip`
- `remote_port`
- `direction`
- `payload_hex`
- `received_at`
- `raw_seq`

返回 `raw_event_id` 和 `parse_status`。该接口不要求同步完成业务解析。

`capture_id` 在第一次本地落盘后固定，补传必须复用；`message_id` 保留协议/历史兼容。重复 capture 返回原 `raw_event_id`、`parse_status=duplicate` 和 `duplicate=true`，不会再次执行业务投影。

### POST `/api/v1/adapter/device-events`

请求字段：

- `protocol_version`
- `source_type`
- `message_id`
- `gateway_name`
- `gateway_external_id`
- `remote_ip`
- `remote_port`
- `reported_at_ms`
- `events`

返回 `raw_event_id` 和 `parse_status`。该接口用于结构化事件上报（如 `telemetry` / `register` / `heartbeat`），可用于联调场景回放真实字段。

`telemetry` 联调最小例子：

```json
{
  "protocol_version":"nehuang-gateway-adapter-v1",
  "source_type":"gateway_adapter",
  "message_id":"mock-telemetry-001",
  "gateway_name":"FP_TEST_ADMIN_001",
  "gateway_external_id":"TEST_001",
  "remote_ip":"127.0.0.1",
  "remote_port":12345,
  "reported_at_ms":"2026-07-06T10:00:00.000Z",
  "events":[
    {
      "event_type":"telemetry",
      "frequency":1,
      "device_external_id":"1001",
      "function_type":1,
      "channel_no":1,
      "value":26.5,
      "unit":"C"
    }
  ]
}
```

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
| GET | `/api/v1/admin/farmers/{user_id}/devices` | 正式；按 point/collector/exclusive-gateway 绑定投影可见功能点，shared 网关不扩大范围 |
| GET | `/api/v1/admin/farmers/{user_id}/devices/{point_id}/history?range=1d` | 正式；先校验功能点属于目标农户，支持 `1h/6h/1d/24h/3d/7d/30d` |
| GET | `/api/v1/admin/collectors` | 正式 |
| GET | `/api/v1/admin/bindings` | 正式 |
| POST | `/api/v1/admin/bindings` | 正式；支持 `binding_type=gateway/point/collector` |
| DELETE | `/api/v1/admin/bindings/{bind_id}` | 正式 |
| GET | `/api/v1/admin/gateway-bind-candidates?keyword=` | 正式；返回设备网关候选（含 access_mode 与已绑定农户） |
| PATCH | `/api/v1/admin/gateways/{gateway_id}` | 正式；更新 `access_mode=shared/exclusive` |
| POST | `/api/v1/admin/gateways/{gateway_id}/collectors` | 正式建档；给已注册设备网关创建/更新采集执行器和默认功能点 |
| POST | `/api/v1/admin/gateways/{gateway_id}/seed-points` | 兼容旧路径；内部等同 `/collectors` |
| GET | `/api/v1/admin/raw-events?limit=` | 正式 |
| GET | `/api/v1/admin/control-logs` | 正式；读取 `control_commands`，展示来源、command hex、命令状态、下发/完成时间，并附展示型下行 `protocol_summary` |
| GET | `/api/v1/admin/operation-logs` | 正式；读取 `op_records`，展示绑定、解绑、配置、控制等操作审计 |
| GET | `/api/v1/admin/device-tree` | 正式 |
| GET | `/api/v1/admin/device-points?keyword=&bind_status=` | 正式 |
| PATCH | `/api/v1/admin/device-points/{point_id}` | 正式，更新点位配置 |
| POST | `/api/v1/admin/device-points/{point_id}/control` | 正式创建异步控制命令，最终下发和 ACK 由 Adapter 完成 |
| GET | `/api/v1/admin/payload-events` | 正式 |
| GET | `/api/v1/admin/payload-events/{raw_event_id}` | 正式 |

说明：日志类接口分页/筛选能力说明：

- `payload-events` 支持分页+筛选：`page`、`size`、`event_type`、`parse_status`、`start`、`end`、`keyword`。
- `payload-events` 列表会附带展示型 `protocol_label` 和可识别时的 `device_identity`；原始事件 ID 是数据库记录 ID，`message_id` 是 Adapter 生成的“TCP 连接 ID-连接内帧序号”。
- `control-logs` 支持分页+筛选：`page`、`size`、`start`、`end`、`keyword`、`cmd_status`、`source`；不带 `page/size` 时兼容旧数组响应。每条命令的 `protocol_summary` 按下行方向解释 MCU1/Modbus 字段；旧 `F0/0F/00` 会标识为非完整 MCU1 联调控制字。
- `operation-logs` 支持分页+筛选：`page`、`size`、`start`、`end`、`keyword`、`op_type`；不带 `page/size` 时兼容旧数组响应。
- `raw-events` 当前保留 `limit` 模式。

农户数据工作台接口不复用 Mini HTTP token。设备摘要复用同一套绑定权限 SQL；历史点同时保留兼容字段 `real_temp` 和通用字段 `value`，管理端优先读取 `value`。历史查询按范围使用固定时间桶，每桶保留最后一条样本并按时间升序返回，最多 500 点，避免长区间只显示最早一段数据。停用、删除或不在目标农户范围内的功能点统一返回 404，避免共享设备网关造成横向数据展开。

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
| GET | `/api/v1/mini/control-commands/{command_id}` | 正式；查询当前农户已绑定设备的异步控制命令状态 |

### GET `/api/v1/mini/control-commands/{command_id}`

返回 `command_id`、`status`、`finished_at_ms`、`error_message`。`command_id` 保持字符串；`status` 使用既有 `pending`、`sent`、`ack`、`succeeded`、`failed`、`timeout` 语义。完成时间和错误信息尚未产生时返回 `null`。

接口会按命令关联的 `point_id` 复用现有绑定权限检查；point、collector 绑定以及 exclusive gateway 绑定均沿用 C 端设备可见性规则，未绑定返回 `403`。

## Shared

| Method | Path | Auth | Status |
|--------|------|------|--------|
| GET | `/api/v1/gateways` | Admin token | 正式 |
