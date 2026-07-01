-- 003_user_management_binding.sql
-- 补充用户地址和备注字段。
-- 兼容 MySQL 5.7：手动判断列是否存在。

USE nehuang_db;

SET @col_exists = (SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'users' AND COLUMN_NAME = 'address');
SET @sql = IF(@col_exists = 0,
  'ALTER TABLE users ADD COLUMN address VARCHAR(512) DEFAULT '''' AFTER mobile',
  'SELECT ''address already exists'' AS msg');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col_exists = (SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'users' AND COLUMN_NAME = 'remark');
SET @sql = IF(@col_exists = 0,
  'ALTER TABLE users ADD COLUMN remark VARCHAR(512) DEFAULT '''' AFTER address',
  'SELECT ''remark already exists'' AS msg');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
