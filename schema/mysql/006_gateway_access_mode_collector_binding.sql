-- 涅凰智农 IoT 云平台：设备网关共享/独享模式与采集执行器绑定索引
-- 兼容 MySQL 5.7：使用 INFORMATION_SCHEMA + PREPARE 避免重复列/索引错误。

USE nehuang_db;

SET @has_access_mode := (
  SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'cloud_gateways'
    AND COLUMN_NAME = 'access_mode'
);
SET @sql := IF(@has_access_mode = 0,
  'ALTER TABLE cloud_gateways ADD COLUMN access_mode VARCHAR(16) NOT NULL DEFAULT ''shared'' COMMENT ''使用模式：shared=共享 exclusive=独享'' AFTER gateway_name',
  'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @has_access_idx := (
  SELECT COUNT(*) FROM INFORMATION_SCHEMA.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'cloud_gateways'
    AND INDEX_NAME = 'idx_cloud_gateways_access_mode'
);
SET @sql := IF(@has_access_idx = 0,
  'ALTER TABLE cloud_gateways ADD KEY idx_cloud_gateways_access_mode (access_mode)',
  'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @has_bind_node_idx := (
  SELECT COUNT(*) FROM INFORMATION_SCHEMA.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'bind_relations'
    AND INDEX_NAME = 'idx_bind_user_node_status'
);
SET @sql := IF(@has_bind_node_idx = 0,
  'ALTER TABLE bind_relations ADD KEY idx_bind_user_node_status (user_id, node_id, bind_status)',
  'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

