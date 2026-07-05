# 涅凰智农 IoT 服务端

这是涅凰智农 IoT 平台的 Go 服务端仓库，包含 REST API、MySQL 数据访问、嵌入式 Admin/C 端 H5 页面和 Adapter 设备事件入口。

私有项目背景、业务资料、产品判断和长期复盘不在本代码仓维护；当前代码事实以本 README、`docs/current_api.md` 和 `schema/mysql/README.md` 为准。

## Quick Start

```bash
cd /Users/shore/Desktop/ShoreOS/services/nehuang-iot-server
go test ./...
go run ./cmd/server
```

服务默认读取：

- MySQL defaults file：`configs/mysql/nehuang_agent.cnf`
- HTTP 地址：`NEHUANG_HTTP_ADDR`，默认 `:8080`
- Adapter token：`NEHUANG_ADAPTER_TOKEN`
- JWT secret：`NEHUANG_JWT_SECRET`

## Commands

| Command | Description |
|---------|-------------|
| `go test ./...` | 运行后端测试 |
| `go build ./cmd/server` | 编译服务端 |
| `go build ./cmd/gateway-adapter` | 编译硬件 TCP Adapter |
| `go build ./cmd/gateway-adapter-mock` | 编译硬件 TCP 模拟客户端 |
| `go build ./cmd/gateway-adapter-replay` | 编译 raw frame 回放工具 |
| `go build ./cmd/hash_password` | 编译部署脚本使用的密码哈希工具 |
| `bash -n deploy.sh` | 检查部署脚本语法 |
| `./deploy.sh` | 加载 `.env`、执行迁移、创建管理员、编译并启动服务 |

## Architecture

- HTTP：Go 标准库 `net/http`。
- Frontend：`web/static/` 通过 `//go:embed` 打进服务端二进制。
- Database：MySQL `nehuang_db`，schema 位于 `schema/mysql/*.sql`。
- Auth：Admin/Mini 使用不同 JWT audience；Adapter 使用 `X-Adapter-Token`。
- Device model：`cloud_gateways -> collector_nodes -> device_points` 是主模型，`device_realtime_maps` 只做兼容投影。
- Gateway Adapter：`cmd/gateway-adapter` 独立接收硬件 TCP，先落 raw frame journal，再转发可识别事件。

## Docs

- 文档入口：`docs/README.md`
- 当前 API：`docs/current_api.md`
- 当前产品现状与 Gap：`docs/current_product_status.md`
- Gateway Adapter：`docs/gateway_adapter.md`
- 硬件协议当前摘要：`docs/hardware_protocol_current.md`
- 部署流程：`docs/deploy_guide.md`
- 表结构和迁移：`schema/mysql/README.md`
- 历史产品/架构材料：`docs/` 下旧文档，仅作背景参考，改动前先核对代码事实。
