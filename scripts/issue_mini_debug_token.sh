#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

TTL_HOURS=720
USER_ID=""
LIST_FARMERS=false

usage() {
  cat <<'EOF'
Usage:
  scripts/issue_mini_debug_token.sh --list-farmers
  scripts/issue_mini_debug_token.sh --user-id <farmer_user_id> [--ttl-hours <hours>]

Examples:
  scripts/issue_mini_debug_token.sh --list-farmers
  scripts/issue_mini_debug_token.sh --user-id 329722558708453376 --ttl-hours 8760

Notes:
  - This signs a browser-debug mini token locally with NEHUANG_JWT_SECRET.
  - It does not call mock-login and does not create users.
  - Default TTL is 720 hours (30 days).
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --list-farmers)
      LIST_FARMERS=true
      shift
      ;;
    --user-id)
      USER_ID="${2:-}"
      shift 2
      ;;
    --ttl-hours)
      TTL_HOURS="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ -f .env ]]; then
  set -a
  # shellcheck disable=SC1091
  source .env
  set +a
fi

if [[ -z "${NEHUANG_JWT_SECRET:-}" ]]; then
  echo "NEHUANG_JWT_SECRET is required. Run this inside the deployed repo or export it first." >&2
  exit 1
fi

if [[ "$TTL_HOURS" == "" || ! "$TTL_HOURS" =~ ^[0-9]+$ || "$TTL_HOURS" -le 0 ]]; then
  echo "--ttl-hours must be a positive integer" >&2
  exit 2
fi

if [[ "$LIST_FARMERS" == true ]]; then
  if [[ -x scripts/mysql_nehuang.sh ]]; then
    scripts/mysql_nehuang.sh -e "SELECT user_id, COALESCE(display_name, username, '') AS display_name, status FROM users WHERE user_type=2 AND role=3 ORDER BY updated_at DESC LIMIT 50;"
    exit 0
  fi
  echo "scripts/mysql_nehuang.sh is required for --list-farmers" >&2
  exit 1
fi

if [[ -z "$USER_ID" || ! "$USER_ID" =~ ^[0-9]+$ ]]; then
  echo "--user-id <farmer_user_id> is required" >&2
  usage >&2
  exit 2
fi

export USER_ID TTL_HOURS
python3 - <<'PY'
import base64
import hashlib
import hmac
import json
import os
import time

secret = os.environ["NEHUANG_JWT_SECRET"].encode("utf-8")
now_ms = int(time.time() * 1000)
ttl_hours = int(os.environ["TTL_HOURS"])
claims = {
    "user_id": int(os.environ["USER_ID"]),
    "user_type": 2,
    "role": 3,
    "aud": "mini",
    "iat": now_ms,
    "exp": now_ms + ttl_hours * 60 * 60 * 1000,
}

def b64url(data: bytes) -> str:
    return base64.urlsafe_b64encode(data).decode("ascii").rstrip("=")

header = b64url(json.dumps({"alg": "HS256", "typ": "JWT"}, separators=(",", ":")).encode("utf-8"))
payload = b64url(json.dumps(claims, separators=(",", ":")).encode("utf-8"))
signing_input = f"{header}.{payload}".encode("ascii")
signature = b64url(hmac.new(secret, signing_input, hashlib.sha256).digest())
print(f"{header}.{payload}.{signature}")
PY
