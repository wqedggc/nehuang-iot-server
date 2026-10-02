-- 008: raw 时间范围索引。已同名同定义可安全重跑；同名异定义立即失败。
-- 必须用 mysql 默认遇错停止模式，不得使用 --force；全部定义校验在任何 DDL 前完成。
-- 执行前按 docs/performance_and_retention.md 核对备份、空间和 metadata lock。
SET SESSION lock_wait_timeout=5;
SET @idx_raw_source_received_definition = (SELECT GROUP_CONCAT(CONCAT(column_name, ':', non_unique, ':', COALESCE(sub_part, 0)) ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='raw_device_events' AND index_name='idx_raw_source_received');
SET @check_sql = IF(@idx_raw_source_received_definition IS NULL OR @idx_raw_source_received_definition='source_type:1:0,received_at_ms:1:0,raw_event_id:1:0', 'SELECT 1', 'SELECT * FROM `ERROR_008_index_definition_mismatch`');
PREPARE check_stmt FROM @check_sql;
EXECUTE check_stmt;
DEALLOCATE PREPARE check_stmt;
SET @idx_raw_received_type_status_definition = (SELECT GROUP_CONCAT(CONCAT(column_name, ':', non_unique, ':', COALESCE(sub_part, 0)) ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='raw_device_events' AND index_name='idx_raw_received_type_status');
SET @check_sql = IF(@idx_raw_received_type_status_definition IS NULL OR @idx_raw_received_type_status_definition='received_at_ms:1:0,event_type:1:0,parse_status:1:0', 'SELECT 1', 'SELECT * FROM `ERROR_008_index_definition_mismatch`');
PREPARE check_stmt FROM @check_sql;
EXECUTE check_stmt;
DEALLOCATE PREPARE check_stmt;
SET @ddl = IF(@idx_raw_source_received_definition IS NULL, 'ALTER TABLE raw_device_events ADD INDEX idx_raw_source_received (source_type,received_at_ms,raw_event_id), ALGORITHM=INPLACE, LOCK=NONE', 'SELECT 1');
PREPARE ddl_stmt FROM @ddl;
EXECUTE ddl_stmt;
DEALLOCATE PREPARE ddl_stmt;
SET @ddl = IF(@idx_raw_received_type_status_definition IS NULL, 'ALTER TABLE raw_device_events ADD INDEX idx_raw_received_type_status (received_at_ms,event_type,parse_status), ALGORITHM=INPLACE, LOCK=NONE', 'SELECT 1');
PREPARE ddl_stmt FROM @ddl;
EXECUTE ddl_stmt;
DEALLOCATE PREPARE ddl_stmt;
