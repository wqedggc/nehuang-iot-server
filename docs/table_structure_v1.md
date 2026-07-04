# 涅凰智农 V1 最终表结构文档

## 1. 表设计规范

本表结构严格执行以下规范：

- 所有雪花 ID 使用 `BIGINT UNSIGNED`。
- 所有时间字段使用 `TIMESTAMP(3)`。
- `created_at` 固定为 `TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)`。
- `updated_at` 固定为 `TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)`。
- 状态字段使用 `TINYINT UNSIGNED`。
- 外部 ID / 关键指纹使用 `VARCHAR(64)`。
- `store/platform` 类短枚举如后续出现使用 `VARCHAR(16)`。
- 原始快照使用 `TEXT`。
- 扩展信息使用 `VARCHAR(512)`。
- 禁止使用 `JSON` 字段类型。
- 所有表必须包含 `KEY idx_updated_at (updated_at)`。

## 2. 表清单

| 表名 | 说明 |
|---|---|
| `users` | 平台用户表，承载管理员与微信农户 |
| `cloud_gateways` | 云边采集数据汇总表，对应 E90 / FP 设备 |
| `collector_nodes` | 采集执行器节点表，对应 `gateway_id + frequency + device_external_id` |
| `device_points` | 采集执行器功能点表，对应节点下的温度、墒情、限位、继电器、水泵、风机等点位 |
| `device_realtime_maps` | 甲方旧实时状态字段兼容表，后续作为实时状态投影 |
| `bind_relations` | 农户与功能点绑定关系表 |
| `sensor_data` | 功能点时序曲线表 |
| `op_records` | 操作记录表 |
| `raw_device_events` | 原始设备事件表 |
| `control_commands` | 控制命令表 |
| `control_results` | 控制结果表 |
| `protocol_command_templates` | 协议命令模板表 |

## 2.1 当前核心抽象

V1 当前采用三层设备数据模型：

```text
cloud_gateways
  -> collector_nodes
  -> device_points
```

- `cloud_gateways`：云边/LoRa 通信接入单元，注册包中的 `FP_` 标识进入这里。
- `collector_nodes`：采集执行器节点，硬件稳定身份为 `gateway_id + frequency + device_external_id`。
- `device_points`：节点下的功能点/通道，例如温度、墒情、限位、继电器、水泵、风机。
- `device_realtime_maps`：保留甲方旧字段和验收语义，作为兼容投影表；新业务查询、绑定、控制优先使用 `point_id`。

## 3. 字段说明表
### 3.1 `users`

平台用户表

| 字段名 | 类型 | 允许为空 | 说明 |
|---|---|---|---|
| `user_id` | `BIGINT UNSIGNED NOT NULL` | 否 | 用户雪花ID |
| `user_type` | `TINYINT UNSIGNED NOT NULL` | 否 | 用户类型：1=后台管理员 2=微信农户 |
| `role` | `TINYINT UNSIGNED NOT NULL DEFAULT 3` | 否 | 角色：1=super_admin 2=admin 3=farmer |
| `username` | `VARCHAR(64) DEFAULT NULL` | 是 | 登录名或农户展示名 |
| `mobile` | `VARCHAR(64) DEFAULT NULL` | 是 | 手机号 |
| `wechat_openid` | `VARCHAR(64) DEFAULT NULL` | 是 | 微信openid |
| `wechat_unionid` | `VARCHAR(64) DEFAULT NULL` | 是 | 微信unionid |
| `password_hash` | `VARCHAR(512) DEFAULT NULL` | 是 | 后台账号密码哈希 |
| `display_name` | `VARCHAR(64) DEFAULT NULL` | 是 | 展示名称 |
| `avatar_url` | `VARCHAR(512) DEFAULT NULL` | 是 | 头像URL |
| `status` | `TINYINT UNSIGNED NOT NULL DEFAULT 1` | 否 | 状态：1=active 2=disabled 3=deleted |
| `last_login_at_ms` | `TIMESTAMP(3) NULL DEFAULT NULL` | 是 | 最后登录时间 |
| `created_at` | `TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)` | 否 | 创建时间 |
| `updated_at` | `TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)` | 否 | 更新时间 |

### 3.2 `cloud_gateways`

云边采集数据汇总表

