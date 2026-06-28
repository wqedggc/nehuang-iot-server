#!/usr/bin/env bash
# ============================================================================
# 涅凰智农 IoT 云平台 — 一键部署脚本
# ============================================================================
# 功能：
#   1. 检查前置条件（go、mysql）
#   2. 加载 .env 环境变量
#   3. 生成 MySQL 配置文件（configs/mysql/nehuang_agent.cnf）
#   4. 初始化数据库（建表 DDL）
#   5. 创建管理员账号（bcrypt 密码哈希）
#   6. 编译服务端二进制
#   7. 后台启动服务
#   8. 健康检查验证
#
# 用法：
#   chmod +x deploy.sh
#   ./deploy.sh
# ============================================================================

set -euo pipefail

# ----------------------------------------------------------------------------
# 颜色定义
# ----------------------------------------------------------------------------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# ----------------------------------------------------------------------------
# 日志函数
# ----------------------------------------------------------------------------
log_info() {
    echo -e "${BLUE}[INFO]${NC} $*"
}

log_success() {
    echo -e "${GREEN}[OK]${NC} $*"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $*"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $*" >&2
}

# ----------------------------------------------------------------------------
# 错误陷阱：任何步骤失败时输出错误信息并退出
# ----------------------------------------------------------------------------
trap 'log_error "部署过程中断，请检查上方日志。退出码: $?"' ERR

# ----------------------------------------------------------------------------
# 项目根目录（脚本所在目录即为项目根）
# ----------------------------------------------------------------------------
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$PROJECT_ROOT"

log_info "项目根目录: $PROJECT_ROOT"

# ----------------------------------------------------------------------------
# Step 1: 检查前置条件
# ----------------------------------------------------------------------------
log_info "Step 1/8: 检查前置条件..."

if ! command -v go &>/dev/null; then
    log_error "未找到 go 命令，请先安装 Go (https://go.dev/dl/)"
    exit 1
fi
log_success "Go 版本: $(go version)"

if ! command -v mysql &>/dev/null; then
    log_error "未找到 mysql 命令，请先安装 MySQL 客户端"
    exit 1
fi
log_success "MySQL 客户端: $(mysql --version)"

# ----------------------------------------------------------------------------
# Step 2: 加载 .env 环境变量
# ----------------------------------------------------------------------------
log_info "Step 2/8: 加载环境变量..."

if [ ! -f ".env" ]; then
    if [ -f ".env.example" ]; then
        log_warn ".env 文件不存在，从 .env.example 复制..."
        cp .env.example .env
        log_warn "请编辑 .env 文件填入实际配置后重新运行本脚本"
        log_info "  vi .env"
        exit 0
    else
        log_error ".env 和 .env.example 均不存在，无法继续"
        exit 1
    fi
fi

# 加载 .env（允许变量展开）
set -a
# shellcheck disable=SC1091
source .env
set +a

# 设置默认值
NEHUANG_JWT_SECRET="${NEHUANG_JWT_SECRET:-nehuang-dev-secret-change-me}"
NEHUANG_ADMIN_PASSWORD="${NEHUANG_ADMIN_PASSWORD:-admin123}"
NEHUANG_MYSQL_USER="${NEHUANG_MYSQL_USER:-root}"
NEHUANG_MYSQL_HOST="${NEHUANG_MYSQL_HOST:-127.0.0.1}"
NEHUANG_MYSQL_PORT="${NEHUANG_MYSQL_PORT:-3306}"
NEHUANG_MYSQL_DATABASE="${NEHUANG_MYSQL_DATABASE:-nehuang_db}"
NEHUANG_MYSQL_SOCKET="${NEHUANG_MYSQL_SOCKET:-}"

log_success "环境变量加载完成"
log_info "  数据库: ${NEHUANG_MYSQL_USER}@${NEHUANG_MYSQL_HOST}:${NEHUANG_MYSQL_PORT}/${NEHUANG_MYSQL_DATABASE}"
if [ -n "$NEHUANG_MYSQL_SOCKET" ]; then
    log_info "  Socket: $NEHUANG_MYSQL_SOCKET"
