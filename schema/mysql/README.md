# MySQL Schema

当前正式建表脚本：

- `001_nehuang_v1.sql`
- `002_device_node_point_model.sql`
- `003_user_management_binding.sql`
- `004_bind_relations_user_id.sql`
- `005_fix_realtime_projection.sql`

执行前确认：

1. MySQL 支持 5.7/8.0，DDL 使用 `utf8mb4_unicode_ci`。
2. 字符集使用 `utf8mb4`。
3. 所有业务表必须有主键、`created_at`、`updated_at`。
4. 甲方 5 张核心表字段不能删除或改名。
5. SQL 文件必须按文件名顺序执行。
6. `deploy.sh` 使用 `schema_migrations` 记录已执行版本，并会对旧库做 baseline 探测，避免重复执行旧 ALTER。

开发写库链路见：

- `../../docs/table_structure_v1.md`
