-- 004_bind_relations_user_id.sql
-- 补全 bind_relations 表缺失的列和约束
-- 适用于 MySQL 5.7，手动判断列是否存在

-- Step 1: 添加 user_id 列（如果不存在）
SET @col_exists = (SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
  WHERE TABLE_SCHEMA = 'nehuang_db' AND TABLE_NAME = 'bind_relations' AND COLUMN_NAME = 'user_id');
SET @sql = IF(@col_exists = 0,
  'ALTER TABLE bind_relations ADD COLUMN user_id BIGINT UNSIGNED DEFAULT NULL COMMENT ''绑定农户用户ID'' AFTER bind_id',
  'SELECT ''user_id already exists'' AS msg');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- Step 2: 添加 bind_user_name 列
SET @col_exists = (SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
  WHERE TABLE_SCHEMA = 'nehuang_db' AND TABLE_NAME = 'bind_relations' AND COLUMN_NAME = 'bind_user_name');
SET @sql = IF(@col_exists = 0,
  'ALTER TABLE bind_relations ADD COLUMN bind_user_name VARCHAR(64) NOT NULL DEFAULT '''' COMMENT ''绑定农户名称'' AFTER user_id',
  'SELECT ''bind_user_name already exists'' AS msg');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- Step 3: 添加 node_id 列
SET @col_exists = (SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
  WHERE TABLE_SCHEMA = 'nehuang_db' AND TABLE_NAME = 'bind_relations' AND COLUMN_NAME = 'node_id');
SET @sql = IF(@col_exists = 0,
  'ALTER TABLE bind_relations ADD COLUMN node_id BIGINT UNSIGNED DEFAULT NULL COMMENT ''采集执行器节点ID'' AFTER realtime_id',
  'SELECT ''node_id already exists'' AS msg');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- Step 4: 添加 point_id 列
SET @col_exists = (SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
  WHERE TABLE_SCHEMA = 'nehuang_db' AND TABLE_NAME = 'bind_relations' AND COLUMN_NAME = 'point_id');
SET @sql = IF(@col_exists = 0,
  'ALTER TABLE bind_relations ADD COLUMN point_id BIGINT UNSIGNED DEFAULT NULL COMMENT ''功能点ID'' AFTER node_id',
  'SELECT ''point_id already exists'' AS msg');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- Step 5: 添加 user_id + bind_status 复合索引
SET @idx_exists = (SELECT COUNT(*) FROM INFORMATION_SCHEMA.STATISTICS
  WHERE TABLE_SCHEMA = 'nehuang_db' AND TABLE_NAME = 'bind_relations' AND INDEX_NAME = 'idx_bind_user');
SET @sql = IF(@idx_exists = 0,
  'ALTER TABLE bind_relations ADD INDEX idx_bind_user (user_id, bind_status)',
  'SELECT ''idx_bind_user already exists'' AS msg');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- Step 6: 添加 point_id + bind_status 唯一索引（确保一个 point 同一时间只有一条 active 绑定）
SET @idx_exists = (SELECT COUNT(*) FROM INFORMATION_SCHEMA.STATISTICS
  WHERE TABLE_SCHEMA = 'nehuang_db' AND TABLE_NAME = 'bind_relations' AND INDEX_NAME = 'uk_point_active');
SET @sql = IF(@idx_exists = 0,
  'ALTER TABLE bind_relations ADD UNIQUE INDEX uk_point_active (point_id, bind_status)',
  'SELECT ''uk_point_active already exists'' AS msg');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
