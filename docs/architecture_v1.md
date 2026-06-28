# 涅凰智农 IoT 云平台 V1.0 架构说明

> 注意：本文记录架构边界和模块划分。最终物理表名、字段类型、字段命名、索引和写库 cycle 以 `schema/mysql/001_nehuang_v1.sql`、`schema/mysql/002_device_node_point_model.sql` 与 `docs/table_structure_v1.md` 为准。

## 1. 结论

V1 不是继续维护旧 Python 业务系统，而是在保留现有 Gateway 通信能力的基础上建设新的 Go 云端业务平台。

必须保留甲方已经定义的 5 张核心表和字段语义：

- `cloud_gateway`：云边 E90/FP 设备在线状态。
- `bind_relation`：农户与云边设备/采集执行器关系。
- `device_realtime_map`：智农采集执行器实时状态与配置。
- `sensor_data`：历史采集曲线数据。
- `op_record`：手动操作记录。

需要新增的平台能力：

- 用户/管理员登录与角色。
- 采集执行器节点模型 `collector_nodes`。
- 节点下功能点/通道模型 `device_points`。
- 原始设备事件归档。
- 控制命令状态机。
- 控制结果回传记录。
- 协议命令模板。
- B 端和 C 端 API 投影。

## 2. 当前代码事实

当前压缩包中较可信的可运行路径是：

```text
003rukou.py
  -> tcp_gateway.py
  -> 01app_core/data_cache_huancun/devices.json
  -> 01app_core/routes/*
```

`tcp_gateway.py` 当前只完成：

- TCP Server，监听 `8081`。
- 下发接口，监听 `8083` 的 `/send?did=&cmd=`。
- 识别包含 `FP` 的注册包。
- 将设备在线状态写入 `devices.json`。
- 尝试更新 `cloud_gateway`。

当前没有完成：

- 工作包解析。
- `last_data` 写入。
- 温度/墒情/限位数据入库。
- 控制命令状态追踪。
- 微信用户登录。
- 管理员账号体系。

因此当前“设备端到服务端”的通路只打通了在线状态，不是完整采集数据通路。

## 3. 甲方资料约束

甲方的字段表明确要求保留以下字段。

### 3.1 云边数据汇总表

`cloud_gateway` 保留：

- `id`
- `dev_id`
- `dev_name`
- `ip`
- `port`
- `dev_sn`
- `online`
- `create_time`
- `last_heartbeat`

含义：`FP_` 前缀设备只要心跳正常，就进入该表；35 秒内有心跳视为在线。

### 3.2 农户关联表

`bind_relation` 保留：

- `bind_id`
- `bind_usr_name`
- `id`
- `shed_no`
- `freq`
- `devid`
- `func`

含义：农户绑定的不只是一个 Gateway，还要能定位到某个棚号、频点、采集执行器 ID 和功能类型。

### 3.3 采集器实时表

`device_realtime_map` 保留：

- `de_re_id`
- `id`
- `freq`
- `devid`
- `func`
- `best_min`
- `best_max`
- `unit`
- `real_temp`
- `run_mode`
- `rel1`
- `rel2`
- `poll_interval`
- `poll_cmd`

含义：这是 B 端/C 端操作页的旧字段来源。同一 `id + freq + devid` 在旧模型中唯一；当前新模型以 `collector_nodes` 承接节点身份，以 `device_points` 承接具体功能点，`device_realtime_map` 保留为兼容投影。

### 3.4 时序数据表

`sensor_data` 保留：

- `sensor_id`
- `id`
- `freq`
- `devid`
- `sensor_time`
- `real_temp`
- `poll_interval`

含义：用于温湿度/采集曲线，后续可做 30 天数据清理或月度汇总。

### 3.5 操作记录表

`op_record` 保留：

- `op_id`
- `id`
- `bind_usr_name`
- `freq`
- `devid`
- `run_mode`
- `rel1`
- `rel2`
- `op_time`

含义：`manu` 手动模式下记录超管端和微信端操作；`auto` 自动模式动作不作为用户手动操作写入该表。

## 4. V1 数据流

### 4.1 在线状态

```text
E90
  -> TCP Gateway.py
  -> devices.json
  -> Go Device Adapter
  -> cloud_gateway
  -> REST API
  -> Admin/MiniProgram
```

第一阶段 Go 服务端可以通过轮询或文件监听读取 `devices.json`。后续建议让 Gateway 直接 HTTP 回调 Go Adapter，JSON 文件只保留为调试缓存。

### 4.2 工作数据

```text
E220/485 IO
  -> E90 透传
  -> Gateway 接收工作包
  -> Device Adapter 解析
  -> raw_device_events
  -> collector_nodes
  -> device_points
  -> device_realtime_map optional
  -> sensor_data
```

协议未定稿前，所有原始包先进入 `raw_device_events`。解析成功后再更新实时表和历史表。业务层不得直接处理十六进制。

### 4.3 控制命令

```text
Admin/MiniProgram/Auto Job
  -> Control Service
  -> control_commands(created)
  -> Gateway /send
  -> control_commands(sent)
  -> Device result
  -> control_results
  -> control_commands(succeeded/failed/timeout)
  -> op_record
```

控制命令不能只调用 `/send` 后返回字符串，必须落库记录状态，否则无法排查设备未执行、回包失败、重复点击等问题。

## 5. Go 服务端模块划分

建议目录：

```text
cmd/server
  main.go

internal/config
  配置加载、MySQL、Gateway 地址、JWT/Session 配置

internal/domain
  Gateway、CollectorDevice、User、Binding、SensorReading、ControlCommand 等领域结构

internal/adapter
  devices.json 解析
  注册包/心跳包/工作包解析
  控制命令封装
  控制结果解析

internal/service
  UserService
  DeviceService
  BindingService
  ControlService
  AdminService

internal/repository/mysql
  MySQL 读写实现

internal/api/http
  Admin API
  MiniProgram API
  Adapter API

internal/job
  离线检测
  devices.json 同步
  控制命令超时扫描
  历史数据清理
```