| 字段名 | 类型 | 允许为空 | 说明 |
|---|---|---|---|
| `gateway_id` | `BIGINT UNSIGNED NOT NULL` | 否 | 云边设备雪花ID |
| `gateway_external_id` | `VARCHAR(64) NOT NULL` | 否 | 设备短ID，去除FP_前缀 |
| `gateway_name` | `VARCHAR(64) NOT NULL` | 否 | 设备名称，包含FP_前缀 |
| `ip` | `VARCHAR(64) NOT NULL DEFAULT ''` | 否 | 设备上线公网IP地址 |
| `port` | `INT NOT NULL DEFAULT 40000` | 否 | 设备接入端口号 |
| `gateway_sn` | `VARCHAR(64) NOT NULL DEFAULT ''` | 否 | 设备硬件序列号 |
| `online_status` | `TINYINT UNSIGNED NOT NULL DEFAULT 0` | 否 | 在线状态：0=离线 1=在线 |
| `source_type` | `TINYINT UNSIGNED NOT NULL DEFAULT 1` | 否 | 接入来源：1=tcp_gateway 2=adapter_api |
| `last_raw_event_id` | `BIGINT UNSIGNED DEFAULT NULL` | 是 | 最后一次原始事件ID |
| `first_seen_at_ms` | `TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)` | 否 | 首次入库时间 |
| `last_heartbeat_at_ms` | `TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)` | 否 | 最后一次心跳时间 |
| `created_at` | `TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)` | 否 | 创建时间 |
| `updated_at` | `TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)` | 否 | 更新时间 |

### 3.3 `collector_nodes`

采集执行器节点表

身份规则：同一云边设备下 `frequency + device_external_id` 必须唯一。该组合来自硬件写入的固定频点和设备 ID；`shed_no`、农户、名称、备注等只属于业务配置，不参与硬件身份。

| 字段名 | 类型 | 允许为空 | 说明 |
|---|---|---|---|
| `node_id` | `BIGINT UNSIGNED NOT NULL` | 否 | 采集执行器节点雪花ID |
| `gateway_id` | `BIGINT UNSIGNED NOT NULL` | 否 | 所属云边设备ID |
| `frequency` | `INT NOT NULL` | 否 | 通信频点 |
| `device_external_id` | `VARCHAR(64) NOT NULL` | 否 | 采集执行器设备ID |
| `node_name` | `VARCHAR(64) NOT NULL DEFAULT ''` | 否 | 节点业务名称 |
| `shed_no` | `VARCHAR(64) NOT NULL DEFAULT ''` | 否 | 棚号/农户自定义片区 |
| `base_name` | `VARCHAR(64) NOT NULL DEFAULT ''` | 否 | 基地名称 |
| `area_name` | `VARCHAR(64) NOT NULL DEFAULT ''` | 否 | 区域名称 |
| `collector_no` | `INT DEFAULT NULL` | 是 | 页面展示的采集节点序号 |
| `node_status` | `TINYINT UNSIGNED NOT NULL DEFAULT 1` | 否 | 节点状态：1=active 2=disabled 3=deleted |
| `online_status` | `TINYINT UNSIGNED NOT NULL DEFAULT 0` | 否 | 在线状态：0=离线 1=在线 |
| `last_report_at_ms` | `TIMESTAMP(3) NULL DEFAULT NULL` | 是 | 最后一次上报时间 |
| `last_raw_event_id` | `BIGINT UNSIGNED DEFAULT NULL` | 是 | 最后一次原始事件ID |
| `remark` | `VARCHAR(512) DEFAULT NULL` | 是 | 备注 |
| `extra` | `VARCHAR(512) DEFAULT NULL` | 是 | 扩展信息，禁止JSON类型 |
| `created_at` | `TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)` | 否 | 创建时间 |
| `updated_at` | `TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)` | 否 | 更新时间 |

### 3.4 `device_points`

采集执行器功能点表

说明：一个采集执行器节点下可以有多个功能点。用户和管理员页面真正展示、绑定、控制、看曲线的对象优先是 `device_points.point_id`。