fi

# ----------------------------------------------------------------------------
# Step 3: 生成 MySQL 配置文件
# ----------------------------------------------------------------------------
log_info "Step 3/8: 生成 MySQL 配置文件..."

CNF_DIR="configs/mysql"
CNF_FILE="${CNF_DIR}/nehuang_agent.cnf"

mkdir -p "$CNF_DIR"

# 生成 .cnf 文件（格式与 config.go loadMySQLDefaultsFile 解析逻辑一致）
{
    echo "[client]"
    echo "user=${NEHUANG_MYSQL_USER}"
    echo "password=${NEHUANG_MYSQL_PASSWORD}"
    echo "host=${NEHUANG_MYSQL_HOST}"
    echo "port=${NEHUANG_MYSQL_PORT}"
    echo "database=${NEHUANG_MYSQL_DATABASE}"
    echo "default-character-set=utf8mb4"
    if [ -n "$NEHUANG_MYSQL_SOCKET" ]; then
        echo "socket=${NEHUANG_MYSQL_SOCKET}"
    fi
} > "$CNF_FILE"

chmod 600 "$CNF_FILE" # 保护含密码的配置文件

log_success "MySQL 配置文件已生成: $CNF_FILE"

# ----------------------------------------------------------------------------
# 构建 MySQL 客户端连接参数
# ----------------------------------------------------------------------------
# 密码为空时不传 -p 参数，避免 mysql 提示输入密码
MYSQL_BASE_ARGS=(-u"${NEHUANG_MYSQL_USER}" -h"${NEHUANG_MYSQL_HOST}" -P"${NEHUANG_MYSQL_PORT}")
if [ -n "${NEHUANG_MYSQL_PASSWORD:-}" ]; then
    MYSQL_BASE_ARGS+=(-p"${NEHUANG_MYSQL_PASSWORD}")
fi

# 检查 MySQL 连接是否正常
log_info "验证 MySQL 连接..."
if ! mysql "${MYSQL_BASE_ARGS[@]}" -e "SELECT 1" &>/dev/null; then
    log_error "无法连接 MySQL，请检查 .env 中的数据库配置"
    exit 1
fi
log_success "MySQL 连接正常"

# ----------------------------------------------------------------------------
# Step 4: 初始化数据库（建表 DDL）
# ----------------------------------------------------------------------------
log_info "Step 4/8: 初始化数据库..."

# 检查 users 表是否已存在，避免重复建表（虽然 DDL 使用 IF NOT EXISTS，
# 但显式检查可提供更好的日志体验）
TABLE_EXISTS=$(mysql "${MYSQL_BASE_ARGS[@]}" -N -e \
    "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='${NEHUANG_MYSQL_DATABASE}' AND table_name='users';" 2>/dev/null || echo "0")

if [ "$TABLE_EXISTS" -gt 0 ]; then
    log_warn "数据库已存在 users 表，跳过建表步骤"
else
    log_info "执行建表 DDL: schema/mysql/001_nehuang_v1.sql"
    mysql "${MYSQL_BASE_ARGS[@]}" < schema/mysql/001_nehuang_v1.sql
    log_success "数据库表创建完成"
fi

# ----------------------------------------------------------------------------
# Step 5: 创建管理员账号
# ----------------------------------------------------------------------------
log_info "Step 5/8: 创建管理员账号..."

# 编译 hash_password 工具
log_info "编译 hash_password 工具..."
go build -o /tmp/nehuang_hash_password ./cmd/hash_password
log_success "hash_password 工具编译完成"

# 生成 bcrypt 密码哈希
HASH=$(/tmp/nehuang_hash_password "$NEHUANG_ADMIN_PASSWORD")
if [ -z "$HASH" ]; then
    log_error "生成密码哈希失败"
    exit 1
fi
log_success "密码哈希生成完成"

