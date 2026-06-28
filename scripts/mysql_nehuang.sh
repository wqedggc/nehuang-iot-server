#!/usr/bin/env bash
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEFAULT_CREDENTIALS_FILE="$PROJECT_ROOT/configs/mysql/nehuang_agent.cnf"
CREDENTIALS_FILE="${NEHUANG_MYSQL_DEFAULTS_FILE:-$DEFAULT_CREDENTIALS_FILE}"

usage() {
  cat <<'USAGE'
涅凰智农 MySQL 登录脚本

用法：
  scripts/mysql_nehuang.sh
  scripts/mysql_nehuang.sh -e "SHOW TABLES;"
  scripts/mysql_nehuang.sh < schema/mysql/001_nehuang_v1.sql

默认凭证：
  configs/mysql/nehuang_agent.cnf

可选环境变量：
  NEHUANG_MYSQL_DEFAULTS_FILE   指定其他 MySQL defaults-extra-file

首次使用：
  scripts/init_mysql_nehuang.sh
USAGE
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

if [[ ! -f "$CREDENTIALS_FILE" ]]; then
  echo "missing MySQL credentials file: $CREDENTIALS_FILE" >&2
  echo "请先执行 scripts/init_mysql_nehuang.sh 创建 nehuang_db 和专用账号。" >&2
  exit 1
fi

mysql --defaults-extra-file="$CREDENTIALS_FILE" "$@"