| 字段名 | 类型 | 允许为空 | 说明 |
|---|---|---|---|
| `point_id` | `BIGINT UNSIGNED NOT NULL` | 否 | 功能点雪花ID |
| `node_id` | `BIGINT UNSIGNED NOT NULL` | 否 | 采集执行器节点ID |
| `gateway_id` | `BIGINT UNSIGNED NOT NULL` | 否 | 所属云边设备ID |
| `frequency` | `INT NOT NULL` | 否 | 通信频点 |
| `device_external_id` | `VARCHAR(64) NOT NULL` | 否 | 采集执行器设备ID |
| `function_type` | `TINYINT UNSIGNED NOT NULL` | 否 | 功能类型：1=温度 2=墒情 3=限位 4=继电器 5=水泵 6=风机 |
| `channel_no` | `INT NOT NULL DEFAULT 0` | 否 | 节点内通道号，未知时为0 |
| `point_name` | `VARCHAR(64) NOT NULL DEFAULT ''` | 否 | 功能点名称 |
| `unit` | `VARCHAR(16) NOT NULL DEFAULT ''` | 否 | 数值单位 |
| `best_min` | `FLOAT DEFAULT NULL` | 是 | 适宜数值下限 |
| `best_max` | `FLOAT DEFAULT NULL` | 是 | 适宜数值上限 |
| `real_value` | `FLOAT DEFAULT NULL` | 是 | 通用实时值 |
| `real_temp` | `FLOAT NOT NULL DEFAULT 0` | 否 | 甲方温度字段语义保留：温度实时值 |
| `value_status` | `TINYINT UNSIGNED NOT NULL DEFAULT 0` | 否 | 数值状态：0=unknown 1=normal 2=low 3=high 4=open 5=closed 6=offline |
| `run_mode` | `TINYINT UNSIGNED NOT NULL DEFAULT 1` | 否 | 运行模式：1=auto 2=manu |
| `controllable_status` | `TINYINT UNSIGNED NOT NULL DEFAULT 0` | 否 | 是否可控：0=否 1=是 |
| `rel1_status` | `TINYINT UNSIGNED NOT NULL DEFAULT 0` | 否 | 继电器1状态：0=关 1=开 |
| `rel2_status` | `TINYINT UNSIGNED NOT NULL DEFAULT 0` | 否 | 继电器2状态：0=关 1=开 |
| `operation_state` | `TINYINT UNSIGNED NOT NULL DEFAULT 0` | 否 | 操作状态：0=unknown 1=open 2=stop 3=close |
| `poll_interval` | `INT NOT NULL DEFAULT 300` | 否 | 轮询采集周期，单位秒 |
| `poll_cmd` | `TEXT` | 是 | 轮询下发十六进制采集指令 |
| `point_status` | `TINYINT UNSIGNED NOT NULL DEFAULT 1` | 否 | 功能点状态：1=active 2=disabled 3=deleted |
| `last_report_at_ms` | `TIMESTAMP(3) NULL DEFAULT NULL` | 是 | 最后一次工作数据上报时间 |
| `last_raw_event_id` | `BIGINT UNSIGNED DEFAULT NULL` | 是 | 最后一次原始事件ID |
| `remark` | `VARCHAR(512) DEFAULT NULL` | 是 | 备注 |
| `extra` | `VARCHAR(512) DEFAULT NULL` | 是 | 扩展信息，禁止JSON类型 |
| `created_at` | `TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)` | 否 | 创建时间 |
| `updated_at` | `TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)` | 否 | 更新时间 |

### 3.5 `device_realtime_maps`

智农采集执行器-云边设备映射对照表。当前保留为兼容投影表，承接甲方已有字段语义；新代码优先以 `device_points` 为实时状态主表。

身份规则：同一云边设备下 `frequency + device_external_id` 必须唯一。该组合来自硬件写入的固定频点和设备 ID；`shed_no`、农户、名称、备注等只属于业务配置，不参与硬件身份。