# 插入管理员记录（如已存在则更新密码）
log_info "写入管理员账号 (user_id=10001, username=admin)..."
mysql "${MYSQL_BASE_ARGS[@]}" "${NEHUANG_MYSQL_DATABASE}" -e \
    "INSERT INTO users (user_id, user_type, role, username, password_hash, display_name, status, created_at, updated_at) \
     VALUES (10001, 1, 2, 'admin', '${HASH}', '管理员', 1, NOW(3), NOW(3)) \
     ON DUPLICATE KEY UPDATE password_hash = VALUES(password_hash), updated_at = NOW(3);"
log_success "管理员账号已就绪 (username=admin)"

# ----------------------------------------------------------------------------
# Step 6: 编译服务端
# ----------------------------------------------------------------------------
log_info "Step 6/8: 编译服务端..."

# 如果已有运行中的服务，先停止
if pgrep -f "nehuang-server" &>/dev/null; then
    log_warn "检测到已有 nehuang-server 进程，正在停止..."
    pkill -f "nehuang-server" || true
    sleep 2
fi

go build -o nehuang-server ./cmd/server
log_success "服务端编译完成: nehuang-server"

# ----------------------------------------------------------------------------
# Step 7: 启动服务
# ----------------------------------------------------------------------------
log_info "Step 7/8: 启动服务..."

# 设置 JWT 密钥环境变量（服务端从 env 读取）
export NEHUANG_JWT_SECRET

# 后台启动服务
nohup ./nehuang-server > server.log 2>&1 &
SERVER_PID=$!

log_info "服务进程 PID: $SERVER_PID"
log_info "日志文件: server.log"

# 等待服务启动（最多 15 秒）
log_info "等待服务启动..."
MAX_WAIT=15
WAITED=0
HEALTH_OK=false

while [ $WAITED -lt $MAX_WAIT ]; do
    # 检查进程是否存活
    if ! kill -0 "$SERVER_PID" 2>/dev/null; then
        log_error "服务进程异常退出，请检查 server.log"
        tail -20 server.log >&2 || true
        exit 1
    fi

    # 健康检查
    if curl -sf http://localhost:8080/healthz &>/dev/null; then
        HEALTH_OK=true
        break
    fi

    sleep 1
    WAITED=$((WAITED + 1))
done

# ----------------------------------------------------------------------------
# Step 8: 健康检查验证
# ----------------------------------------------------------------------------
log_info "Step 8/8: 健康检查验证..."

if [ "$HEALTH_OK" = true ]; then
    HEALTH_RESPONSE=$(curl -s http://localhost:8080/healthz)
    log_success "健康检查通过: $HEALTH_RESPONSE"
else
    log_error "服务在 ${MAX_WAIT} 秒内未响应健康检查"
    log_error "请检查 server.log:"
    tail -30 server.log >&2 || true
    exit 1
fi

# ----------------------------------------------------------------------------
# 部署结果摘要
# ----------------------------------------------------------------------------
echo ""
echo -e "${GREEN}========================================================================${NC}"
echo -e "${GREEN}  涅凰智农 IoT 云平台 — 部署成功！${NC}"
echo -e "${GREEN}========================================================================${NC}"
echo ""
echo -e "  ${BLUE}服务地址:${NC}        http://localhost:8080"
echo -e "  ${BLUE}健康检查:${NC}        http://localhost:8080/healthz"
echo -e "  ${BLUE}进程 PID:${NC}         ${SERVER_PID}"
echo -e "  ${BLUE}日志文件:${NC}        server.log"
echo -e "  ${BLUE}管理员账号:${NC}      admin / ${NEHUANG_ADMIN_PASSWORD}"
echo -e "  ${BLUE}数据库:${NC}          ${NEHUANG_MYSQL_DATABASE}@${NEHUANG_MYSQL_HOST}:${NEHUANG_MYSQL_PORT}"
echo ""
echo -e "  ${YELLOW}常用命令:${NC}"
echo -e "    查看日志:     tail -f server.log"
echo -e "    停止服务:     kill ${SERVER_PID}"
echo -e "    重启服务:     ./deploy.sh"
echo ""
