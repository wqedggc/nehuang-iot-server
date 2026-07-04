# 涅凰智农 IoT 当前产品现状与 Gap

> 更新日期：2026-07-05  
> 口径：本文件记录当前代码真实实现和甲方最新硬件侧逻辑带来的产品/联调差距，不以历史方案文档中的设想为准。

## 一、当前结论

当前系统已经从“只靠 HTTP Adapter 上报 JSON”的阶段，推进到：

```text
硬件网关 TCP
  -> Go gateway-adapter 原始收包和 raw journal
  -> 全量 raw frame 上报 Go 服务
  -> Go 服务先写 raw_device_events，再最小解析 register/heartbeat
  -> Go nehuang-server 维护 MySQL 主模型
  -> Admin 管理已有设备点位和绑定
  -> Mini/C 端查看已绑定设备并创建异步控制命令
```

当前最关键的边界是：

- `gateway-adapter` 先解决“真实硬件 bytes 能稳定进云端、能留痕、能回放”。
- `nehuang-server` 继续作为业务事实来源，负责用户、绑定、设备点位、控制命令、Admin/Mini API。
- 甲方最新硬件文档中的 4 小时累计工作包、时隙调度、告警重复上报、ACK/清缓存语义还没有完成代码实现。

因此，当前产品已经具备 **本地/云端联调入口**，但还不是完整硬件闭环。

## 二、已实现范围

### 2.1 Go 服务端

已实现：

- Admin 登录、Dashboard、用户/农户管理。
- 设备树：`cloud_gateways -> collector_nodes -> device_points`。
- 功能点搜索、绑定、解绑。
- 点位配置 API：
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
- 原始事件查看。
- Payload 排查页面。
- Mini 微信登录接口。
- Mini 农户资料、设备列表、实时数据、历史曲线。
- Dashboard/Mini history 支持 `1d/3d/7d/30d`，兼容 `1h/6h/24h`。
- 后台 offline scanner 按阈值修正 gateway/node/point 离线状态。
- migration runner 和 MySQL 5.7/8.0 兼容 collation。

### 2.2 Gateway Adapter

已实现：

- 独立二进制 `cmd/gateway-adapter`。
- TCP 监听硬件网关连接，默认 `NEHUANG_GATEWAY_TCP_ADDR=:8081`。
- 健康检查端口，默认 `NEHUANG_GATEWAY_HEALTH_ADDR=:18081`。
- 每个 TCP 连接生成 `connection_id`。
- 收到任何 bytes 后立即写 raw frame journal。
- 将每次 TCP read 到的原始 bytes 上报到 `POST /api/v1/adapter/raw-frames`。
- runtime log 使用结构化 JSON。
- debug/protocol log 通过 `NEHUANG_GATEWAY_DEBUG=true` 开启。
- 识别 ASCII `FP_xxx` 注册包。
- 识别 `HEARTBEAT/HB/PING/HEART` 心跳包。
- 注册/心跳的业务解析由 Go 服务在 raw frame 落库后完成。
- CRC-16/MODBUS 固定实现。
- `cmd/gateway-adapter-mock` 可模拟注册、心跳、工作包、告警包、未知包。
- `cmd/gateway-adapter-replay` 可回放 raw journal 并统计分类结果。

当前限制：

- 工作包已经能进入 Go 服务 `raw_device_events`，但仍不解析字段。
- 告警包已经能进入 Go 服务 `raw_device_events`，但仍不入业务告警。
- 未实现真实下行 hex 写回硬件连接。
- 未实现时隙调度。

### 2.3 控制链路

已实现：

- `control_commands` / `control_results` 领域模型。
- Mini 控制请求创建异步控制命令并返回 `command_id + pending`。
- Adapter 内部接口：
  - 拉取 pending command。
  - 标记 sent。
  - 回写 ack/succeeded/failed/timeout。

未实现：

- Admin 控制发起页面。
- Adapter 真实下发 hex。
- MCU ACK 解析。
- 命令超时扫描完整接入。
- 控制结果驱动前端状态刷新。

### 2.4 设备接入与主数据

当前设备主数据仍由硬件上报驱动：

- raw frame 中的 register/heartbeat 会创建或更新 `cloud_gateways`。
- telemetry/work 后续应创建或更新：
  - `collector_nodes`
  - `device_points`
  - `sensor_data`
  - `device_realtime_maps`

当前没有实现“后台手动新增设备/网关/采集执行器/功能点”的完整产品能力。

后台的“绑定设备”实际含义是：

```text
从已经上报并入库的 device_points 中选择一个功能点
  -> 绑定给某个农户
```

不是：

```text
管理员手动填写 gateway / frequency / device_external_id / function_type
  -> 创建一个新硬件设备
```

### 2.5 小程序端

服务端已实现：

- 微信 `wx.login` code 换取 mini token：`POST /api/v1/mini/login`。
- 浏览器调试 token 方式仅用于 H5 调试，不是正式登录。
- 获取和编辑农户资料。
- 查看已绑定设备列表。
- 查看设备实时数据。
- 查看历史曲线。
- 创建异步控制命令。

小程序端当前状态：

- Taro 小程序代码在 `/Users/shore/WorkBuddy/shoreos-taro`。
- 已增加涅凰独立构建入口 `TARO_APP_TARGET=nehuang`。
- 涅凰登录页已走 `Taro.login()` -> `/api/v1/mini/login`。
- 尚未完成微信开发者工具/真机端到端验收。

### 2.6 部署与运维

