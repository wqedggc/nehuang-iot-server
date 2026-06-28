-- 涅凰智农 IoT 云平台 V1.0 设备节点/功能点抽象修正
-- 依赖：001_nehuang_v1.sql 已执行。
-- 目的：
-- 1. 将“采集执行器节点”和“节点下功能点/通道”拆开。
-- 2. 服务端完整硬件身份为：gateway_id + frequency + device_external_id。
-- 3. C 端和 B 端实际展示、绑定、控制、曲线查询优先面向 point_id。

USE nehuang_db;

CREATE TABLE IF NOT EXISTS collector_nodes (
  node_id BIGINT UNSIGNED NOT NULL COMMENT '采集执行器节点雪花ID',
  gateway_id BIGINT UNSIGNED NOT NULL COMMENT '所属云边设备ID',
  frequency INT NOT NULL COMMENT '通信频点',
  device_external_id VARCHAR(64) NOT NULL COMMENT '采集执行器设备ID',
  node_name VARCHAR(64) NOT NULL DEFAULT '' COMMENT '节点业务名称',
  shed_no VARCHAR(64) NOT NULL DEFAULT '' COMMENT '棚号/农户自定义片区',
  base_name VARCHAR(64) NOT NULL DEFAULT '' COMMENT '基地名称',
  area_name VARCHAR(64) NOT NULL DEFAULT '' COMMENT '区域名称',
  collector_no INT DEFAULT NULL COMMENT '页面展示的采集节点序号',
  node_status TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '节点状态：1=active 2=disabled 3=deleted',
  online_status TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '在线状态：0=离线 1=在线',
  last_report_at_ms TIMESTAMP(3) NULL DEFAULT NULL COMMENT '最后一次上报时间',
  last_raw_event_id BIGINT UNSIGNED DEFAULT NULL COMMENT '最后一次原始事件ID',
  remark VARCHAR(512) DEFAULT NULL COMMENT '备注',
  extra VARCHAR(512) DEFAULT NULL COMMENT '扩展信息，禁止JSON类型',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (node_id),
  UNIQUE KEY uk_collector_gateway_frequency_device (gateway_id, frequency, device_external_id),
  KEY idx_collector_gateway (gateway_id),
  KEY idx_collector_shed (shed_no),
  KEY idx_collector_online (online_status, last_report_at_ms),
  KEY idx_updated_at (updated_at),
  CONSTRAINT fk_collector_gateway FOREIGN KEY (gateway_id) REFERENCES cloud_gateways(gateway_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='采集执行器节点表';

CREATE TABLE IF NOT EXISTS device_points (
  point_id BIGINT UNSIGNED NOT NULL COMMENT '功能点雪花ID',
  node_id BIGINT UNSIGNED NOT NULL COMMENT '采集执行器节点ID',
  gateway_id BIGINT UNSIGNED NOT NULL COMMENT '所属云边设备ID',
  frequency INT NOT NULL COMMENT '通信频点',
  device_external_id VARCHAR(64) NOT NULL COMMENT '采集执行器设备ID',
  function_type TINYINT UNSIGNED NOT NULL COMMENT '功能类型：1=温度 2=墒情 3=限位 4=继电器 5=水泵 6=风机',
  channel_no INT NOT NULL DEFAULT 0 COMMENT '节点内通道号，未知时为0',
  point_name VARCHAR(64) NOT NULL DEFAULT '' COMMENT '功能点名称',
  unit VARCHAR(16) NOT NULL DEFAULT '' COMMENT '数值单位',
  best_min FLOAT DEFAULT NULL COMMENT '适宜数值下限',
  best_max FLOAT DEFAULT NULL COMMENT '适宜数值上限',
  real_value FLOAT DEFAULT NULL COMMENT '通用实时值',
  real_temp FLOAT NOT NULL DEFAULT 0 COMMENT '甲方温度字段语义保留：温度实时值',
  value_status TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '数值状态：0=unknown 1=normal 2=low 3=high 4=open 5=closed 6=offline',
  run_mode TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '运行模式：1=auto 2=manu',
  controllable_status TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '是否可控：0=否 1=是',
  rel1_status TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '继电器1状态：0=关 1=开',
  rel2_status TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '继电器2状态：0=关 1=开',
  operation_state TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '操作状态：0=unknown 1=open 2=stop 3=close',
  poll_interval INT NOT NULL DEFAULT 300 COMMENT '轮询采集周期，单位秒',
  poll_cmd TEXT COMMENT '轮询下发十六进制采集指令',
  point_status TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '功能点状态：1=active 2=disabled 3=deleted',
  last_report_at_ms TIMESTAMP(3) NULL DEFAULT NULL COMMENT '最后一次工作数据上报时间',
  last_raw_event_id BIGINT UNSIGNED DEFAULT NULL COMMENT '最后一次原始事件ID',
  remark VARCHAR(512) DEFAULT NULL COMMENT '备注',
  extra VARCHAR(512) DEFAULT NULL COMMENT '扩展信息，禁止JSON类型',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (point_id),
  UNIQUE KEY uk_point_node_function_channel (node_id, function_type, channel_no),
  KEY idx_point_gateway_device (gateway_id, frequency, device_external_id),
  KEY idx_point_function (function_type, point_status),
  KEY idx_updated_at (updated_at),
  CONSTRAINT fk_point_node FOREIGN KEY (node_id) REFERENCES collector_nodes(node_id),
  CONSTRAINT fk_point_gateway FOREIGN KEY (gateway_id) REFERENCES cloud_gateways(gateway_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='采集执行器功能点表';

ALTER TABLE device_realtime_maps
  ADD COLUMN node_id BIGINT UNSIGNED DEFAULT NULL COMMENT '采集执行器节点ID' AFTER gateway_id,
  ADD COLUMN point_id BIGINT UNSIGNED DEFAULT NULL COMMENT '功能点ID' AFTER node_id,
  ADD KEY idx_realtime_node (node_id),
  ADD KEY idx_realtime_point (point_id),
  ADD CONSTRAINT fk_realtime_node FOREIGN KEY (node_id) REFERENCES collector_nodes(node_id),
  ADD CONSTRAINT fk_realtime_point FOREIGN KEY (point_id) REFERENCES device_points(point_id);

ALTER TABLE bind_relations
  ADD COLUMN node_id BIGINT UNSIGNED DEFAULT NULL COMMENT '采集执行器节点ID' AFTER realtime_id,
  ADD COLUMN point_id BIGINT UNSIGNED DEFAULT NULL COMMENT '功能点ID' AFTER node_id,
  DROP INDEX uk_bind_user_gateway_device_status,
  ADD UNIQUE KEY uk_bind_user_point_status (user_id, point_id, bind_status),
  ADD KEY idx_bind_user_gateway_device_status (user_id, gateway_id, frequency, device_external_id, bind_status),
  ADD KEY idx_bind_node (node_id),
  ADD KEY idx_bind_point (point_id, bind_status),
  ADD CONSTRAINT fk_bind_node FOREIGN KEY (node_id) REFERENCES collector_nodes(node_id),
  ADD CONSTRAINT fk_bind_point FOREIGN KEY (point_id) REFERENCES device_points(point_id);

ALTER TABLE sensor_data
  ADD COLUMN node_id BIGINT UNSIGNED DEFAULT NULL COMMENT '采集执行器节点ID' AFTER realtime_id,
  ADD COLUMN point_id BIGINT UNSIGNED DEFAULT NULL COMMENT '功能点ID' AFTER node_id,
  ADD KEY idx_sensor_node_sampled (node_id, sampled_at_ms),
  ADD KEY idx_sensor_point_sampled (point_id, sampled_at_ms),
  ADD CONSTRAINT fk_sensor_node FOREIGN KEY (node_id) REFERENCES collector_nodes(node_id),
  ADD CONSTRAINT fk_sensor_point FOREIGN KEY (point_id) REFERENCES device_points(point_id);

ALTER TABLE op_records
  ADD COLUMN node_id BIGINT UNSIGNED DEFAULT NULL COMMENT '采集执行器节点ID' AFTER realtime_id,
  ADD COLUMN point_id BIGINT UNSIGNED DEFAULT NULL COMMENT '功能点ID' AFTER node_id,
  ADD KEY idx_op_node_operated (node_id, operated_at_ms),
  ADD KEY idx_op_point_operated (point_id, operated_at_ms),
  ADD CONSTRAINT fk_op_node FOREIGN KEY (node_id) REFERENCES collector_nodes(node_id),
  ADD CONSTRAINT fk_op_point FOREIGN KEY (point_id) REFERENCES device_points(point_id);

ALTER TABLE raw_device_events
  ADD COLUMN node_id BIGINT UNSIGNED DEFAULT NULL COMMENT '解析后的采集执行器节点ID' AFTER gateway_id,
  ADD COLUMN point_id BIGINT UNSIGNED DEFAULT NULL COMMENT '解析后的功能点ID' AFTER node_id,
  ADD KEY idx_raw_node_received (node_id, received_at_ms),
  ADD KEY idx_raw_point_received (point_id, received_at_ms),
  ADD CONSTRAINT fk_raw_node FOREIGN KEY (node_id) REFERENCES collector_nodes(node_id),
  ADD CONSTRAINT fk_raw_point FOREIGN KEY (point_id) REFERENCES device_points(point_id);

ALTER TABLE control_commands
  ADD COLUMN node_id BIGINT UNSIGNED DEFAULT NULL COMMENT '采集执行器节点ID' AFTER realtime_id,
  ADD COLUMN point_id BIGINT UNSIGNED DEFAULT NULL COMMENT '功能点ID' AFTER node_id,
  ADD KEY idx_command_node_status (node_id, command_status, requested_at_ms),
  ADD KEY idx_command_point_status (point_id, command_status, requested_at_ms),
  ADD CONSTRAINT fk_command_node FOREIGN KEY (node_id) REFERENCES collector_nodes(node_id),
  ADD CONSTRAINT fk_command_point FOREIGN KEY (point_id) REFERENCES device_points(point_id);

ALTER TABLE control_results
  ADD COLUMN node_id BIGINT UNSIGNED DEFAULT NULL COMMENT '采集执行器节点ID' AFTER realtime_id,
  ADD COLUMN point_id BIGINT UNSIGNED DEFAULT NULL COMMENT '功能点ID' AFTER node_id,
  ADD KEY idx_result_node_received (node_id, received_at_ms),
  ADD KEY idx_result_point_received (point_id, received_at_ms),
  ADD CONSTRAINT fk_result_node FOREIGN KEY (node_id) REFERENCES collector_nodes(node_id),
  ADD CONSTRAINT fk_result_point FOREIGN KEY (point_id) REFERENCES device_points(point_id);
