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

| Method | Path | Auth | Status |
|--------|------|------|--------|
| POST | `/api/v1/adapter/device-events` | `X-Adapter-Token` | 正式 |

说明：Adapter 事件入口接收 register、heartbeat、telemetry/work、control_result 等 envelope。当前 control_result 仍不是完整控制闭环。

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
| POST | `/api/v1/admin/bindings` | 正式 |
| DELETE | `/api/v1/admin/bindings/{bind_id}` | 正式 |
| GET | `/api/v1/admin/raw-events?limit=` | 正式 |
| GET | `/api/v1/admin/control-logs?limit=` | 占位视图，依赖 `op_records` |
| GET | `/api/v1/admin/device-tree` | 正式 |
| GET | `/api/v1/admin/device-points?keyword=&bind_status=` | 正式 |
| PATCH | `/api/v1/admin/device-points/{point_id}` | 正式，更新点位配置 |
| GET | `/api/v1/admin/payload-events` | 正式 |
| GET | `/api/v1/admin/payload-events/{raw_event_id}` | 正式 |

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
| POST | `/api/v1/mini/devices/{point_id}/control` | 占位，尚未接完整控制闭环 |

## Shared

| Method | Path | Auth | Status |
|--------|------|------|--------|
| GET | `/api/v1/gateways` | Admin token | 正式 |
