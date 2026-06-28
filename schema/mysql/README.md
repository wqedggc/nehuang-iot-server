# MySQL Schema

当前正式建表脚本：

- `001_nehuang_v1.sql`
- `002_device_node_point_model.sql`

执行前确认：

1. MySQL 版本建议 8.0+。
2. 字符集使用 `utf8mb4`。
3. 所有业务表必须有主键、`created_at`、`updated_at`。
4. 甲方 5 张核心表字段不能删除或改名。
5. `002_device_node_point_model.sql` 需要在 `001_nehuang_v1.sql` 之后执行，用于补齐采集执行器节点和功能点抽象。

开发写库链路见：

- `../../docs/table_structure_v1.md`