已实现：

- `deploy.sh` 支持基础 migration runner。
- 腾讯云 Go 服务部署链路已跑通。
- `/healthz`、`/readyz` 可用于部署后检查。
- 线上 mock login 关闭。

未完成：

- `gateway-adapter` 还没有纳入正式腾讯云 systemd/deploy 流程。
- 硬件 TCP 端口、防火墙、安全组、日志目录和进程守护需要部署方案固化。
- Nginx stream 仅作为可选方案，第一版默认硬件 TCP 直连 Adapter。

## 三、甲方最新硬件文档带来的新增要求

甲方最新硬件侧逻辑说明书明确了以下方向：

- MCU 本地承担温控判断和保护逻辑，云端不再执行旧 `jisuan.py`。
- MCU 每 5 分钟缓存一组温度，标准 4 小时上报 48 组。
- 标准工作包最大约 163 字节。
- 云端读取成功后，需要通知 MCU 清空缓存并重新计时。
- 单个云边集群按最多 100 台采集执行器规划，每台 2 秒专有时序，满配 200 秒周期。
- 上摆周期用于云端下发读取/控制/同步；下摆周期用于 MCU 上报。
- 告警包需要在后续 3 个自有时隙重复上报，云端需要去重。
- 手动控制命令暂定为 `F0` 开、`0F` 关、`00` 停。
- Modbus CRC-16 作为包级校验，云端和硬件端算法必须一致。

这些要求已经整理到 `docs/hardware_protocol_current.md`。

## 四、主要 Gap

| Gap | 当前状态 | 影响 | 需要甲方确认 |
|-----|----------|------|--------------|
| 真实工作包 parser | 未实现 | 不能把 4 小时温度记录写入 `sensor_data` | 最终 bytes 样例、长度字段、温度编码、CRC 覆盖范围 |
| 告警 parser 与去重 | 未实现 | R/S 故障不能进入告警业务和通知 | 告警包最终格式、重复 3 次的去重窗口 |
| ACK/清缓存语义 | 未实现 | 无法确认 MCU 是否已收到、是否可清缓存 | `AA AA BB ZZ` 在不同场景的准确含义 |
| 时隙调度 | 未实现 | 不能按 2 秒窗口可靠下发命令 | 时隙由云端、Adapter、硬件网关还是 MCU 维护 |
| 控制真实下发 | 命令队列骨架已实现 | Mini/Admin 控制不能真正到硬件 | 下行 hex 完整格式和 ACK 格式 |
| Adapter 生产部署 | 未完成 | 云端还不能让硬件正式切到新 TCP 入口 | TCP 端口、安全组、systemd、日志目录 |
| 设备首次入库 | register 已有，work 入点位未完成 | 真实采集执行器和功能点不能自动完整建档 | 工作包里如何表达 function_type/channel_no |
| 手动新增设备 | 未实现 | 如果甲方要施工前录入设备，当前不满足 | 是否需要手动创建 gateway/node/point 或 Excel 导入 |
| 点位配置 UI | API 已实现，H5 编辑页不完整 | Admin 仍不方便配置点位 | 是否一期必须有可视化点位编辑 |
| 小程序真机验收 | 后端具备接口，端到端未验收 | 不能确认微信生产登录完整可用 | AppID、域名、测试账号、真机验收流程 |
| 告警通知 | 未实现 | 故障不能推送给农户 | 通知渠道、模板、频率、免打扰规则 |
| 生产安全 | 基础可用，未完整加固 | 线上风险 | HTTPS、域名、备份、审计、密码策略 |

## 五、当前最优先确认事项

1. 真实硬件 TCP 到云端 Adapter 的联调方式：
   - 硬件网关是否可以作为 TCP client 连接云端 `IP:8081`。
   - 注册包和心跳包真实 bytes 是什么。
   - 工作包真实 bytes 样例是什么。
2. 工作包字段：
   - `AA AA` 是 2 字节设备 ID 还是两个独立字段。
   - 温度 `XX XX` 是否为有符号 16 位，是否除以 10。
   - `YY` 是总长度还是数据组数。
   - CRC 覆盖范围和字节顺序。
3. 时隙/控制：
   - 云端是否必须主动卡上摆窗口。
   - 硬件网关是否能缓存下行命令并在窗口到达时转发。
   - MCU 是否支持主动取 pending command。
4. ACK/清缓存：
   - MCU 上报历史数据后，云端用什么包确认收到。
   - MCU 什么时候清空缓存。
   - 连续失败 5 次后的告警和数据丢弃如何对用户展示。
5. 产品流程：
   - 设备是否必须真实上报后才出现在后台。
   - 是否需要后台手动新增/导入设备。
   - 新设备首次上报后是否进入“待配置/待确认”列表。

## 六、建议下一步

当前下一步不应先做复杂 UI，而应先完成硬件接入的最小闭环：

1. 部署 `gateway-adapter` 到腾讯云独立端口。
2. 让甲方硬件网关连 TCP 端口并发送注册、心跳、工作包样例。
3. 用 raw journal 和 replay 工具确认真实包形态。
4. 根据真实样例补 parser。
5. 先打通：
   - 注册
   - 心跳
   - 一个工作包入 `sensor_data`
   - 一个告警包入告警记录
   - 一个控制命令 pending -> sent -> ack
6. 再补 Admin/Mini 的完整展示和操作体验。

在真实 raw 包稳定前，不建议把字段偏移、时隙策略、ACK/清缓存语义写死进生产业务逻辑。
