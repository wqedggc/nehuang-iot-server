#!/usr/bin/env bash
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONFIG_DIR="$PROJECT_ROOT/configs/mysql"
CREDENTIALS_FILE="$CONFIG_DIR/nehuang_agent.cnf"

DB_NAME="${NEHUANG_DB_NAME:-nehuang_db}"
APP_USER="${NEHUANG_DB_USER:-nehuang_agent}"
ADMIN_USER="${NEHUANG_MYSQL_ADMIN_USER:-root}"
MYSQL_HOST="${NEHUANG_MYSQL_HOST:-127.0.0.1}"
MYSQL_PORT="${NEHUANG_MYSQL_PORT:-3306}"

sql_escape() {
  printf "%s" "$1" | sed "s/'/''/g"
}

mkdir -p "$CONFIG_DIR"

if [[ -z "${NEHUANG_DB_PASSWORD:-}" ]]; then
  printf "请输入将要创建的 MySQL 专用账号 %s 的密码: " "$APP_USER" >&2
  stty -echo
  read -r APP_PASSWORD
  stty echo
  printf "\n" >&2
else
  APP_PASSWORD="$NEHUANG_DB_PASSWORD"
fi

if [[ -z "$APP_PASSWORD" ]]; then
  echo "password must not be empty" >&2
  exit 1
fi

ESCAPED_PASSWORD="$(sql_escape "$APP_PASSWORD")"

mysql --protocol=TCP -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u "$ADMIN_USER" -p <<SQL
CREATE DATABASE IF NOT EXISTS \`$DB_NAME\`
  DEFAULT CHARACTER SET utf8mb4
  DEFAULT COLLATE utf8mb4_0900_ai_ci;

CREATE USER IF NOT EXISTS '$APP_USER'@'localhost'
  IDENTIFIED BY '$ESCAPED_PASSWORD';

ALTER USER '$APP_USER'@'localhost'
  IDENTIFIED BY '$ESCAPED_PASSWORD';

GRANT SELECT, INSERT, UPDATE, DELETE, CREATE, REFERENCES, INDEX, ALTER
  ON \`$DB_NAME\`.* TO '$APP_USER'@'localhost';

FLUSH PRIVILEGES;
SQL

cat > "$CREDENTIALS_FILE" <<EOF
[client]
user=$APP_USER
password=$APP_PASSWORD
host=$MYSQL_HOST
port=$MYSQL_PORT
database=$DB_NAME
default-character-set=utf8mb4
EOF

chmod 600 "$CREDENTIALS_FILE"

echo "created credentials file: $CREDENTIALS_FILE"
echo "next: scripts/mysql_nehuang.sh < schema/mysql/001_nehuang_v1.sql"