| 字段名 | 类型 | 允许为空 | 说明 |
|---|---|---|---|
| `realtime_id` | `BIGINT UNSIGNED NOT NULL` | 否 | 采集执行器实时状态雪花ID |
| `gateway_id` | `BIGINT UNSIGNED NOT NULL` | 否 | 云边设备ID |
| `node_id` | `BIGINT UNSIGNED DEFAULT NULL` | 是 | 采集执行器节点ID |
| `point_id` | `BIGINT UNSIGNED DEFAULT NULL` | 是 | 功能点ID |
| `shed_no` | `VARCHAR(64) NOT NULL DEFAULT ''` | 否 | 棚号/农户棚舍编号 |
| `collector_no` | `INT DEFAULT NULL` | 是 | 页面展示的设备序号 |
| `frequency` | `INT NOT NULL` | 否 | 智农采集执行器通信频点 |
| `device_external_id` | `VARCHAR(64) NOT NULL` | 否 | 智农采集执行器设备ID |
| `function_type` | `TINYINT UNSIGNED NOT NULL` | 否 | 功能类型：1=温度 2=墒情 3=限位 |
| `best_min` | `FLOAT DEFAULT NULL` | 是 | 适宜数值下限 |
| `best_max` | `FLOAT DEFAULT NULL` | 是 | 适宜数值上限 |
| `unit` | `VARCHAR(16) NOT NULL DEFAULT ''` | 否 | 数值单位 |
| `real_value` | `FLOAT DEFAULT NULL` | 是 | 通用实时值 |
| `real_temp` | `FLOAT NOT NULL DEFAULT 0` | 否 | 甲方温度字段语义保留：温度实时值 |
| `value_status` | `TINYINT UNSIGNED NOT NULL DEFAULT 0` | 否 | 数值状态：0=unknown 1=normal 2=low 3=high 4=open 5=closed |
| `run_mode` | `TINYINT UNSIGNED NOT NULL DEFAULT 1` | 否 | 运行模式：1=auto 2=manu |
| `rel1_status` | `TINYINT UNSIGNED NOT NULL DEFAULT 0` | 否 | 继电器1状态：0=关 1=开 |
| `rel2_status` | `TINYINT UNSIGNED NOT NULL DEFAULT 0` | 否 | 继电器2状态：0=关 1=开 |
| `operation_state` | `TINYINT UNSIGNED NOT NULL DEFAULT 0` | 否 | 操作状态：0=unknown 1=open 2=stop 3=close |
| `poll_interval` | `INT NOT NULL DEFAULT 300` | 否 | 轮询采集周期，单位秒 |
| `poll_cmd` | `TEXT` | 是 | 轮询下发十六进制采集指令 |
| `last_report_at_ms` | `TIMESTAMP(3) NULL DEFAULT NULL` | 是 | 最后一次工作数据上报时间 |
| `last_raw_event_id` | `BIGINT UNSIGNED DEFAULT NULL` | 是 | 最后一次原始事件ID |
| `extra` | `VARCHAR(512) DEFAULT NULL` | 是 | 扩展信息，禁止JSON类型 |
| `created_at` | `TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)` | 否 | 创建时间 |
| `updated_at` | `TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)` | 否 | 更新时间 |

### 3.6 `bind_relations`

农户关联表

| 字段名 | 类型 | 允许为空 | 说明 |
|---|---|---|---|
| `bind_id` | `BIGINT UNSIGNED NOT NULL` | 否 | 农户设备绑定雪花ID |
| `user_id` | `BIGINT UNSIGNED DEFAULT NULL` | 是 | 绑定农户用户ID |
| `bind_user_name` | `VARCHAR(64) NOT NULL` | 否 | 绑定农户名称 |
| `gateway_id` | `BIGINT UNSIGNED NOT NULL` | 否 | 云边设备ID |
| `realtime_id` | `BIGINT UNSIGNED DEFAULT NULL` | 是 | 采集执行器实时状态ID |
| `node_id` | `BIGINT UNSIGNED DEFAULT NULL` | 是 | 采集执行器节点ID |
| `point_id` | `BIGINT UNSIGNED DEFAULT NULL` | 是 | 功能点ID |
| `shed_no` | `VARCHAR(64) NOT NULL DEFAULT ''` | 否 | 棚号/农户棚舍编号 |
| `frequency` | `INT NOT NULL` | 否 | 智农采集执行器通信频点 |
| `device_external_id` | `VARCHAR(64) NOT NULL` | 否 | 智农采集执行器设备ID |
| `function_type` | `TINYINT UNSIGNED NOT NULL` | 否 | 功能类型：1=温度 2=墒情 3=限位 |
| `bind_status` | `TINYINT UNSIGNED NOT NULL DEFAULT 1` | 否 | 绑定状态：1=active 2=unbound 3=deleted |
| `bound_at_ms` | `TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)` | 否 | 绑定时间 |
| `unbound_at_ms` | `TIMESTAMP(3) NULL DEFAULT NULL` | 是 | 解绑时间 |
| `created_by` | `BIGINT UNSIGNED DEFAULT NULL` | 是 | 创建人用户ID |
| `extra` | `VARCHAR(512) DEFAULT NULL` | 是 | 扩展信息，禁止JSON类型 |
| `created_at` | `TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)` | 否 | 创建时间 |
| `updated_at` | `TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)` | 否 | 更新时间 |

