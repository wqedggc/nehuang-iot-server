#!/usr/bin/env bash
# Existing installation update. No credentials, permissions or database writes.
set -euo pipefail
cd "$(dirname "$0")"
for tool in go mysql python3; do command -v "$tool" >/dev/null || { echo "Missing tool: $tool" >&2; exit 1; }; done
[ -f .env ] && [ -f configs/mysql/nehuang_agent.cnf ] || { echo 'Existing environment and MySQL config required; initialization is separate.' >&2; exit 1; }
set -a
source .env
set +a
# Read-only schema gate. Missing migrations require a separate reviewed plan.
applied=$(mysql --defaults-extra-file=configs/mysql/nehuang_agent.cnf -N -s -e 'SELECT version FROM schema_migrations' 2>/dev/null) || { echo 'Schema preflight failed.' >&2; exit 1; }
for migration in schema/mysql/*.sql; do
  version=$(basename "$migration")
  if ! [[ $'\n'"$applied"$'\n' == *$'\n'"$version"$'\n'* ]]; then echo "Missing migration: $version; no changes performed." >&2; exit 1; fi
done
mkdir -p .deploy-build
build=$(mktemp -d "$PWD/.deploy-build/release.XXXXXX")
trap 'rm -rf "$build"' EXIT
# Both builds finish before any running process is stopped.
go build -o "$build/nehuang-server" ./cmd/server
go build -o "$build/gateway-adapter" ./cmd/gateway-adapter
python3 scripts/deploy_runtime.py "$PWD" "$build"
