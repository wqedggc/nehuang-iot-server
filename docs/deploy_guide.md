# 涅凰智农 IoT — 部署流程文档

> 最后更新：2026-07-05（Go HTTP Server + Go TCP Gateway Adapter）

## 一、服务器信息

| 项目 | 值 |
|------|-----|
| 云厂商 | 腾讯云 |
| IP | 43.143.208.153 |
| 系统 | CentOS 7 |
| Go 版本 | 1.24.0 |
| MySQL | 5.7（systemd 管理） |
| SSH 连接 | `ssh -i ~/shore.pem root@43.143.208.153` |
| 项目路径 | `/home/work/nehuang-iot-server/` |
| HTTP 服务端口 | 8080 |
| 硬件 TCP Adapter 端口 | 8081 |
| Adapter health 端口 | 18081 |

## 二、代码仓库

| 仓库 | 地址 |
|------|------|
| Go 服务端（含前端） | `https://cnb.cool/wqedggc/nehuang-iot-server`（CNB） |
| Go 服务端（GitHub 镜像） | `https://github.com/wqedggc/nehuang-iot-server.git`（GitHub） |

本地项目路径：`/Users/shore/Desktop/ShoreOS/services/nehuang-iot-server/`

> **注意**：前端静态文件已通过 `//go:embed` 嵌入 Go 二进制中，无需独立的前端仓库。前端代码在 `web/static/` 目录下，与服务端代码同仓库管理。

## 三、目录结构

```
/home/work/nehuang-iot-server/
├── cmd/server/main.go          # HTTP 服务入口
├── cmd/gateway-adapter/        # 硬件 TCP Adapter
├── cmd/gateway-adapter-mock/   # TCP 模拟客户端
├── cmd/gateway-adapter-replay/ # raw journal 回放工具
├── cmd/hash_password/          # bcrypt 密码哈希工具（deploy.sh 使用）
├── internal/
│   ├── api/http/
│   │   ├── server.go           # 路由注册（含 //go:embed 静态文件托管）
│   │   ├── middleware.go        # CORS / 日志 / JWT 认证 / 角色校验
│   │   ├── adapter_handler.go  # 设备事件接收（Adapter API）
│   │   ├── admin_handler.go    # 管理端接口
│   │   ├── admin_dashboard.go  # Dashboard 接口
│   │   └── mini_handler.go     # C 端接口
│   ├── auth/                   # JWT 签发 & 校验（HS256，手写无三方依赖）
│   ├── config/                 # 环境变量配置（含 AdapterToken）
│   ├── domain/                 # 领域模型（User, Gateway, Device 等）
│   ├── idgen/                  # 雪花 ID 生成器
│   ├── service/                # 业务服务层（Dashboard 聚合等）
│   └── repository/mysql/       # MySQL 数据访问层
│       ├── db.go               # 数据库连接
│       ├── user.go             # 用户
│       ├── gateway.go          # 云边设备 + Dashboard
│       └── device.go           # 设备事件/节点/功能点/传感器/绑定
├── web/static/                 # 前端静态文件（//go:embed 嵌入二进制）
│   ├── admin/                  # 管理端 HTML/CSS/JS
│   ├── mini/                   # C 端 HTML/CSS/JS
│   └── mock/                   # Mock API 层（mock-api.js）
├── configs/mysql/              # MySQL 配置文件（nehuang_agent.cnf）
├── schema/mysql/               # 数据库迁移 SQL（001-005，deploy.sh 顺序执行）
├── scripts/
│   └── mock_device_events.sh   # Mock HTTP 设备事件上报脚本
├── .env                        # 环境变量（JWT secret、MySQL 密码等）
├── .env.example                # 环境变量模板
├── deploy.sh                   # 一键部署脚本（首次部署）
├── nehuang-server              # 编译后的 Go 二进制
├── gateway-adapter             # 编译后的 TCP Adapter 二进制（待纳入正式 deploy）
├── server.log                  # HTTP 服务运行日志
└── gateway-adapter.log         # TCP Adapter 运行日志
```

## 四、架构总览

