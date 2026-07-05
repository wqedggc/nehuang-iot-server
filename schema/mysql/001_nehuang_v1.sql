-- 涅凰智农 IoT 云平台 V1.0 最终建表 DDL
-- 规范：
-- 1. 主键/关联雪花 ID 使用 BIGINT UNSIGNED。
-- 2. 所有时间字段使用 TIMESTAMP(3)，命名只能为 created_at / updated_at / xxx_at_ms。
-- 3. 状态字段使用 TINYINT UNSIGNED。
-- 4. 禁止 JSON 字段类型，原始快照使用 TEXT，扩展信息使用 VARCHAR(512)。
-- 5. 所有表必须包含 idx_updated_at(updated_at)。

CREATE DATABASE IF NOT EXISTS nehuang_db
  DEFAULT CHARACTER SET utf8mb4
  DEFAULT COLLATE utf8mb4_unicode_ci;

USE nehuang_db;

CREATE TABLE IF NOT EXISTS users (
  user_id BIGINT UNSIGNED NOT NULL COMMENT '用户雪花ID',
  user_type TINYINT UNSIGNED NOT NULL COMMENT '用户类型：1=后台管理员 2=微信农户',
  role TINYINT UNSIGNED NOT NULL DEFAULT 3 COMMENT '角色：1=super_admin 2=admin 3=farmer',
  username VARCHAR(64) DEFAULT NULL COMMENT '登录名或农户展示名',
  mobile VARCHAR(64) DEFAULT NULL COMMENT '手机号',
  wechat_openid VARCHAR(64) DEFAULT NULL COMMENT '微信openid',
  wechat_unionid VARCHAR(64) DEFAULT NULL COMMENT '微信unionid',
  password_hash VARCHAR(512) DEFAULT NULL COMMENT '后台账号密码哈希',
  display_name VARCHAR(64) DEFAULT NULL COMMENT '展示名称',
  avatar_url VARCHAR(512) DEFAULT NULL COMMENT '头像URL',
  status TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '状态：1=active 2=disabled 3=deleted',
  last_login_at_ms TIMESTAMP(3) NULL DEFAULT NULL COMMENT '最后登录时间',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (user_id),
  UNIQUE KEY uk_users_wechat_openid (wechat_openid),
  UNIQUE KEY uk_users_username (username),
  KEY idx_updated_at (updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='平台用户表';

CREATE TABLE IF NOT EXISTS cloud_gateways (
  gateway_id BIGINT UNSIGNED NOT NULL COMMENT '云边设备雪花ID',
  gateway_external_id VARCHAR(64) NOT NULL COMMENT '设备短ID，去除FP_前缀',
  gateway_name VARCHAR(64) NOT NULL COMMENT '设备名称，包含FP_前缀',
  access_mode VARCHAR(16) NOT NULL DEFAULT 'shared' COMMENT '使用模式：shared=共享 exclusive=独享',
  ip VARCHAR(64) NOT NULL DEFAULT '' COMMENT '设备上线公网IP地址',
  port INT NOT NULL DEFAULT 40000 COMMENT '设备接入端口号',
  gateway_sn VARCHAR(64) NOT NULL DEFAULT '' COMMENT '设备硬件序列号',
  online_status TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '在线状态：0=离线 1=在线',
  source_type TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '接入来源：1=tcp_gateway 2=adapter_api',
  last_raw_event_id BIGINT UNSIGNED DEFAULT NULL COMMENT '最后一次原始事件ID',
  first_seen_at_ms TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '首次入库时间',
  last_heartbeat_at_ms TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '最后一次心跳时间',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (gateway_id),
  UNIQUE KEY uk_cloud_gateways_name (gateway_name),
  UNIQUE KEY uk_cloud_gateways_external_id (gateway_external_id),
  KEY idx_cloud_gateways_access_mode (access_mode),
  KEY idx_cloud_gateways_online (online_status, last_heartbeat_at_ms),
  KEY idx_updated_at (updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='云边采集数据汇总表';

CREATE TABLE IF NOT EXISTS device_realtime_maps (
  realtime_id BIGINT UNSIGNED NOT NULL COMMENT '采集执行器实时状态雪花ID',
  gateway_id BIGINT UNSIGNED NOT NULL COMMENT '云边设备ID',
  shed_no VARCHAR(64) NOT NULL DEFAULT '' COMMENT '棚号/农户棚舍编号',
  collector_no INT DEFAULT NULL COMMENT '页面展示的设备序号',
  frequency INT NOT NULL COMMENT '智农采集执行器通信频点',
  device_external_id VARCHAR(64) NOT NULL COMMENT '智农采集执行器设备ID',
  function_type TINYINT UNSIGNED NOT NULL COMMENT '功能类型：1=温度 2=墒情 3=限位',
  channel_no INT NOT NULL DEFAULT 0 COMMENT '节点内通道号，未知时为0',
  best_min FLOAT DEFAULT NULL COMMENT '适宜数值下限',
  best_max FLOAT DEFAULT NULL COMMENT '适宜数值上限',
  unit VARCHAR(16) NOT NULL DEFAULT '' COMMENT '数值单位',
  real_value FLOAT DEFAULT NULL COMMENT '通用实时值',
  real_temp FLOAT NOT NULL DEFAULT 0 COMMENT '既有温度字段语义保留：温度实时值',
  value_status TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '数值状态：0=unknown 1=normal 2=low 3=high 4=open 5=closed',
  run_mode TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '运行模式：1=auto 2=manu',
  rel1_status TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '继电器1状态：0=关 1=开',
  rel2_status TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '继电器2状态：0=关 1=开',
  operation_state TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '操作状态：0=unknown 1=open 2=stop 3=close',
  poll_interval INT NOT NULL DEFAULT 300 COMMENT '轮询采集周期，单位秒',
  poll_cmd TEXT COMMENT '轮询下发十六进制采集指令',
  last_report_at_ms TIMESTAMP(3) NULL DEFAULT NULL COMMENT '最后一次工作数据上报时间',
  last_raw_event_id BIGINT UNSIGNED DEFAULT NULL COMMENT '最后一次原始事件ID',
  extra VARCHAR(512) DEFAULT NULL COMMENT '扩展信息，禁止JSON类型',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (realtime_id),
  UNIQUE KEY uk_realtime_gateway_freq_device_func_channel (gateway_id, frequency, device_external_id, function_type, channel_no),
  KEY idx_realtime_function_type (function_type),
  KEY idx_updated_at (updated_at),
  CONSTRAINT fk_realtime_gateway FOREIGN KEY (gateway_id) REFERENCES cloud_gateways(gateway_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='智农采集执行器-云边设备映射对照表';

CREATE TABLE IF NOT EXISTS bind_relations (
  bind_id BIGINT UNSIGNED NOT NULL COMMENT '农户设备绑定雪花ID',
  user_id BIGINT UNSIGNED DEFAULT NULL COMMENT '绑定农户用户ID',
  bind_user_name VARCHAR(64) NOT NULL COMMENT '绑定农户名称',
  gateway_id BIGINT UNSIGNED NOT NULL COMMENT '云边设备ID',
  realtime_id BIGINT UNSIGNED DEFAULT NULL COMMENT '采集执行器实时状态ID',
  shed_no VARCHAR(64) NOT NULL DEFAULT '' COMMENT '棚号/农户棚舍编号',
  frequency INT NOT NULL COMMENT '智农采集执行器通信频点',
  device_external_id VARCHAR(64) NOT NULL COMMENT '智农采集执行器设备ID',
  function_type TINYINT UNSIGNED NOT NULL COMMENT '功能类型：1=温度 2=墒情 3=限位',
  bind_status TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '绑定状态：1=active 2=unbound 3=deleted',
  bound_at_ms TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '绑定时间',
  unbound_at_ms TIMESTAMP(3) NULL DEFAULT NULL COMMENT '解绑时间',
  created_by BIGINT UNSIGNED DEFAULT NULL COMMENT '创建人用户ID',
  extra VARCHAR(512) DEFAULT NULL COMMENT '扩展信息，禁止JSON类型',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (bind_id),
  UNIQUE KEY uk_bind_user_gateway_device_status (user_id, gateway_id, frequency, device_external_id, bind_status),
  KEY idx_bind_gateway_device (gateway_id, frequency, device_external_id),
  KEY idx_bind_user_name (bind_user_name),
  KEY idx_updated_at (updated_at),
  CONSTRAINT fk_bind_user FOREIGN KEY (user_id) REFERENCES users(user_id),
  CONSTRAINT fk_bind_gateway FOREIGN KEY (gateway_id) REFERENCES cloud_gateways(gateway_id),
  CONSTRAINT fk_bind_realtime FOREIGN KEY (realtime_id) REFERENCES device_realtime_maps(realtime_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='农户关联表';

CREATE TABLE IF NOT EXISTS sensor_data (
  sensor_id BIGINT UNSIGNED NOT NULL COMMENT '传感器时序数据雪花ID',
  gateway_id BIGINT UNSIGNED NOT NULL COMMENT '云边设备ID',
  realtime_id BIGINT UNSIGNED DEFAULT NULL COMMENT '采集执行器实时状态ID',
  frequency INT NOT NULL COMMENT '智农采集执行器通信频点',
  device_external_id VARCHAR(64) NOT NULL COMMENT '智农采集执行器设备ID',
  sampled_at_ms TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '设备记录曲线上的时点信息',
  real_value FLOAT DEFAULT NULL COMMENT '通用采集值',
  real_temp FLOAT NOT NULL DEFAULT 0 COMMENT '既有温度字段语义保留：温度采集值',
  function_type TINYINT UNSIGNED NOT NULL COMMENT '功能类型：1=温度 2=墒情 3=限位',
  unit VARCHAR(16) NOT NULL DEFAULT '' COMMENT '单位',
  poll_interval INT NOT NULL DEFAULT 300 COMMENT '轮询采集周期，单位秒',
  raw_event_id BIGINT UNSIGNED DEFAULT NULL COMMENT '原始事件ID',
  extra VARCHAR(512) DEFAULT NULL COMMENT '扩展信息，禁止JSON类型',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (sensor_id),
  KEY idx_sensor_gateway_device_sampled (gateway_id, frequency, device_external_id, sampled_at_ms),
  KEY idx_sensor_realtime_sampled (realtime_id, sampled_at_ms),
  KEY idx_updated_at (updated_at),
  CONSTRAINT fk_sensor_gateway FOREIGN KEY (gateway_id) REFERENCES cloud_gateways(gateway_id),
  CONSTRAINT fk_sensor_realtime FOREIGN KEY (realtime_id) REFERENCES device_realtime_maps(realtime_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='温湿度智农采集执行器时序曲线表';

CREATE TABLE IF NOT EXISTS op_records (
  op_id BIGINT UNSIGNED NOT NULL COMMENT '操作记录雪花ID',
  gateway_id BIGINT UNSIGNED NOT NULL COMMENT '云边设备ID',
  realtime_id BIGINT UNSIGNED DEFAULT NULL COMMENT '采集执行器实时状态ID',
  user_id BIGINT UNSIGNED DEFAULT NULL COMMENT '操作者用户ID',
  bind_user_name VARCHAR(64) NOT NULL COMMENT '绑定农户名称',
  operator_role TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '操作者角色：0=unknown 1=super_admin 2=admin 3=farmer',
  frequency INT NOT NULL COMMENT '智农采集执行器通信频点',
  device_external_id VARCHAR(64) NOT NULL COMMENT '智农采集执行器设备ID',
  run_mode TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '运行模式：1=auto 2=manu',
  rel1_status TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '继电器1状态：0=关 1=开',
  rel2_status TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '继电器2状态：0=关 1=开',
  op_type TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '操作类型：0=unknown 1=bind 2=unbind 3=control 4=config',
  op_content VARCHAR(512) DEFAULT NULL COMMENT '操作内容',
  command_id BIGINT UNSIGNED DEFAULT NULL COMMENT '关联控制命令ID',
  operated_at_ms TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '设备被操作的时间',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (op_id),
  KEY idx_op_user_operated (user_id, operated_at_ms),
  KEY idx_op_gateway_device_operated (gateway_id, frequency, device_external_id, operated_at_ms),
  KEY idx_updated_at (updated_at),
  CONSTRAINT fk_op_gateway FOREIGN KEY (gateway_id) REFERENCES cloud_gateways(gateway_id),
  CONSTRAINT fk_op_realtime FOREIGN KEY (realtime_id) REFERENCES device_realtime_maps(realtime_id),
  CONSTRAINT fk_op_user FOREIGN KEY (user_id) REFERENCES users(user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='操作记录表';

CREATE TABLE IF NOT EXISTS raw_device_events (
  raw_event_id BIGINT UNSIGNED NOT NULL COMMENT '原始设备事件雪花ID',
  gateway_id BIGINT UNSIGNED DEFAULT NULL COMMENT '云边设备ID',
  gateway_name VARCHAR(64) DEFAULT NULL COMMENT 'FP设备名',
  event_type TINYINT UNSIGNED NOT NULL COMMENT '事件类型：1=register 2=heartbeat 3=work 4=control_result 5=json_snapshot 6=alarm 7=unknown',
  source_type TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '事件来源：1=gateway 2=adapter 3=manual',
  remote_ip VARCHAR(64) DEFAULT NULL COMMENT '远端IP',
  remote_port INT DEFAULT NULL COMMENT '远端端口',
  payload_hex TEXT COMMENT '原始十六进制',
  payload_snapshot TEXT COMMENT '原始快照文本，禁止JSON类型',
  parse_status TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '解析状态：1=pending 2=parsed 3=failed 4=ignored',
  parse_error VARCHAR(512) DEFAULT NULL COMMENT '解析错误',
  received_at_ms TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '接收时间',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (raw_event_id),
  KEY idx_raw_gateway_received (gateway_id, received_at_ms),
  KEY idx_raw_type_status (event_type, parse_status),
  KEY idx_updated_at (updated_at),
  CONSTRAINT fk_raw_gateway FOREIGN KEY (gateway_id) REFERENCES cloud_gateways(gateway_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='原始设备事件表';

CREATE TABLE IF NOT EXISTS control_commands (
  command_id BIGINT UNSIGNED NOT NULL COMMENT '控制命令雪花ID',
  gateway_id BIGINT UNSIGNED NOT NULL COMMENT '云边设备ID',
  realtime_id BIGINT UNSIGNED DEFAULT NULL COMMENT '采集执行器实时状态ID',
  frequency INT NOT NULL COMMENT '智农采集执行器通信频点',
  device_external_id VARCHAR(64) NOT NULL COMMENT '智农采集执行器设备ID',
  command_source TINYINT UNSIGNED NOT NULL COMMENT '命令来源：1=admin 2=mini 3=auto 4=system',
  command_type TINYINT UNSIGNED NOT NULL COMMENT '命令类型：1=relay_open 2=relay_close 3=relay_stop 4=poll 5=read_status 6=custom_hex',
  run_mode TINYINT UNSIGNED NOT NULL DEFAULT 2 COMMENT '命令产生时运行模式：1=auto 2=manu',
  rel1_target_status TINYINT UNSIGNED DEFAULT NULL COMMENT '目标继电器1状态：0=关 1=开',
  rel2_target_status TINYINT UNSIGNED DEFAULT NULL COMMENT '目标继电器2状态：0=关 1=开',
  command_hex TEXT NOT NULL COMMENT '实际下发十六进制命令',
  command_status TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '命令状态：1=created 2=sent 3=ack 4=succeeded 5=failed 6=timeout',
  requested_by BIGINT UNSIGNED DEFAULT NULL COMMENT '操作者用户ID',
  requested_at_ms TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '请求时间',
  sent_at_ms TIMESTAMP(3) NULL DEFAULT NULL COMMENT '下发时间',
  finished_at_ms TIMESTAMP(3) NULL DEFAULT NULL COMMENT '完成时间',
  gateway_response VARCHAR(512) DEFAULT NULL COMMENT 'Gateway返回',
  error_message VARCHAR(512) DEFAULT NULL COMMENT '失败原因',
  extra VARCHAR(512) DEFAULT NULL COMMENT '扩展信息，禁止JSON类型',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (command_id),
  KEY idx_command_gateway_status (gateway_id, command_status, requested_at_ms),
  KEY idx_command_user_requested (requested_by, requested_at_ms),
  KEY idx_updated_at (updated_at),
  CONSTRAINT fk_command_gateway FOREIGN KEY (gateway_id) REFERENCES cloud_gateways(gateway_id),
  CONSTRAINT fk_command_realtime FOREIGN KEY (realtime_id) REFERENCES device_realtime_maps(realtime_id),
  CONSTRAINT fk_command_user FOREIGN KEY (requested_by) REFERENCES users(user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='控制命令表';

CREATE TABLE IF NOT EXISTS control_results (
  result_id BIGINT UNSIGNED NOT NULL COMMENT '控制结果雪花ID',
  command_id BIGINT UNSIGNED DEFAULT NULL COMMENT '关联控制命令ID',
  gateway_id BIGINT UNSIGNED NOT NULL COMMENT '云边设备ID',
  realtime_id BIGINT UNSIGNED DEFAULT NULL COMMENT '采集执行器实时状态ID',
  frequency INT DEFAULT NULL COMMENT '通信频点',
  device_external_id VARCHAR(64) DEFAULT NULL COMMENT '采集执行器设备ID',
  result_status TINYINT UNSIGNED NOT NULL COMMENT '结果状态：1=ack 2=succeeded 3=failed 4=unknown',
  result_hex TEXT COMMENT '原始回包十六进制',
  result_snapshot TEXT COMMENT '解析后结果文本，禁止JSON类型',
  raw_event_id BIGINT UNSIGNED DEFAULT NULL COMMENT '原始事件ID',
  received_at_ms TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '接收时间',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (result_id),
  KEY idx_result_command (command_id),
  KEY idx_result_gateway_received (gateway_id, received_at_ms),
  KEY idx_updated_at (updated_at),
  CONSTRAINT fk_result_command FOREIGN KEY (command_id) REFERENCES control_commands(command_id),
  CONSTRAINT fk_result_gateway FOREIGN KEY (gateway_id) REFERENCES cloud_gateways(gateway_id),
  CONSTRAINT fk_result_realtime FOREIGN KEY (realtime_id) REFERENCES device_realtime_maps(realtime_id),
  CONSTRAINT fk_result_raw_event FOREIGN KEY (raw_event_id) REFERENCES raw_device_events(raw_event_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='控制结果表';

CREATE TABLE IF NOT EXISTS protocol_command_templates (
  template_id BIGINT UNSIGNED NOT NULL COMMENT '协议命令模板雪花ID',
  function_type TINYINT UNSIGNED NOT NULL COMMENT '功能类型：1=温度 2=墒情 3=限位 4=relay',
  command_type TINYINT UNSIGNED NOT NULL COMMENT '命令类型：1=poll 2=relay_open 3=relay_close 4=relay_stop 5=read_status 6=custom',
  channel_no INT DEFAULT NULL COMMENT '通道号',
  command_hex TEXT NOT NULL COMMENT '十六进制命令',
  description VARCHAR(512) DEFAULT NULL COMMENT '说明',
  active_status TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '启用状态：0=停用 1=启用',
  extra VARCHAR(512) DEFAULT NULL COMMENT '扩展信息，禁止JSON类型',
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (template_id),
  KEY idx_template_function_command (function_type, command_type),
  KEY idx_updated_at (updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='协议命令模板表';
