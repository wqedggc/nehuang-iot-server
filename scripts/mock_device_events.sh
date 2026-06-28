#!/usr/bin/env bash
# 涅凰智农 IoT — Mock 设备事件上报脚本
#
# 用法：
#   scripts/mock_device_events.sh \
#     --base-url http://127.0.0.1:8080 \
#     --token "$NEHUANG_ADAPTER_TOKEN" \
#     --rounds 20 \
#     --interval 2
#
# 功能：模拟 1 个云边设备 + 2 个采集执行器节点，
#       每个节点含温度、墒情、限位 3 个功能点，
#       每轮发送 heartbeat + telemetry，连续多轮形成历史曲线。

set -euo pipefail

# ─── 默认参数 ───
BASE_URL="http://127.0.0.1:8080"
TOKEN="nehuang-adapter-dev-token"
ROUNDS=20
INTERVAL=2

# ─── 解析命令行参数 ───
while [[ $# -gt 0 ]]; do
  case "$1" in
    --base-url) BASE_URL="$2"; shift 2 ;;
    --token)    TOKEN="$2"; shift 2 ;;
    --rounds)   ROUNDS="$2"; shift 2 ;;
    --interval) INTERVAL="$2"; shift 2 ;;
    *) echo "未知参数: $1"; exit 1 ;;
  esac
done

API="${BASE_URL}/api/v1/adapter/device-events"

echo "============================================"
echo "涅凰智农 Mock 设备事件上报"
echo "============================================"
echo "API:    $API"
echo "轮数:   $ROUNDS"
echo "间隔:   ${INTERVAL}s"
echo "============================================"

# ─── Mock 场景配置 ───
GATEWAY_NAME="FP_TEST_ADMIN_001"
GATEWAY_EXT_ID="TEST_ADMIN_001"

# 2 个采集执行器节点
NODES=(
  "1|1001"  # frequency=1, device_external_id=1001
  "2|1002"  # frequency=2, device_external_id=1002
)

# 每个节点的 3 个功能点
FUNCTION_TYPES=(
  "1|C|温度"   # function_type=1, unit=C
  "2|%|墒情"   # function_type=2, unit=%
  "3||限位"    # function_type=3, unit=空
)

# ─── 生成 telemetry 值（带小幅随机波动） ───
generate_value() {
  local func_type="$1"
  local base="$2"
  # 随机波动 ±5%
  local delta=$(echo "scale=2; ($RANDOM % 100 - 50) / 1000" | bc 2>/dev/null || echo "0")
  echo "$base + $delta" | bc 2>/dev/null || echo "$base"
}

# 基础值
declare -A BASE_VALUES
BASE_VALUES["1|1001|1"]="26.5"   # 节点1 温度
BASE_VALUES["1|1001|2"]="55.2"   # 节点1 墒情
BASE_VALUES["1|1001|3"]="0"      # 节点1 限位
BASE_VALUES["2|1002|1"]="28.3"   # 节点2 温度
BASE_VALUES["2|1002|2"]="62.1"   # 节点2 墒情
BASE_VALUES["2|1002|3"]="0"      # 节点2 限位

success_count=0
fail_count=0

for ((round=1; round<=ROUNDS; round++)); do
  MSG_ID="mock-$(date +%Y%m%d)-$(printf '%04d' $round)"
  TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

  # 构建 events 数组
  EVENTS="["
  # 先加一个 heartbeat
  EVENTS+='{"event_type":"heartbeat"},'

  # 为每个节点、每个功能点生成 telemetry
  for node_info in "${NODES[@]}"; do
    IFS='|' read -r freq devid <<< "$node_info"
    for func_info in "${FUNCTION_TYPES[@]}"; do
      IFS='|' read -r ftype unit fname <<< "$func_info"
      key="${freq}|${devid}|${ftype}"
      base="${BASE_VALUES[$key]:-25.0}"
      val=$(generate_value "$ftype" "$base")

      EVENTS+="{\"event_type\":\"telemetry\",\"frequency\":${freq},\"device_external_id\":\"${devid}\",\"function_type\":${ftype},\"channel_no\":1,\"value\":${val},\"unit\":\"${unit}\"},"
    done
  done
  EVENTS="${EVENTS%,}]"

  # 构建完整请求体
  BODY=$(cat <<EOF
{
  "protocol_version": "mock-v1",
  "source_type": "mock_script",
  "message_id": "${MSG_ID}",
  "gateway_name": "${GATEWAY_NAME}",
  "gateway_external_id": "${GATEWAY_EXT_ID}",
  "remote_ip": "127.0.0.1",
  "remote_port": 40000,
  "reported_at_ms": "${TIMESTAMP}",
  "events": ${EVENTS}
}
EOF
)

  # 发送请求
  RESP=$(curl -s -w "\n%{http_code}" -X POST "$API" \
    -H "Content-Type: application/json" \
    -H "X-Adapter-Token: $TOKEN" \
    -d "$BODY" 2>&1)

  HTTP_CODE=$(echo "$RESP" | tail -1)
  RESP_BODY=$(echo "$RESP" | sed '$d')

  if [[ "$HTTP_CODE" == "200" ]]; then
    success_count=$((success_count + 1))
    echo "[第 ${round}/${ROUNDS} 轮] ✅ HTTP $HTTP_CODE — $MSG_ID"
  else
    fail_count=$((fail_count + 1))
    echo "[第 ${round}/${ROUNDS} 轮] ❌ HTTP $HTTP_CODE — $MSG_ID"
    echo "  响应: $RESP_BODY"
  fi

  # 间隔等待
  if [[ $round -lt $ROUNDS ]]; then
    sleep "$INTERVAL"
  fi
done

echo ""
echo "============================================"
echo "完成！成功: $success_count, 失败: $fail_count"
echo "============================================"