```
浏览器 / 小程序
  │
  ▼
┌─────────────────────────────────────────────────────────────┐
│  Go HTTP Server (:8080)  — 单一二进制，含嵌入式前端          │
│                                                              │
│  corsMiddleware → loggingMiddleware →                        │
│                                                              │
│  ┌───────────────────┐  ┌─────────────────────────────────┐ │
│  │ REST API 路由      │  │ 静态文件托管 (//go:embed)        │ │
│  │                   │  │                                  │ │
│  │ /healthz          │  │ / → 302 → /admin/               │ │
│  │ /readyz           │  │ /admin/*  (web/static/admin/)   │ │
│  │ /api/v1/admin/*   │  │ /mini/*   (web/static/mini/)    │ │
│  │ /api/v1/mini/*    │  │ /mock/*   (web/static/mock/)    │ │
│  │ /api/v1/gateways  │  │                                  │ │
│  │ /api/v1/adapter/* │  │                                  │ │
│  └────────┬──────────┘  └─────────────────────────────────┘ │
│           │                                                  │
│           ▼                                                  │
│  MySQL (127.0.0.1:3306)                                     │
│  库: nehuang_db                                              │
└─────────────────────────────────────────────────────────────┘
  ▲
  │ HTTP 内部事件 / 命令接口 (X-Adapter-Token)
  │
┌─────────────────────────────────────────────────────────────┐
│ Go Gateway Adapter (:8081 TCP, :18081 health)               │
│ - 硬件 TCP 连接                                              │
│ - raw frame journal                                          │
│ - register/heartbeat 转发                                    │
│ - 后续工作包/告警包/ACK parser                               │
└─────────────────────────────────────────────────────────────┘
  ▲
  │ TCP bytes
  │
硬件网关 / gateway-adapter-mock
```

> **前端部署方式**：前端静态文件（`web/static/`）通过 Go 1.16+ 的 `//go:embed` 指令在编译时嵌入二进制中，无需单独部署前端。修改前端代码后，重新 `go build` 即可生效。

## 五、环境变量（.env）

```bash
# JWT 密钥
NEHUANG_JWT_SECRET=nehuang-dev-secret-change-me

# Adapter API 认证 token（Mock 脚本和 Gateway 使用）
NEHUANG_ADAPTER_TOKEN=nehuang-adapter-dev-token

# Gateway Adapter
NEHUANG_GATEWAY_TCP_ADDR=:8081
NEHUANG_GATEWAY_HEALTH_ADDR=:18081
NEHUANG_GATEWAY_API_BASE_URL=http://43.143.208.153:8080
NEHUANG_GATEWAY_RAW_LOG_DIR=data/gateway-adapter/raw-frames
NEHUANG_GATEWAY_DEBUG=false

# 管理员初始密码
NEHUANG_ADMIN_PASSWORD=admin123

# 微信小程序登录配置
# 注意：AppSecret 只能写入服务器 .env，禁止提交到代码库或文档。
NEHUANG_WECHAT_APPID=wx_your_mini_program_appid
NEHUANG_WECHAT_SECRET=your_wechat_app_secret
NEHUANG_ENABLE_MOCK_LOGIN=false

# MySQL
NEHUANG_MYSQL_USER=root
NEHUANG_MYSQL_PASSWORD=Nehuang@2026!
NEHUANG_MYSQL_HOST=127.0.0.1
NEHUANG_MYSQL_PORT=3306
NEHUANG_MYSQL_DATABASE=nehuang_db

# 离线扫描（默认开启）
NEHUANG_OFFLINE_SCAN_ENABLED=true
NEHUANG_OFFLINE_THRESHOLD_SECONDS=35
NEHUANG_OFFLINE_SCAN_INTERVAL_SECONDS=10
```

## 六、首次部署流程

### 6.1 服务器环境准备

```bash
# 安装 Go 1.24
yum install -y golang

# 安装 MySQL 5.7（如果还没有）
yum install -y mysql-server
systemctl start mysqld
systemctl enable mysqld

# 获取临时 root 密码
grep 'temporary password' /var/log/mysqld.log
mysql_secure_installation
```

### 6.2 克隆仓库

```bash
cd /home/work
git clone https://cnb.cool/wqedggc/nehuang-iot-server.git
cd nehuang-iot-server
```

### 6.3 配置环境变量

```bash
cp .env.example .env
# 编辑 .env，填入实际的 JWT secret、MySQL 密码、Adapter Token 等
vim .env
```

### 6.4 一键部署

```bash
chmod +x deploy.sh
./deploy.sh
```

`deploy.sh` 自动完成以下步骤：
1. 检查前置条件（Go、MySQL 客户端）
2. 加载 `.env` 环境变量
3. 生成 MySQL 配置文件 `configs/mysql/nehuang_agent.cnf`
4. 创建/检查 `schema_migrations`，按顺序执行 `schema/mysql/*.sql`
5. 创建管理员账号（user_id=10001, username=admin）
6. 编译服务端二进制（含嵌入式前端）
7. 后台启动服务
8. 健康检查验证

### 6.5 验证

