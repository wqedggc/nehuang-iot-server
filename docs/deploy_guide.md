# 涅凰智农 IoT — 部署流程文档

> 最后更新：2026-06-29（设备事件 Mock 入库部署后更新）

## 一、服务器信息

| 项目 | 值 |
|------|-----|
| 云厂商 | 腾讯云 |
| IP | 43.143.208.153 |
| 系统 | CentOS 7 |
| Go 版本 | 1.24.0（2026-06-29 升级） |
| MySQL | 5.7（systemd 管理） |
| SSH 连接 | `ssh -i ~/shore.pem root@43.143.208.153` |
| 项目路径 | `/home/work/nehuang-iot-server/` |
| 服务端口 | 8080 |

## 二、代码仓库

| 仓库 | 地址 |
|------|------|
| Go 服务端 | `https://cnb.cool/wqedggc/nehuang-iot-server`（CNB） |
| 前端原型 | `https://cnb.cool/wqedggc/nehuang-iot-prototype`（CNB） |

本地项目路径：
- Go 服务端：`~/Library/Mobile Documents/com~apple~CloudDocs/Knowledge/nehuang-iot-server/`
- 前端原型：`~/Library/Mobile Documents/com~apple~CloudDocs/Knowledge/prototype/`

## 三、目录结构

```
/home/work/nehuang-iot-server/
├── cmd/server/main.go          # 服务入口
├── internal/
│   ├── api/http/
│   │   ├── server.go           # 路由注册 + 静态文件托管
│   │   ├── middleware.go        # CORS / 日志 / JWT 认证 / 角色校验
│   │   ├── adapter_handler.go  # 设备事件接收（Adapter API）
│   │   ├── admin_handler.go    # 管理端接口
│   │   └── mini_handler.go     # C 端接口
│   ├── auth/                   # JWT 签发 & 校验（HS256，手写无三方依赖）
│   ├── config/                 # 环境变量配置（含 AdapterToken）
│   ├── domain/                 # 领域模型（User, Gateway, Device 等）
│   ├── idgen/                  # 雪花 ID 生成器
│   └── repository/mysql/       # MySQL 数据访问层
│       ├── db.go               # 数据库连接
│       ├── user.go             # 用户
│       ├── gateway.go          # 云边设备 + Dashboard
│       └── device.go           # 设备事件/节点/功能点/传感器/绑定
├── configs/mysql/              # MySQL 建表 SQL
├── schema/mysql/               # 数据库 schema
├── scripts/
│   └── mock_device_events.sh   # Mock 设备事件上报脚本
├── prototype/                  # 前端静态文件（由 CNB 仓库拉取）
│   ├── admin/                  # 管理端 HTML/CSS/JS
│   ├── mini/                   # C 端 HTML/CSS/JS
│   └── mock/                   # Mock API 层（mock-api.js）
├── .env                        # 环境变量（JWT secret、MySQL 密码等）
├── nehuang-server              # 编译后的 Go 二进制
├── server.log                  # 运行日志
└── deploy.sh                   # 一键部署脚本（首次使用）
```

## 四、架构总览

```
浏览器 / 小程序
  │
  ▼
┌─────────────────────────────────────────────┐
│  Go HTTP Server (:8080)                     │
│                                              │
│  corsMiddleware → loggingMiddleware →        │
│                                              │
│  ┌─────────────────┐  ┌───────────────────┐ │
│  │ REST API 路由    │  │ 静态文件托管        │ │
│  │                 │  │                    │ │
│  │ /healthz        │  │ / → 302 → /admin/  │ │
│  │ /readyz         │  │ /admin/*           │ │
│  │ /api/v1/admin/* │  │ /mini/*            │ │
│  │ /api/v1/mini/*  │  │ /mock/*            │ │
│  │ /api/v1/gateways│  │                    │ │
│  │ /api/v1/adapter │  │                    │ │
│  └────────┬────────┘  └───────────────────┘ │
│           │                                  │
│           ▼                                  │
│  MySQL (127.0.0.1:3306)                     │
│  库: nehuang_db                              │
└─────────────────────────────────────────────┘
  ▲
  │ (X-Adapter-Token 认证)
  │
Mock 脚本 / TCP Gateway
```

## 五、环境变量（.env）

```bash
# JWT 密钥
NEHUANG_JWT_SECRET=nehuang-dev-secret-change-me

# Adapter API 认证 token（Mock 脚本和 Gateway 使用）
NEHUANG_ADAPTER_TOKEN=nehuang-adapter-dev-token

# 管理员初始密码
NEHUANG_ADMIN_PASSWORD=admin123

# MySQL
NEHUANG_MYSQL_USER=root
NEHUANG_MYSQL_PASSWORD=Nehuang@2026!
NEHUANG_MYSQL_HOST=127.0.0.1
NEHUANG_MYSQL_PORT=3306
NEHUANG_MYSQL_DATABASE=nehuang_db
```

## 六、首次部署流程

### 6.1 服务器环境准备

```bash
# 安装 Go 1.20（如果还没有）
yum install -y golang

# 安装 MySQL 5.7（如果还没有）
yum install -y mysql-server
systemctl start mysqld
systemctl enable mysqld

# 获取临时 root 密码
grep 'temporary password' /var/log/mysqld.log
mysql_secure_installation
```

### 6.2 初始化数据库

```bash
cd /home/work/nehuang-iot-server

# 建库
mysql -u root -p < schema/mysql/001_create_database.sql

# 建表
mysql -u root -p nehuang_db < schema/mysql/002_create_tables.sql

# 或使用一键脚本
./scripts/init_mysql_nehuang.sh
```

