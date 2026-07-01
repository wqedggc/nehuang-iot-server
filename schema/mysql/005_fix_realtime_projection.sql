-- 005_fix_realtime_projection.sql
-- 修正 device_realtime_maps 兼容投影，使其能表达同一采集执行器下的多个功能点。
-- 兼容 MySQL 5.7：所有条件 DDL 均通过 INFORMATION_SCHEMA + PREPARE 实现。

USE nehuang_db;

-- Step 1: 增加 channel_no，和 device_points 的节点内通道语义保持一致。
SET @col_exists = (SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'device_realtime_maps' AND COLUMN_NAME = 'channel_no');
SET @sql = IF(@col_exists = 0,
  'ALTER TABLE device_realtime_maps ADD COLUMN channel_no INT NOT NULL DEFAULT 0 COMMENT ''节点内通道号，未知时为0'' AFTER function_type',
  'SELECT ''channel_no already exists'' AS msg');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- Step 2: 先新增和新三层点位模型一致的唯一键。
-- gateway_id 外键依赖一个以 gateway_id 起始的索引，必须先建新索引再删旧索引。
SET @idx_exists = (SELECT COUNT(*) FROM INFORMATION_SCHEMA.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'device_realtime_maps' AND INDEX_NAME = 'uk_realtime_gateway_freq_device_func_channel');
SET @sql = IF(@idx_exists = 0,
  'ALTER TABLE device_realtime_maps ADD UNIQUE INDEX uk_realtime_gateway_freq_device_func_channel (gateway_id, frequency, device_external_id, function_type, channel_no)',
  'SELECT ''uk_realtime_gateway_freq_device_func_channel already exists'' AS msg');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- Step 3: 删除旧唯一键。旧键缺少 function_type/channel_no，会阻止同一节点多个功能点同步。
SET @idx_exists = (SELECT COUNT(*) FROM INFORMATION_SCHEMA.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'device_realtime_maps' AND INDEX_NAME = 'uk_realtime_gateway_freq_device');
SET @sql = IF(@idx_exists > 0,
  'ALTER TABLE device_realtime_maps DROP INDEX uk_realtime_gateway_freq_device',
  'SELECT ''uk_realtime_gateway_freq_device already absent'' AS msg');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