```bash
# 健康检查
curl http://localhost:8080/healthz
# → {"status":"ok"}

# 就绪检查
curl http://localhost:8080/readyz
# → {"status":"ready"}

# 访问管理端
curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/admin/
# → 200

# 测试登录
curl -X POST http://localhost:8080/api/v1/admin/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}'
```

## 七、日常更新部署流程

### 7.1 标准部署：推送代码 + 服务器编译

**⚠️ 铁律：所有代码（后端 Go + 前端静态文件）必须走 git 流程。严禁直接 scp 覆盖服务器文件。**

```bash
# === 本地操作 ===
cd /Users/shore/Desktop/ShoreOS/services/nehuang-iot-server/

# 修改代码后提交，并推送到 CNB + GitHub
git add -A
git commit -m "描述你的改动"
git push cnb master
/Users/shore/Desktop/Knowledge/tools/github_device_push.sh \
  --repo /Users/shore/Desktop/ShoreOS/services/nehuang-iot-server \
  --remote origin \
  --ref master

# === 服务器操作 ===
ssh -i ~/shore.pem root@43.143.208.153

cd /home/work/nehuang-iot-server

# 从 CNB 拉取最新代码
git pull origin master

# 编译（Go 1.24，含嵌入式前端，无需额外步骤）
go build -o nehuang-server ./cmd/server/

# 重启服务
pkill -f nehuang-server; sleep 1
set -a
source .env
set +a
nohup ./nehuang-server > server.log 2>&1 &

# 验证
sleep 2
curl http://localhost:8080/healthz
```

### 7.2 前端代码更新

前端代码位于 `web/static/` 目录下，通过 `//go:embed` 嵌入 Go 二进制。修改流程与后端完全一致：

```bash
# === 本地操作 ===
# 修改 web/static/ 下的 HTML/CSS/JS 文件
git add -A
git commit -m "feat: 描述前端改动"
git push cnb master
/Users/shore/Desktop/Knowledge/tools/github_device_push.sh \
  --repo /Users/shore/Desktop/ShoreOS/services/nehuang-iot-server \
  --remote origin \
  --ref master

# === 服务器操作 ===
ssh -i ~/shore.pem root@43.143.208.153
cd /home/work/nehuang-iot-server
git pull origin master
go build -o nehuang-server ./cmd/server/
pkill -f nehuang-server; sleep 1
set -a
source .env
set +a
nohup ./nehuang-server > server.log 2>&1 &
```

> **不再需要**独立的前端仓库（`nehuang-iot-prototype`）和独立的前端部署步骤。前后端统一在一个仓库、一个二进制中。

## 八、运维命令速查

```bash
# 查看服务状态
ps aux | grep nehuang-server

# 查看监听端口
ss -tlnp | grep 8080

# 查看日志
tail -f /home/work/nehuang-iot-server/server.log

# 重启服务
pkill -f nehuang-server || true
sleep 1
cd /home/work/nehuang-iot-server
set -a
source .env
set +a
nohup ./nehuang-server > server.log 2>&1 &

# 健康检查
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz

# 测试登录
curl -X POST http://localhost:8080/api/v1/admin/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}'

# 测试 Adapter 接口
curl -X POST http://localhost:8080/api/v1/adapter/raw-frames \
  -H "Content-Type: application/json" \
  -H "X-Adapter-Token: nehuang-adapter-dev-token" \
  -d '{"protocol_version":"nehuang-gateway-adapter-v1","source_type":"gateway_adapter","message_id":"manual-1","connection_id":"manual","gateway_name":"","remote_ip":"127.0.0.1","remote_port":40000,"direction":"inbound","payload_hex":"46505F544553545F41444D494E5F3030310A","received_at":"2026-07-04T18:08:04.79022Z","raw_seq":1}'

# 兼容旧 JSON 事件入口
curl -X POST http://localhost:8080/api/v1/adapter/device-events \
  -H "Content-Type: application/json" \
  -H "X-Adapter-Token: nehuang-adapter-dev-token" \
  -d '{"protocol_version":"mock-v1","source_type":"test","message_id":"test-1","gateway_name":"FP_TEST","gateway_external_id":"TEST","remote_ip":"127.0.0.1","remote_port":40000,"reported_at_ms":"2026-06-29T10:00:00Z","events":[{"event_type":"heartbeat"}]}'

# 运行 HTTP Mock 脚本（生成设备数据）
cd /home/work/nehuang-iot-server
bash scripts/mock_device_events.sh \
  --base-url http://43.143.208.153:8080 \
  --token nehuang-adapter-dev-token \
  --rounds 20 \
  --interval 1

# 启动 Gateway Adapter（临时方式；生产建议 systemd）
NEHUANG_GATEWAY_TCP_ADDR=:8081 \
NEHUANG_GATEWAY_HEALTH_ADDR=:18081 \
NEHUANG_GATEWAY_API_BASE_URL=http://43.143.208.153:8080 \
NEHUANG_ADAPTER_TOKEN=nehuang-adapter-dev-token \
NEHUANG_GATEWAY_RAW_LOG_DIR=data/gateway-adapter/raw-frames \
nohup ./gateway-adapter > gateway-adapter.log 2>&1 &

# 测试 Gateway Adapter health
curl http://localhost:18081/healthz

# 本地模拟硬件 TCP 发包
go run ./cmd/gateway-adapter-mock -addr 127.0.0.1:8081 -frame all

# 查看 raw journal
tail -n 20 data/gateway-adapter/raw-frames/raw-frames-YYYYMMDD.jsonl

# 查看 Go 服务 raw payload 入库情况
curl -H "Authorization: Bearer <admin-token>" \
  "http://localhost:8080/api/v1/admin/payload-events?limit=20"

# 验证静态文件（嵌入式前端）
curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/admin/
curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/mini/
curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/mock/mock-api.js
```