## 6. API 边界

### 6.1 Adapter API

- `POST /api/v1/adapter/gateway/snapshot`
  - 接收或同步 `devices.json` 快照。
- `POST /api/v1/adapter/events`
  - 接收注册、心跳、工作包、控制结果。
- `POST /api/v1/adapter/parse/retry`
  - 重试解析失败原始事件。

### 6.2 Admin API

- `POST /api/v1/admin/login`
- `GET /api/v1/admin/dashboard`
- `GET /api/v1/admin/gateways`
- `GET /api/v1/admin/collectors`
- `POST /api/v1/admin/bindings`
- `DELETE /api/v1/admin/bindings/{bind_id}`
- `POST /api/v1/admin/control-commands`
- `GET /api/v1/admin/control-commands`
- `GET /api/v1/admin/operation-logs`

### 6.3 MiniProgram API

- `POST /api/v1/mini/login`
- `GET /api/v1/mini/me`
- `GET /api/v1/mini/devices`
- `GET /api/v1/mini/devices/{de_re_id}/realtime`
- `GET /api/v1/mini/devices/{de_re_id}/history`
- `PATCH /api/v1/mini/devices/{de_re_id}/config`
- `POST /api/v1/mini/devices/{de_re_id}/control`

## 7. 页面数据投影

### 7.1 C 端“用户及设置”

来源：

- `users`
- `bind_relation`
- `cloud_gateway`
- `device_realtime_map`

展示列：

- 用户名
- 序号
- 棚号
- 频点
- ID
- 功能
- 状态
- 操作

### 7.2 C 端/管理端“操作”

来源：

- `device_realtime_map`
- `sensor_data`
- `control_commands`
- `op_record`

展示列：

- 棚号
- 设备序号
- 功能
- 最佳范围
- 单位
- 实际值
- 模式
- 操作

温度设备使用三档操作：开、停、关。墒情设备使用两档操作：开、关。限位设备以状态读取和展示为主。

## 8. 迁移策略

### 阶段 1：保留 Gateway，修正入库

- 不改现场协议。
- 不改 E90 固件。
- 不改 TCP Gateway 的接入职责。
- Go 服务端先读取 `devices.json`，补齐 `cloud_gateway`。
- 修正 `cloud_gateway` 必填字段问题：`dev_id/dev_sn` 必须有默认值或 Adapter 补齐。

### 阶段 2：新增业务库和 API

- 按顺序执行 `schema/mysql/001_nehuang_v1.sql` 和 `schema/mysql/002_device_node_point_model.sql`。
- 初始化管理员用户。
- 用 Go API 替代页面直接读 JSON。
- C 端和 B 端只访问 REST API。

### 阶段 3：接入工作包解析

- 工作包原始内容进入 `raw_device_events`。
- 按协议解析到 `collector_nodes`、`device_points` 和 `sensor_data`，必要时同步 `device_realtime_map` 兼容投影。
- 解析失败保留原始事件，人工可追踪。

### 阶段 4：控制闭环

- 控制命令先写 `control_commands`。
- Gateway 下发成功只代表“已发送”，不代表“执行成功”。
- 设备回包写 `control_results`。
- 最终更新命令状态。

## 9. 不纳入 V1

- 硬件研发。
- E90 固件修改。
- TCP Gateway 重写。
- OTA。
- AI 分析。
- 多租户。
- 完整报表中心。
- 独立告警中心。

这些能力不要混入 V1 数据模型和服务边界。

## 10. 上线与迭代发布方案

V1 架构需要支持后续新增功能时尽量不影响设备数据接收。

### 10.1 分层稳定性

```text
设备接入层
  -> 数据处理层
  -> MySQL
  -> 业务 API 层
  -> 小程序 / 管理后台
```

- 设备接入层：保留 TCP Gateway，稳定接收硬件上传数据。
- 数据处理层：Device Adapter 解析、校验并落库，原始数据先写 `raw_device_events`。
- 数据库层：MySQL 作为唯一事实来源，固化设备状态、历史数据、绑定关系和控制日志。
- 业务 API 层：给小程序和后台提供查询、配置、控制能力。

后台和小程序新增功能不直接影响 TCP 接入层；业务 API 升级不应破坏已经落库的数据。

### 10.2 单机滚动发布

一期只有一台服务器时，业务 API 可以在同一台机器上启动两个实例：

```text
Nginx
  -> nehuang-api-a
  -> nehuang-api-b
```

发布流程：

1. Nginx 同时代理两个 API 实例。
2. 先摘掉实例 A 的流量。
3. 升级实例 A。
4. 调用 `/healthz` 和 `/readyz` 做健康检查。
5. 检查通过后把实例 A 加回流量。
6. 再按同样步骤升级实例 B。

该方案能让后台和小程序接口做到低停机、秒级切换和可回滚。

### 10.3 不能过度承诺

单机部署仍然存在单点：

- 服务器单点。
- MySQL 单点。
- TCP Gateway 单点。
- 设备 TCP 长连接在 Gateway 重启时可能短暂断开。

因此对外表述应为：一期可做到低停机、可回滚、可恢复，不承诺大型集群意义上的绝对零中断。

### 10.4 接口兼容

小程序页面可能存在新旧版本并存。服务端新增功能时遵守：

- 旧接口尽量保持兼容。
- 新功能优先新增接口或新增字段。
- 不让新增曲线、配置、控制模块影响已有设备列表、实时数据和控制接口。