### 3.7 `sensor_data`

温湿度智农采集执行器时序曲线表

| 字段名 | 类型 | 允许为空 | 说明 |
|---|---|---|---|
| `sensor_id` | `BIGINT UNSIGNED NOT NULL` | 否 | 传感器时序数据雪花ID |
| `gateway_id` | `BIGINT UNSIGNED NOT NULL` | 否 | 云边设备ID |
| `realtime_id` | `BIGINT UNSIGNED DEFAULT NULL` | 是 | 采集执行器实时状态ID |
| `node_id` | `BIGINT UNSIGNED DEFAULT NULL` | 是 | 采集执行器节点ID |
| `point_id` | `BIGINT UNSIGNED DEFAULT NULL` | 是 | 功能点ID |
| `frequency` | `INT NOT NULL` | 否 | 智农采集执行器通信频点 |
| `device_external_id` | `VARCHAR(64) NOT NULL` | 否 | 智农采集执行器设备ID |
| `sampled_at_ms` | `TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)` | 否 | 设备记录曲线上的时点信息 |
| `real_value` | `FLOAT DEFAULT NULL` | 是 | 通用采集值 |
| `real_temp` | `FLOAT NOT NULL DEFAULT 0` | 否 | 甲方温度字段语义保留：温度采集值 |
| `function_type` | `TINYINT UNSIGNED NOT NULL` | 否 | 功能类型：1=温度 2=墒情 3=限位 |
| `unit` | `VARCHAR(16) NOT NULL DEFAULT ''` | 否 | 单位 |
| `poll_interval` | `INT NOT NULL DEFAULT 300` | 否 | 轮询采集周期，单位秒 |
| `raw_event_id` | `BIGINT UNSIGNED DEFAULT NULL` | 是 | 原始事件ID |
| `extra` | `VARCHAR(512) DEFAULT NULL` | 是 | 扩展信息，禁止JSON类型 |
| `created_at` | `TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)` | 否 | 创建时间 |
| `updated_at` | `TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)` | 否 | 更新时间 |

### 3.8 `op_records`

操作记录表

| 字段名 | 类型 | 允许为空 | 说明 |
|---|---|---|---|
| `op_id` | `BIGINT UNSIGNED NOT NULL` | 否 | 操作记录雪花ID |
| `gateway_id` | `BIGINT UNSIGNED NOT NULL` | 否 | 云边设备ID |
| `realtime_id` | `BIGINT UNSIGNED DEFAULT NULL` | 是 | 采集执行器实时状态ID |
| `node_id` | `BIGINT UNSIGNED DEFAULT NULL` | 是 | 采集执行器节点ID |
| `point_id` | `BIGINT UNSIGNED DEFAULT NULL` | 是 | 功能点ID |
| `user_id` | `BIGINT UNSIGNED DEFAULT NULL` | 是 | 操作者用户ID |
| `bind_user_name` | `VARCHAR(64) NOT NULL` | 否 | 绑定农户名称 |
| `operator_role` | `TINYINT UNSIGNED NOT NULL DEFAULT 0` | 否 | 操作者角色：0=unknown 1=super_admin 2=admin 3=farmer |
| `frequency` | `INT NOT NULL` | 否 | 智农采集执行器通信频点 |
| `device_external_id` | `VARCHAR(64) NOT NULL` | 否 | 智农采集执行器设备ID |
| `run_mode` | `TINYINT UNSIGNED NOT NULL DEFAULT 1` | 否 | 运行模式：1=auto 2=manu |
| `rel1_status` | `TINYINT UNSIGNED NOT NULL DEFAULT 0` | 否 | 继电器1状态：0=关 1=开 |
| `rel2_status` | `TINYINT UNSIGNED NOT NULL DEFAULT 0` | 否 | 继电器2状态：0=关 1=开 |
| `op_type` | `TINYINT UNSIGNED NOT NULL DEFAULT 0` | 否 | 操作类型：0=unknown 1=bind 2=unbind 3=control 4=config |
| `op_content` | `VARCHAR(512) DEFAULT NULL` | 是 | 操作内容 |
| `command_id` | `BIGINT UNSIGNED DEFAULT NULL` | 是 | 关联控制命令ID |
| `operated_at_ms` | `TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)` | 否 | 设备被操作的时间 |
| `created_at` | `TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)` | 否 | 创建时间 |
| `updated_at` | `TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)` | 否 | 更新时间 |

