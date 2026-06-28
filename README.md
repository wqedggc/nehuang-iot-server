# 涅凰智农 IoT 云平台 V1.0

本目录是涅凰智农新 Go 服务端的项目上下文与后续工程落点。

新开 Codex 线程或上下文丢失时，先读：

1. `docs/project_context.md`
2. `docs/table_structure_v1.md`
3. `docs/architecture_v1.md`
4. `docs/mysql_setup.md`
5. `schema/mysql/001_nehuang_v1.sql`
6. `schema/mysql/002_device_node_point_model.sql`

辅助图：

- `docs/server_architecture_visual.html`：服务端视角全局架构图
- `docs/system_views_visual.html`：硬件链路、农户视角、管理员视角说明图

当前结论：保留旧 Python Gateway 的 TCP 接入职责，业务平台新建；MySQL 是业务唯一事实来源，`devices.json` 只作为 Gateway 输出缓存和短期 Adapter 输入。

表结构以 `schema/mysql/001_nehuang_v1.sql` 和 `schema/mysql/002_device_node_point_model.sql` 为最终 DDL，字段说明和写库 cycle 以 `docs/table_structure_v1.md` 为准。

本地 MySQL 初始化和导入方式见 `docs/mysql_setup.md`。不要复用 `shoreos_agent`，涅凰平台使用独立的 `nehuang_db` 和 `nehuang_agent`。

## 当前 Go 服务端骨架

框架选择：当前不引入 Gin / Hertz，先使用 Go 标准库 `net/http`。原因是一期目标是尽快把“数据库 -> 业务服务 -> REST API”的通路跑通，标准库已经支持路由、JSON 响应、中间件和优雅退出，依赖最少，后续如需统一接入更多 Web 能力再替换为 Gin 也不难。

启动：

```bash
cd /Users/shore/Knowledge/nehuang-iot-server
go run ./cmd/server
```

如需换端口：

```bash
NEHUANG_HTTP_ADDR=:18080 go run ./cmd/server
```

当前接口：

```text
GET /healthz
GET /readyz
GET /api/v1/admin/dashboard
GET /api/v1/gateways
```
