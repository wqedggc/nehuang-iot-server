# 涅凰智农项目上下文入口

## 1. 项目定位

涅凰智农 V1.0 是农业自动化 IoT 云平台。当前目标不是重写硬件、E90 固件或现场协议，而是在已有设备通信能力基础上建设新的云端业务平台。

一期范围：

- 数据库
- REST API
- 管理后台接口
- 微信小程序接口
- 用户管理
- 设备管理
- `devices.json` / 设备数据解析入库
- 控制中心
- 权限与日志

一期不做：

- 硬件开发
- TCP Gateway 重写
- E90 固件修改
- 485 协议修改
- OTA
- AI 分析
- 报表中心
- 告警中心
- 多租户

## 2. 当前资料路径

甲方/现状资料：

- `/Users/shore/Downloads/涅凰智农IoT平台系统调研报告.md`
- `/Users/shore/Downloads/nehuang_db.sql`
- `/Users/shore/Downloads/nehuang-mini_b5LkK.tar.gz`
- `/Users/shore/Library/Mobile Documents/com~apple~CloudDocs/lot--plat/涅凰智农极简系统构架.pptx`
- `/Users/shore/Library/Mobile Documents/com~apple~CloudDocs/lot--plat/数据库字段含义汇总表.xlsx`
- `/Users/shore/Library/Mobile Documents/com~apple~CloudDocs/lot--plat/小程序网页端样式、逻辑图(1).pptx`

本地已解包旧代码：

- `/Users/shore/Knowledge/nehuang-mini-source/nehuang-mini`

新服务端上下文和工程落点：

- `/Users/shore/Knowledge/nehuang-iot-server`

## 3. 旧代码事实

较可信的可运行链路：

```text
003rukou.py
  -> tcp_gateway.py
  -> 01app_core/data_cache_huancun/devices.json
  -> 01app_core/routes/*
```

`tcp_gateway.py` 当前实际能力：

- 监听 TCP `8081`。
- 监听命令下发 `8083`，接口为 `/send?did=&cmd=`。
- 识别包含 `FP` 的注册包。
- 维护设备 socket 连接和 35 秒心跳离线判断。
- 写入 `01app_core/data_cache_huancun/devices.json`。
- 尝试更新 `cloud_gateway`。

当前没有完成：

- 工作包解析。
- 传感器数据入库。
- `last_data` 更新。
- 控制命令状态机。
- 控制结果回传追踪。
- 微信用户登录。
- 正式管理员账号体系。

关键判断：当前“设备到服务端”只打通了在线状态，不是完整采集数据链路。

## 4. 甲方必须保留的核心模型

甲方 Excel 明确要求保留 5 张核心表及字段语义：

- `cloud_gateway`：云边 E90 / FP 设备在线状态。
- `bind_relation`：农户、云边设备、棚号、频点、采集执行器 ID、功能的绑定关系。
- `device_realtime_map`：采集执行器实时状态、阈值、模式、继电器、轮询命令。
- `sensor_data`：历史采集曲线。
- `op_record`：手动操作记录。

硬件身份已确认：采集执行器的稳定身份为“同一云边/LoRa 通信接入单元下的频点 + 设备 ID”。频点和设备 ID 由硬件侧写入设备，正常使用中不随农户、棚号、安装位置或业务配置变化而变化。服务端完整定位使用 `cloud_gateway + frequency + device_external_id`；农户、棚号、基地、区域、名称、备注和绑定关系都属于服务端业务数据。

当前服务端抽象已经拆成三层：`cloud_gateways -> collector_nodes -> device_points`。其中 `collector_nodes` 对应采集执行器节点，唯一身份为 `gateway_id + frequency + device_external_id`；`device_points` 对应节点下的温度、墒情、限位、继电器、水泵、风机等功能点。后续 C 端/B 端的一行设备展示、绑定、曲线和控制优先围绕 `point_id` 设计；`device_realtime_maps` 保留为甲方旧字段兼容投影。

硬件容量补充：单个云边/LoRa 通信接入单元实操一般使用 4 个频点，少数型号最多 6 个频点；采集点建议控制在 100 个以下，超过时硬件施工侧应拆分多组云边设备。V1 服务端不做强制拦截，仅作为后台统计和容量提示依据。

正式 V1 数据模型见：

- `schema/mysql/001_nehuang_v1.sql`
- `schema/mysql/002_device_node_point_model.sql`
- `docs/table_structure_v1.md`
- `docs/data_model_v1.md`

注意：旧 Python 里混用了 `farmer/devname` 和 `bind_usr_name/devid` 两套字段。V1 以甲方 Excel 和用户表设计规范为准，最终 DDL 已统一为 snake_case、雪花 ID、`TIMESTAMP(3)`、`TINYINT UNSIGNED` 状态字段、无 JSON 类型。业务语义保留，物理表名和字段名以 `schema/mysql/001_nehuang_v1.sql` 与 `schema/mysql/002_device_node_point_model.sql` 为准。

## 5. 前端视角约束

小程序/网页需要两个主要视角。

用户及设置页：

- 用户名
- 我的设备列表
- 序号
- 棚号
- 频点
- ID
- 功能
- 状态
- 操作

操作页：

- 棚号选择
- 设备序号
- 功能
- 最佳范围
- 单位
- 实际值
- 模式
- 操作

温度设备操作是三档：开、停、关。墒情设备操作是两档：开、关。限位设备以状态读取和展示为主。

## 6. V1 架构原则

必须遵守：

- 不修改现场硬件协议。
- 不修改 E90 固件。
- 不让业务层依赖 TCP。
- 不让页面直接读取 JSON。
- `devices.json` 只作为 Gateway 输出缓存。
- MySQL 是业务唯一事实来源。
- 所有客户端通过 REST API 获取数据。
- 协议解析放在 Device Adapter，不进入业务层。
- 管理后台优先于微信小程序。
- 一期不引入额外大功能。

架构说明见：

- `docs/architecture_v1.md`
- `docs/data_model_v1.md` 的第 7-9 节固化了服务端架构、上线方案和对外沟通口径。

## 7. 推荐开发顺序

1. 初始化 Go 服务端目录和基础配置。
2. 接入 MySQL，按顺序执行 `schema/mysql/001_nehuang_v1.sql` 和 `schema/mysql/002_device_node_point_model.sql`。
3. 实现 `devices.json -> cloud_gateways/raw_device_events` 同步 Adapter。
4. 实现 Admin API：Gateway 列表、采集器列表、绑定关系、控制命令。
5. 实现 MiniProgram API：微信登录、我的设备、实时数据、历史数据、控制设备。
6. 实现 Control Service：命令落库、调用 Gateway、状态更新、超时扫描。
7. 等工作包协议明确后，实现工作包解析并写入 `collector_nodes/device_points/sensor_data`，必要时同步 `device_realtime_maps` 兼容投影。

## 8. 新开 Codex 线程时的提示词

可以直接给新线程这句话：

```text
请先阅读 /Users/shore/Knowledge/nehuang-iot-server/docs/project_context.md，
然后继续涅凰智农 IoT 云平台 V1.0 的 Go 服务端开发。
最终表结构以 schema/mysql/001_nehuang_v1.sql、schema/mysql/002_device_node_point_model.sql 和 docs/table_structure_v1.md 为准。
```