### 3.9 `raw_device_events`

原始设备事件表

| 字段名 | 类型 | 允许为空 | 说明 |
|---|---|---|---|
| `raw_event_id` | `BIGINT UNSIGNED NOT NULL` | 否 | 原始设备事件雪花ID |
| `gateway_id` | `BIGINT UNSIGNED DEFAULT NULL` | 是 | 云边设备ID |
| `node_id` | `BIGINT UNSIGNED DEFAULT NULL` | 是 | 解析后的采集执行器节点ID |
| `point_id` | `BIGINT UNSIGNED DEFAULT NULL` | 是 | 解析后的功能点ID |
| `gateway_name` | `VARCHAR(64) DEFAULT NULL` | 是 | FP设备名 |
| `event_type` | `TINYINT UNSIGNED NOT NULL` | 否 | 事件类型：1=register 2=heartbeat 3=work 4=control_result 5=json_snapshot 6=alarm 7=unknown |
| `source_type` | `TINYINT UNSIGNED NOT NULL DEFAULT 1` | 否 | 事件来源：1=gateway 2=adapter 3=manual |
| `remote_ip` | `VARCHAR(64) DEFAULT NULL` | 是 | 远端IP |
| `remote_port` | `INT DEFAULT NULL` | 是 | 远端端口 |
| `payload_hex` | `TEXT` | 是 | 原始十六进制 |
| `payload_snapshot` | `TEXT` | 是 | 原始快照文本，禁止JSON类型 |
| `parse_status` | `TINYINT UNSIGNED NOT NULL DEFAULT 1` | 否 | 解析状态：1=pending 2=parsed 3=failed 4=ignored |
| `parse_error` | `VARCHAR(512) DEFAULT NULL` | 是 | 解析错误 |
| `received_at_ms` | `TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)` | 否 | 接收时间 |
| `created_at` | `TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)` | 否 | 创建时间 |
| `updated_at` | `TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)` | 否 | 更新时间 |

### 3.10 `control_commands`

控制命令表

| 字段名 | 类型 | 允许为空 | 说明 |
|---|---|---|---|
| `command_id` | `BIGINT UNSIGNED NOT NULL` | 否 | 控制命令雪花ID |
| `gateway_id` | `BIGINT UNSIGNED NOT NULL` | 否 | 云边设备ID |
| `realtime_id` | `BIGINT UNSIGNED DEFAULT NULL` | 是 | 采集执行器实时状态ID |
| `node_id` | `BIGINT UNSIGNED DEFAULT NULL` | 是 | 采集执行器节点ID |
| `point_id` | `BIGINT UNSIGNED DEFAULT NULL` | 是 | 功能点ID |
| `frequency` | `INT NOT NULL` | 否 | 智农采集执行器通信频点 |
| `device_external_id` | `VARCHAR(64) NOT NULL` | 否 | 智农采集执行器设备ID |
| `command_source` | `TINYINT UNSIGNED NOT NULL` | 否 | 命令来源：1=admin 2=mini 3=auto 4=system |
| `command_type` | `TINYINT UNSIGNED NOT NULL` | 否 | 命令类型：1=relay_open 2=relay_close 3=relay_stop 4=poll 5=read_status 6=custom_hex |
| `run_mode` | `TINYINT UNSIGNED NOT NULL DEFAULT 2` | 否 | 命令产生时运行模式：1=auto 2=manu |
| `rel1_target_status` | `TINYINT UNSIGNED DEFAULT NULL` | 是 | 目标继电器1状态：0=关 1=开 |
| `rel2_target_status` | `TINYINT UNSIGNED DEFAULT NULL` | 是 | 目标继电器2状态：0=关 1=开 |
| `command_hex` | `TEXT NOT NULL` | 否 | 实际下发十六进制命令 |
| `command_status` | `TINYINT UNSIGNED NOT NULL DEFAULT 1` | 否 | 命令状态：1=created 2=sent 3=ack 4=succeeded 5=failed 6=timeout |
| `requested_by` | `BIGINT UNSIGNED DEFAULT NULL` | 是 | 操作者用户ID |
| `requested_at_ms` | `TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)` | 否 | 请求时间 |
| `sent_at_ms` | `TIMESTAMP(3) NULL DEFAULT NULL` | 是 | 下发时间 |
| `finished_at_ms` | `TIMESTAMP(3) NULL DEFAULT NULL` | 是 | 完成时间 |
| `gateway_response` | `VARCHAR(512) DEFAULT NULL` | 是 | Gateway返回 |
| `error_message` | `VARCHAR(512) DEFAULT NULL` | 是 | 失败原因 |
| `extra` | `VARCHAR(512) DEFAULT NULL` | 是 | 扩展信息，禁止JSON类型 |
| `created_at` | `TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)` | 否 | 创建时间 |
| `updated_at` | `TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)` | 否 | 更新时间 |