### 6.3 配置环境变量

```bash
cd /home/work/nehuang-iot-server
cp .env.example .env
# 编辑 .env，填入实际的 JWT secret 和 MySQL 密码
vim .env
```

### 6.4 拉取前端代码

```bash
cd /home/work/nehuang-iot-server
rm -rf prototype
git clone https://cnb.cool/wqedggc/nehuang-iot-prototype.git prototype
```

### 6.5 编译并启动

```bash
cd /home/work/nehuang-iot-server

# Go 1.20 兼容性处理（本地 go.mod 是 1.24）
sed -i 's/go 1.24.0/go 1.20/' go.mod
sed -i '/toolchain/d' go.mod

# 降级依赖
go get golang.org/x/crypto@v0.17.0
go get github.com/go-sql-driver/mysql@v1.7.1

# 编译
go build -o nehuang-server ./cmd/server/

# 启动（后台运行）
nohup ./nehuang-server > server.log 2>&1 &

# 验证
curl http://localhost:8080/healthz
# → {"status":"ok"}
```

## 七、日常更新部署流程

### 7.1 标准部署：推送代码 + 服务器编译

**⚠️ 铁律：所有代码（后端 Go + 前端原型）必须走 git 流程。严禁直接 scp 覆盖服务器文件。**

```bash
# === 本地操作 ===
cd ~/Library/Mobile\ Documents/com~apple~CloudDocs/Knowledge/nehuang-iot-server/

# 修改代码后提交推送
git add -A
git commit -m "描述你的改动"
git push cnb master

# === 服务器操作 ===
ssh -i ~/shore.pem root@43.143.208.153

cd /home/work/nehuang-iot-server

# 从 CNB 拉取最新代码
git pull origin master

# 编译（Go 1.24，无需 sed）
go build -o server ./cmd/server/

# 重启服务
pkill server; nohup ./server > server.log 2>&1 &

# 验证
sleep 2
curl http://localhost:8080/healthz
```

### 7.2 更新前端代码

**⚠️ 铁律：前端代码必须走 git 流程。本地改 → commit → push → 服务器 pull。严禁直接 scp 覆盖。**

```bash
# === 本地操作 ===
cd ~/Library/Mobile\ Documents/com~apple~CloudDocs/Knowledge/prototype/

# 修改代码后提交推送
git add -A
git commit -m "feat: 描述改动"
git push origin master

# === 服务器操作 ===
ssh -i ~/shore.pem root@43.143.208.153

# 服务器上有两个仓库：
# 1. 后端：/home/work/nehuang-iot-server/（含静态文件托管）
# 2. 前端：/home/work/nehuang-iot-prototype/（独立前端仓库）

cd /home/work/nehuang-iot-prototype
git pull origin master

# 前端是静态文件，无需编译，直接生效
# 如果后端通过静态文件托管提供前端，需要确认静态文件路径配置
```

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
source .env
nohup ./nehuang-server > server.log 2>&1 &

# 健康检查
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz

# 测试登录
curl -X POST http://localhost:8080/api/v1/admin/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}'

# 测试 Adapter 接口
curl -X POST http://localhost:8080/api/v1/adapter/device-events \
  -H "Content-Type: application/json" \
  -H "X-Adapter-Token: nehuang-adapter-dev-token" \
  -d '{"protocol_version":"mock-v1","source_type":"test","message_id":"test-1","gateway_name":"FP_TEST","gateway_external_id":"TEST","remote_ip":"127.0.0.1","remote_port":40000,"reported_at_ms":"2026-06-29T10:00:00Z","events":[{"event_type":"heartbeat"}]}'

# 运行 Mock 脚本（生成设备数据）
cd /home/work/nehuang-iot-server
bash scripts/mock_device_events.sh \
  --base-url http://127.0.0.1:8080 \
  --token nehuang-adapter-dev-token \
  --rounds 20 \
  --interval 1

# 验证静态文件
curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/admin/
curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/mini/
curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/mock/mock-api.js
```

## 九、注意事项

1. **Go 版本**：服务器和本地统一为 Go 1.24.0（2026-06-29 升级），编译无需 sed。
2. **CORS 中间件**：已在 `server.go` 的 `Routes()` 最外层应用，允许所有来源跨域访问。
3. **静态文件路由**：Go 1.20 的 FileServer 对 `/admin/index.html` 会 301 到 `/admin/`，所以首页重定向到 `/admin/` 而非 `/admin/index.html`。
4. **安全组**：需要在腾讯云控制台放行 8080 端口（TCP 入站），否则外网无法访问。
5. **CNB token**：服务器 git remote 中嵌入了 token，生产环境建议改用 SSH key 认证。
6. **前端模式切换**：`prototype/mock/mock-api.js` 中 `MODE` 变量控制调真实 API（`'real'`）还是本地 mock（`'mock'`）。当前已设为 `'real'`，后端未实现的接口会自动 fallback mock。
7. **Adapter Token**：`NEHUANG_ADAPTER_TOKEN` 用于设备事件上报接口认证，不走用户 JWT。服务器 `.env` 中必须配置，Mock 脚本和未来 Gateway 都需要使用此 token。
8. **NewServer 签名**：已从 `NewServer(repo, cfg.JWT)` 改为 `NewServer(repo, cfg)`，接收完整 Config（含 AdapterToken）。