## 九、系统功能概述

### 9.1 业务模型

```
Gateway (云边网关) ──1:N──→ CollectorNode (采集执行器) ──1:N──→ DevicePoint (功能点)
                                                                      │
                                                            BindRelation (N:1 User 农户)
                                                                      │
                                                            SensorData (时序) / ControlCommand
```

### 9.2 数据库表（9 张业务表）

| 表名 | 用途 |
|------|------|
| `users` | 平台用户（管理员 + 农户），支持密码和微信 OpenID 登录 |
| `cloud_gateways` | 云边网关设备（FP_xxx），记录在线状态、IP、心跳 |
| `collector_nodes` | 采集执行器节点（gateway_id + frequency + device_external_id） |
| `device_points` | 功能点（温度/墒情/限位/继电器/水泵/风机） |
| `bind_relations` | 农户↔功能点绑定（active/unbound/deleted） |
| `sensor_data` | 传感器时序数据（历史曲线） |
| `raw_device_events` | 原始设备事件（register/heartbeat/telemetry/control_result） |
| `control_commands` | 控制命令（created→sent→ack→succeeded/failed/timeout） |
| `op_records` | 操作审计日志 |

### 9.3 API 端点速查

#### Health（无需认证）
| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/healthz` | 健康检查 |
| `GET` | `/readyz` | 就绪检查（含 DB ping） |

#### Admin（JWT aud=admin, role=super_admin/admin）
| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/api/v1/admin/login` | 管理员登录 |
| `GET` | `/api/v1/admin/me` | 当前管理员信息 |
| `GET` | `/api/v1/admin/dashboard?range=1d\|3d\|7d\|30d` | Dashboard 聚合统计 |
| `GET` | `/api/v1/admin/users` | 用户列表 |
| `GET` | `/api/v1/admin/farmers?keyword=` | 农户列表（支持搜索） |
| `GET` | `/api/v1/admin/farmers/{id}` | 农户详情 |
| `PATCH` | `/api/v1/admin/farmers/{id}` | 编辑农户资料 |
| `GET` | `/api/v1/admin/farmers/{id}/bindings` | 农户绑定列表 |
| `GET` | `/api/v1/admin/collectors` | 采集执行器列表 |
| `GET` | `/api/v1/admin/bindings` | 绑定关系列表 |
| `POST` | `/api/v1/admin/bindings` | 创建绑定 |
| `DELETE` | `/api/v1/admin/bindings/{id}` | 解绑 |
| `GET` | `/api/v1/admin/raw-events?limit=` | 原始设备事件 |
| `GET` | `/api/v1/admin/control-logs?limit=` | 控制日志 |
| `GET` | `/api/v1/admin/device-tree` | 设备树（三层结构） |
| `GET` | `/api/v1/admin/device-points?keyword=&bind_status=` | 功能点搜索 |

