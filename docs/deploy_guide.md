# 涅凰智农 IoT — 部署流程文档

> 最后更新：2026-06-29

## 一、服务器信息

| 项目 | 值 |
|------|-----|
| 云厂商 | 腾讯云 |
| IP | 43.143.208.153 |
| 系统 | CentOS 7 |
| Go 版本 | 1.20.12 |
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
│   │   ├── admin_handler.go    # 管理端接口
│   │   └── mini_handler.go     # C 端接口
│   ├── auth/                   # JWT 签发 & 校验（HS256，手写无三方依赖）
│   ├── config/                 # 环境变量配置
│   ├── domain/                 # 领域模型（User, Gateway, Role 等）
│   ├── idgen/                  # 雪花 ID 生成器
│   └── repository/mysql/       # MySQL 数据访问层
├── configs/mysql/              # MySQL 建表 SQL
├── schema/mysql/               # 数据库 schema
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
浏览器
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
│  └────────┬────────┘  └───────────────────┘ │
│           │                                  │
│           ▼                                  │
│  MySQL (127.0.0.1:3306)                     │
│  库: nehuang_db                              │
└─────────────────────────────────────────────┘
```

## 五、环境变量（.env）

```bash
# JWT 密钥
NEHUANG_JWT_SECRET=nehuang-dev-secret-change-me

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

### 7.1 更新 Go 服务端代码

```bash
# === 本地操作 ===
cd ~/Library/Mobile\ Documents/com~apple~CloudDocs/Knowledge/nehuang-iot-server/

# 修改代码后提交推送
git add -A
git commit -m "描述你的改动"
git push origin main

# 如果 CNB 拉不下来，直接 scp server.go
scp -i ~/shore.pem \
  internal/api/http/server.go \
  root@43.143.208.153:/home/work/nehuang-iot-server/internal/api/http/

# === 服务器操作 ===
ssh -i ~/shore.pem root@43.143.208.153

cd /home/work/nehuang-iot-server

# 从 CNB 拉取最新代码
git pull origin main

# Go 1.20 兼容性处理
sed -i 's/go 1.24.0/go 1.20/' go.mod
sed -i '/toolchain/d' go.mod

# 编译
go build -o nehuang-server ./cmd/server/

# 重启服务
kill -9 $(pgrep -f nehuang-server)
nohup ./nehuang-server > server.log 2>&1 &

# 验证
curl http://localhost:8080/healthz
```

### 7.2 更新前端代码

```bash
# === 本地操作 ===
cd ~/Library/Mobile\ Documents/com~apple~CloudDocs/Knowledge/prototype/

# 修改代码后提交推送
git add -A
git commit -m "描述你的改动"
git push cnb master

# === 服务器操作 ===
ssh -i ~/shore.pem root@43.143.208.153

cd /home/work/nehuang-iot-server

# 从 CNB 拉取最新前端
rm -rf prototype
git clone https://cnb.cool/wqedggc/nehuang-iot-prototype.git prototype

# 静态文件即时生效，无需重启服务
curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/admin/
# → 200
```

### 7.3 快速更新（不走 git，直接传文件）

```bash
# === 本地：打包上传 prototype ===
cd ~/Library/Mobile\ Documents/com~apple~CloudDocs/Knowledge/prototype/
tar czf /tmp/prototype.tar.gz --exclude='.git' .
scp -i ~/shore.pem /tmp/prototype.tar.gz root@43.143.208.153:/tmp/

# === 服务器：解压覆盖 ===
ssh -i ~/shore.pem root@43.143.208.153 \
  'cd /home/work/nehuang-iot-server/prototype && tar xzf /tmp/prototype.tar.gz'

# === 本地：直接 scp 单个 Go 文件 ===
scp -i ~/shore.pem \
  internal/api/http/server.go \
  root@43.143.208.153:/home/work/nehuang-iot-server/internal/api/http/

# === 服务器：重新编译重启 ===
ssh -i ~/shore.pem root@43.143.208.153 \
  'cd /home/work/nehuang-iot-server && sed -i "s/go 1.24.0/go 1.20/" go.mod && sed -i "/toolchain/d" go.mod && go build -o nehuang-server ./cmd/server/ && kill -9 $(pgrep -f nehuang-server); nohup ./nehuang-server > server.log 2>&1 &'
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
kill -9 $(pgrep -f nehuang-server)
cd /home/work/nehuang-iot-server && nohup ./nehuang-server > server.log 2>&1 &

# 健康检查
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz

# 测试登录
curl -X POST http://localhost:8080/api/v1/admin/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}'

# 验证静态文件
curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/admin/
curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/mini/
curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/mock/mock-api.js
```

## 九、注意事项

1. **Go 版本兼容**：本地开发用 Go 1.24，服务器是 Go 1.20。每次编译前必须 `sed` 修改 go.mod，并降级 `crypto` 和 `mysql driver`。
2. **CORS 中间件**：已在 `server.go` 的 `Routes()` 最外层应用，允许所有来源跨域访问。
3. **静态文件路由**：Go 1.20 的 FileServer 对 `/admin/index.html` 会 301 到 `/admin/`，所以首页重定向到 `/admin/` 而非 `/admin/index.html`。
4. **安全组**：需要在腾讯云控制台放行 8080 端口（TCP 入站），否则外网无法访问。
5. **CNB token**：服务器 git remote 中嵌入了 token，生产环境建议改用 SSH key 认证。
6. **前端模式切换**：`prototype/mock/mock-api.js` 中 `MODE` 变量控制调真实 API（`'real'`）还是本地 mock（`'mock'`）。