### 3.11 `control_results`

控制结果表

| 字段名 | 类型 | 允许为空 | 说明 |
|---|---|---|---|
| `result_id` | `BIGINT UNSIGNED NOT NULL` | 否 | 控制结果雪花ID |
| `command_id` | `BIGINT UNSIGNED DEFAULT NULL` | 是 | 关联控制命令ID |
| `gateway_id` | `BIGINT UNSIGNED NOT NULL` | 否 | 云边设备ID |
| `realtime_id` | `BIGINT UNSIGNED DEFAULT NULL` | 是 | 采集执行器实时状态ID |
| `node_id` | `BIGINT UNSIGNED DEFAULT NULL` | 是 | 采集执行器节点ID |
| `point_id` | `BIGINT UNSIGNED DEFAULT NULL` | 是 | 功能点ID |
| `frequency` | `INT DEFAULT NULL` | 是 | 通信频点 |
| `device_external_id` | `VARCHAR(64) DEFAULT NULL` | 是 | 采集执行器设备ID |
| `result_status` | `TINYINT UNSIGNED NOT NULL` | 否 | 结果状态：1=ack 2=succeeded 3=failed 4=unknown |
| `result_hex` | `TEXT` | 是 | 原始回包十六进制 |
| `result_snapshot` | `TEXT` | 是 | 解析后结果文本，禁止JSON类型 |
| `raw_event_id` | `BIGINT UNSIGNED DEFAULT NULL` | 是 | 原始事件ID |
| `received_at_ms` | `TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)` | 否 | 接收时间 |
| `created_at` | `TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)` | 否 | 创建时间 |
| `updated_at` | `TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)` | 否 | 更新时间 |

### 3.12 `protocol_command_templates`

协议命令模板表

| 字段名 | 类型 | 允许为空 | 说明 |
|---|---|---|---|
| `template_id` | `BIGINT UNSIGNED NOT NULL` | 否 | 协议命令模板雪花ID |
| `function_type` | `TINYINT UNSIGNED NOT NULL` | 否 | 功能类型：1=温度 2=墒情 3=限位 4=relay |
| `command_type` | `TINYINT UNSIGNED NOT NULL` | 否 | 命令类型：1=poll 2=relay_open 3=relay_close 4=relay_stop 5=read_status 6=custom |
| `channel_no` | `INT DEFAULT NULL` | 是 | 通道号 |
| `command_hex` | `TEXT NOT NULL` | 否 | 十六进制命令 |
| `description` | `VARCHAR(512) DEFAULT NULL` | 是 | 说明 |
| `active_status` | `TINYINT UNSIGNED NOT NULL DEFAULT 1` | 否 | 启用状态：0=停用 1=启用 |
| `extra` | `VARCHAR(512) DEFAULT NULL` | 是 | 扩展信息，禁止JSON类型 |
| `created_at` | `TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)` | 否 | 创建时间 |
| `updated_at` | `TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)` | 否 | 更新时间 |

## 4. 写库 Cycle

