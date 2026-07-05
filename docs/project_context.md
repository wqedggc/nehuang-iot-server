# 涅凰智农项目上下文入口

> 更新日期：2026-07-05

## 1. 项目定位

涅凰智农 V1.0 是农业自动化 IoT 云平台。当前目标不是重写 MCU、E90/E220 固件或现场温控算法，而是在硬件端透传能力基础上建设稳定的云端接入、业务数据固化、管理后台和农户端能力。

当前正式工程落点：

```text
/Users/shore/Desktop/ShoreOS/services/nehuang-iot-server
```

当前服务由两个 Go 进程构成：

- `cmd/server`：HTTP API、Admin/Mini 静态页面、MySQL 业务读写。
- `cmd/gateway-adapter`：硬件 TCP 接入、raw journal、最小报文分类和事件转发。

旧 `tcp_gateway.py` 和 `devices.json` 只作为历史参考和联调对照，不再作为当前正式架构默认路径。

## 2. 当前资料入口

当前事实优先级：

1. 当前代码和 schema。
2. `docs/current_product_status.md`。
3. `docs/current_api.md`。
4. `docs/gateway_adapter.md`。
5. `docs/hardware_protocol_current.md`。
6. `docs/architecture_v1.md`。
7. `docs/data_model_v1.md`。

硬件端最新文档已整理到：

- `docs/hardware_protocol_current.md`

## 3. 当前四层边界

```text
采集执行器 MCU
  -> 云边/LoRa/TCP 硬件网关
  -> Go gateway-adapter
  -> Go nehuang-server + MySQL + Admin/Mini
```

边界说明：

- MCU 负责本地温控判断、继电器保护、温度缓存、限位/故障状态。
- 硬件网关负责 TCP 连接云端和 bytes 透传。
- `gateway-adapter` 负责 TCP 接入、原始数据留痕、最小分类。
- `nehuang-server` 负责业务数据、权限、绑定、点位配置、控制命令、Admin/Mini API。

服务端不负责：

- 给 MCU 分配频点和设备 ID。
- 解决硬件空中撞包本身。
- 重写 MCU 温控逻辑。
- 直接读 NTC 温度模块。

服务端必须负责：

- 接收并保留真实 raw 包。
- 按最终协议解析工作包、告警包、ACK。
- 把解析结果写入 MySQL。
- 给 Admin/Mini 提供稳定展示和操作接口。

## 4. 当前模型

主模型：

```text
cloud_gateways
  -> collector_nodes
  -> device_points
```

含义：

- `cloud_gateways`：云边/LoRa/TCP 通信接入设备。
- `collector_nodes`：采集执行器节点，唯一身份为 `gateway_id + frequency + device_external_id`。
- `device_points`：节点下的功能点/通道，例如温度、墒情、限位、继电器、水泵、风机。
- `device_realtime_maps`：既有旧字段兼容投影，不再作为主抽象。

用户和设备关系：

- Admin 负责绑定农户和功能点。
- Mini 农户只能查看和控制已绑定的 `point_id`。
- 当前没有实现后台手动创建硬件主数据，设备主数据仍依赖 Adapter 上报和后续 parser。

## 5. 硬件端最新口径

硬件端最新逻辑对云端的关键影响：

- MCU 本地替代旧 `jisuan.py`，自动温控逻辑下放到设备端。
- MCU 每 5 分钟缓存一次温度，4 小时上报 48 组。
- 工作包包含温度序列、继电器状态、DI 限位/故障状态、长度和 CRC。
- 单个云边集群最大按 100 台采集执行器规划，每台 2 秒专有时序，满配 200 秒周期。
- 告警包会重复上报 3 次，云端需要去重。
- 手动控制暂定 `F0` 开、`0F` 关、`00` 停。
- CRC 使用 Modbus CRC-16。

详细整理见：

- `docs/hardware_protocol_current.md`

## 6. 当前已实现和未实现

已实现：

- Go HTTP 服务端。
- Admin/Mini 基础 API。
- MySQL schema 和 migration runner。
- 注册/心跳/telemetry JSON envelope 入口。
- Go TCP `gateway-adapter`。
- raw frame journal。
- 本地 mock 发包工具。
- raw journal replay 工具。
- Mini 控制请求创建异步 `control_commands`。
- Adapter 内部 pending/sent/result 命令接口。

未实现：

- 真实工作包 parser。
- 告警 parser 和去重。
- 时隙调度。
- 真实下行 hex。
- ACK/清缓存解析。
- 告警通知。
- 腾讯云正式部署 `gateway-adapter` 的 systemd/deploy 流程。
- 微信小程序真机端到端验收。

完整现状见：

- `docs/current_product_status.md`

## 7. 推荐开发顺序

1. 把 `gateway-adapter` 独立部署到云端并开放 TCP 端口。
2. 让硬件网关连入并发送注册、心跳、工作包样例。
3. 用 raw journal 保存真实 bytes，用 replay 工具确认分类。
4. 基于真实样例补工作包/告警包 parser。
5. 打通一个工作包写入 `sensor_data` 和 realtime。
6. 打通一个告警包入库和去重。
7. 打通一个控制命令 pending -> sent -> ack。
8. 再补 Admin/Mini 的完整控制展示、告警通知和点位配置页面。

## 8. 新开 Codex 线程时的提示词

可以直接给新线程这句话：

```text
请先阅读 /Users/shore/Desktop/ShoreOS/services/nehuang-iot-server/docs/README.md，
然后继续涅凰智农 IoT 云平台开发。当前事实以 current_product_status.md、current_api.md、gateway_adapter.md、hardware_protocol_current.md、architecture_v1.md 和 schema/mysql 为准。
```