#### Mini-Program（JWT aud=mini, role=farmer/admin）
| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/api/v1/mini/mock-login` | 模拟登录（开发用） |
| `POST` | `/api/v1/mini/login` | 微信登录 |
| `GET` | `/api/v1/mini/me` | 当前农户信息 |
| `PATCH` | `/api/v1/mini/me` | 编辑个人资料 |
| `GET` | `/api/v1/mini/devices` | 我的设备列表 |
| `GET` | `/api/v1/mini/devices/{id}/realtime` | 设备实时数据 |
| `GET` | `/api/v1/mini/devices/{id}/history?range=1d` | 历史曲线，支持 1d/3d/7d/30d |
| `POST` | `/api/v1/mini/devices/{id}/control` | 创建设备异步控制命令 |

#### Adapter（X-Adapter-Token 认证）
| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/api/v1/adapter/device-events` | 设备事件上报 |
| `GET` | `/api/v1/adapter/commands/pending?gateway_external_id=` | Adapter 拉取待下发命令 |
| `POST` | `/api/v1/adapter/commands/{command_id}/sent` | Adapter 标记命令已下发 |
| `POST` | `/api/v1/adapter/commands/{command_id}/result` | Adapter 回写 ACK/成功/失败/超时 |

#### Gateways（JWT, role=admin）
| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/v1/gateways?limit=` | 网关设备列表 |

### 9.4 前端模块

| 模块 | 路径 | 说明 |
|------|------|------|
| 管理后台 | `/admin/` | Dashboard、用户管理、设备管理、绑定管理、控制日志 |
| 农户端 | `/mini/` | 设备列表、实时数据、历史曲线、个人资料 |
| Mock 层 | `/mock/mock-api.js` | 三模式 API 层（mock/real/offline） |
| 版本号 | `/version.js` | 全局版本，以当前静态资源为准 |

### 9.5 功能完成度

| ✅ 已完成 | ⚠️ 待完成 |
|----------|----------|
| 用户认证（JWT 双受众隔离） | 设备控制链路真实硬件下发和 ACK |
| 管理端基础 CRUD | Admin 控制页面和点位编辑完整 UI |
| 农户绑定全流程（搜索→绑定→解绑） | 微信小程序真实对接 |
| HTTP 设备事件 Mock 入库 | 真实工作包 parser |
| Go TCP Gateway Adapter raw capture | 告警 parser、去重和通知 |
| Dashboard 真实数据统计（5 类） | Gateway Adapter 正式 systemd/deploy 接入 |
| 嵌入式前端部署 | 生产环境安全加固 |

## 十、注意事项

1. **Go 版本**：服务器和本地统一为 Go 1.24.0，编译无需任何兼容性处理。
2. **前端嵌入**：前端静态文件（`web/static/`）通过 `//go:embed` 在编译时嵌入二进制，修改前端代码后需重新 `go build`。不再需要独立的前端仓库或手动同步静态文件。
3. **CORS 中间件**：已在 `server.go` 的 `Routes()` 最外层应用，允许所有来源跨域访问。
4. **安全组**：HTTP 管理端/API 需要放行 8080；硬件 TCP Adapter 正式联调时还需要放行 8081；Adapter health 18081 建议只允许本机或内网访问。
5. **CNB token**：服务器 git remote 中嵌入了 token，生产环境建议改用 SSH key 认证。
6. **GitHub 认证**：本机或 Agent 推送 GitHub 时必须使用 `/Users/shore/Desktop/Knowledge/tools/github_device_push.sh`。该脚本使用 device flow 和临时 `GH_CONFIG_DIR`，并在 `git push` 时清空默认 credential helper，禁止使用 macOS Keychain 或登录钥匙串密码。
7. **前端模式切换**：`web/static/mock/mock-api.js` 中 `MODE` 变量控制调真实 API（`'real'`）还是本地 mock（`'mock'`）。当前已设为 `'real'`，后端未实现的接口会自动 fallback mock。
8. **Adapter Token**：`NEHUANG_ADAPTER_TOKEN` 用于 `gateway-adapter` 调 Go HTTP 内部接口，不走用户 JWT。硬件网关第一版不直接使用该 token，而是连 TCP 端口。
9. **NewServer 签名**：`NewServer(repo repository, cfg config.Config)`，接收完整 Config（含 JWT + AdapterToken）。
10. **二进制名**：HTTP 服务使用 `nehuang-server`，编译命令为 `go build -o nehuang-server ./cmd/server/`；TCP Adapter 使用 `gateway-adapter`，编译命令为 `go build -o gateway-adapter ./cmd/gateway-adapter/`。
11. **环境变量**：Go 程序通过 `os.Getenv` 读取环境变量，`.env` 文件由 `deploy.sh` 加载。手动 `nohup` 启动时确保所需环境变量已 export（或使用 `deploy.sh` 自动处理）。
12. **部署铁律**：所有代码变更（后端 + 前端）必须走 `git commit → git push → 服务器 git pull → go build` 流程，严禁 scp 覆盖。