### 4.1 Gateway 在线状态 Cycle

```text
Hardware gateway TCP
  -> Go gateway-adapter
  -> raw frame journal
  -> /api/v1/adapter/device-events(register/heartbeat)
  -> cloud_gateways upsert
```

写库步骤：

1. `gateway-adapter` 收到 TCP bytes 后先写 raw frame journal。
2. 识别注册或心跳后，构造 `/api/v1/adapter/device-events` envelope。
3. Go HTTP 服务写入 `raw_device_events`。
4. 按 `gateway_external_id` 查询或创建 `cloud_gateways`。
5. 更新 `online_status`、`ip`、`port`、`last_heartbeat_at_ms`、`last_raw_event_id`。
6. `updated_at` 由 MySQL 自动刷新。

### 4.2 工作包解析 Cycle

```text
Hardware work packet
  -> Go gateway-adapter raw frame journal
  -> raw_device_events(event_type=3 work, parse_status=1 pending)
  -> parser / replay
  -> collector_nodes upsert
  -> device_points update
  -> device_realtime_maps update optional
  -> sensor_data insert
  -> raw_device_events parse_status=2 parsed / 3 failed
```

写库步骤：

1. 原始工作包先写 raw frame journal，后续可同步写 `raw_device_events`，不允许直接丢弃。
2. parser 解析出 `gateway_id + frequency + device_external_id`。
3. 按 `gateway_id + frequency + device_external_id` 查询或创建 `collector_nodes`。
4. 按 `node_id + function_type + channel_no` 查询或创建 `device_points`。
5. 更新 `device_points.real_value/real_temp/value_status/last_report_at_ms`。
6. 如需要兼容旧页面或验收字段，同步更新 `device_realtime_maps`。
7. 插入 `sensor_data` 保存历史曲线，写入 `node_id` 和 `point_id`。
8. 解析失败只更新 `parse_status=3` 和 `parse_error`，原始数据保留。

### 4.3 手动控制 Cycle

```text
Admin/MiniProgram
  -> permission check
  -> control_commands(command_status=1 created)
  -> gateway-adapter pulls pending command
  -> hardware send
  -> control_commands(command_status=2 sent)
  -> control_results / final status
```

写库步骤：

1. 先按 `user_id + point_id` 校验绑定权限；旧数据可兼容 `gateway_id + frequency + device_external_id`。
2. 创建 `control_commands`，记录 `command_hex` 和目标继电器状态。
3. 用户请求立即返回 `command_id + pending`，不等待 MCU 最终 ACK。
4. Adapter 拉取 pending command 后，根据后续时隙策略或设备取命令策略下发。
5. 下发成功只代表已发送，更新 `command_status=2`。
6. 收到 ACK/结果后写 `control_results` 并更新最终状态。
7. 手动模式下根据业务要求写入 `op_records`。

### 4.4 控制结果 Cycle

```text
Device result packet
  -> raw_device_events(event_type=4 control_result)
  -> control_results insert
  -> control_commands status update
  -> device_points update
  -> device_realtime_maps update optional
```

写库步骤：

1. 控制回包先写 `raw_device_events`。
2. 解析结果写入 `control_results`。
3. 根据结果更新 `control_commands.command_status`。
4. 如果回包包含继电器实际状态，同步更新 `device_points.rel1_status/rel2_status/operation_state`。
5. 如需要兼容旧页面或验收字段，同步更新 `device_realtime_maps`。

### 4.5 用户绑定 Cycle

```text
Admin/MiniProgram settings
  -> users
  -> collector_nodes
  -> device_points
  -> bind_relations
  -> device_realtime_maps optional
```

写库步骤：

1. 微信登录或后台创建用户，写入 `users`。
2. 管理员或用户选择棚号、频点、设备 ID 后，写入或更新 `collector_nodes`。
3. 选择功能和通道后，写入或更新 `device_points`。
4. 绑定关系写入 `bind_relations.point_id`。
5. 解绑时不物理删除，更新 `bind_status=2` 和 `unbound_at_ms`。

## 5. 正式 DDL

最终建表脚本：

- `schema/mysql/001_nehuang_v1.sql`
- `schema/mysql/002_device_node_point_model.sql`

后续开发必须以这两个脚本为准；如果表结构变化，先改 DDL，再重新生成本文档。
