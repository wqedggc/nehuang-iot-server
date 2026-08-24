-- Adapter durable raw 重放的中心幂等约束。
-- capture_id 由 Adapter 在原始字节首次落盘前生成，重试必须复用同一值。
-- 独立于历史 message_id，避免为了上线唯一索引而改写/删除旧重复事实。
-- 条件 DDL 同时修复早期数据库已建表但缺少 message_id/event_count/parsed_count 的基线漂移。

SET @has_message_id = (
  SELECT COUNT(*) FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'raw_device_events' AND column_name = 'message_id'
);
SET @ddl = IF(
  @has_message_id = 0,
  'ALTER TABLE raw_device_events ADD COLUMN message_id VARCHAR(128) DEFAULT NULL COMMENT ''协议或上报消息ID'' AFTER parse_error',
  'SELECT 1'
);
PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @has_event_count = (
  SELECT COUNT(*) FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'raw_device_events' AND column_name = 'event_count'
);
SET @ddl = IF(
  @has_event_count = 0,
  'ALTER TABLE raw_device_events ADD COLUMN event_count INT NOT NULL DEFAULT 0 COMMENT ''事件总数'' AFTER message_id',
  'SELECT 1'
);
PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @has_parsed_count = (
  SELECT COUNT(*) FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'raw_device_events' AND column_name = 'parsed_count'
);
SET @ddl = IF(
  @has_parsed_count = 0,
  'ALTER TABLE raw_device_events ADD COLUMN parsed_count INT NOT NULL DEFAULT 0 COMMENT ''成功解析事件数'' AFTER event_count',
  'SELECT 1'
);
PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @has_capture_id = (
  SELECT COUNT(*) FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'raw_device_events' AND column_name = 'capture_id'
);
SET @ddl = IF(
  @has_capture_id = 0,
  'ALTER TABLE raw_device_events ADD COLUMN capture_id VARCHAR(128) DEFAULT NULL COMMENT ''Adapter原始字节捕获幂等ID'' AFTER message_id',
  'SELECT 1'
);
PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

-- 为旧 Adapter journal 对应的中心事实补 capture_id：每组历史 message_id 只选择
-- 最早一条作为 canonical，重复历史行原样保留且 capture_id 继续为 NULL。
-- 这样首次升级不会把磁盘上的旧 journal 全量复制一遍，也不会删除历史重复事实。
UPDATE raw_device_events AS target
JOIN (
  SELECT source_type, message_id, MIN(raw_event_id) AS canonical_raw_event_id
  FROM raw_device_events
  WHERE message_id IS NOT NULL AND message_id <> ''
  GROUP BY source_type, message_id
) AS canonical
  ON canonical.canonical_raw_event_id = target.raw_event_id
SET target.capture_id = target.message_id
WHERE target.capture_id IS NULL;

SET @has_capture_index = (
  SELECT COUNT(*) FROM information_schema.statistics
  WHERE table_schema = DATABASE() AND table_name = 'raw_device_events' AND index_name = 'uk_raw_source_capture'
);
SET @ddl = IF(
  @has_capture_index = 0,
  'ALTER TABLE raw_device_events ADD UNIQUE KEY uk_raw_source_capture (source_type, capture_id)',
  'SELECT 1'
);
PREPARE stmt FROM @ddl;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
