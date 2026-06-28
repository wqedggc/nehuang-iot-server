#!/usr/bin/env bash
# 涅凰智农 IoT — Mock 设备事件上报脚本
#
# 用法：
#   scripts/mock_device_events.sh seed                     # 种子模式：创建云边设备/采集节点/功能点
#   scripts/mock_device_events.sh emit --farmer-user-id N  # 发射模式：仅为农户绑定的 point 生成 telemetry
#   scripts/mock_device_events.sh                          # 默认循环模式（向后兼容）
#   scripts/mock_device_events.sh --rounds 5 --interval 3  # 自定义轮次和间隔
#
# 环境变量：
#   API_BASE        默认 http://localhost:8080
#   ADAPTER_TOKEN   默认 nehuang-adapter-dev-token
#   ADMIN_USERNAME  默认 admin
#   ADMIN_PASSWORD  默认 admin123

set -euo pipefail

# ─── 默认配置 ───
API_BASE="${API_BASE:-http://localhost:8080}"
ADAPTER_TOKEN="${ADAPTER_TOKEN:-nehuang-adapter-dev-token}"
ADMIN_USERNAME="${ADMIN_USERNAME:-admin}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-admin123}"

ADAPTER_API="${API_BASE}/api/v1/adapter/device-events"
ADMIN_LOGIN_API="${API_BASE}/api/v1/admin/login"
ADMIN_BINDINGS_API="${API_BASE}/api/v1/admin/bindings"

# 默认循环参数
ROUNDS=20
INTERVAL=2

# ─── Mock 场景默认配置 ───
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

# ─── 辅助函数 ───

# 生成 telemetry 值（带小幅随机波动）
# 使用纯整数运算避免 bc 兼容性问题，输出格式为 xx.x
generate_value() {
  local func_type="$1"
  local base="$2"
  local base_int
  base_int=$(echo "$base" | awk -F. '{print $1 * 10 + $2}')
  local offset=$(( (RANDOM % 10) - 5 ))  # -5 ~ +4，即 ±0.5
  local val_int=$(( base_int + offset ))
  local int_part=$(( val_int / 10 ))
  local frac_part=$(( val_int % 10 ))
  if [ "$frac_part" -lt 0 ]; then
    frac_part=$(( -frac_part ))
  fi
  echo "${int_part}.${frac_part}"
}

# 发送单轮 device-events 请求
# 参数: $1 = message_id, $2 = events JSON 数组字符串
send_device_events() {
  local msg_id="$1"
  local events_json="$2"
  local timestamp
  timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

  local body
  body=$(cat <<EOF
{
  "protocol_version": "mock-v1",
  "source_type": "mock_script",
  "message_id": "${msg_id}",
  "gateway_name": "${GATEWAY_NAME}",
  "gateway_external_id": "${GATEWAY_EXT_ID}",
  "remote_ip": "127.0.0.1",
  "remote_port": 40000,
  "reported_at_ms": "${timestamp}",
  "events": ${events_json}
}
EOF
)

  local resp
  resp=$(curl -s -w "\n%{http_code}" -X POST "$ADAPTER_API" \
    -H "Content-Type: application/json" \
    -H "X-Adapter-Token: $ADAPTER_TOKEN" \
    -d "$body" 2>&1)

  local http_code
  http_code=$(echo "$resp" | tail -1)
  local resp_body
  resp_body=$(echo "$resp" | sed '$d')

  printf '%s|%s' "$http_code" "$resp_body"
}

# 获取 admin JWT token
admin_login() {
  local resp
  resp=$(curl -s -w "\n%{http_code}" -X POST "$ADMIN_LOGIN_API" \
    -H "Content-Type: application/json" \
    -d "{\"username\":\"${ADMIN_USERNAME}\",\"password\":\"${ADMIN_PASSWORD}\"}" 2>&1)

  local http_code
  http_code=$(echo "$resp" | tail -1)
  local resp_body
  resp_body=$(echo "$resp" | sed '$d')

  if [[ "$http_code" != "200" ]]; then
    echo "[ERROR] Admin 登录失败 (HTTP $http_code): $resp_body" >&2
    return 1
  fi

  # 提取 token
  local token
  token=$(echo "$resp_body" | jq -r '.data.token // empty')
  if [[ -z "$token" ]]; then
    echo "[ERROR] 无法从登录响应中提取 token: $resp_body" >&2
    return 1
  fi
  echo "$token"
}

# 获取指定农户的 active 绑定列表
# 参数: $1 = admin_token, $2 = farmer_user_id
# 返回 JSON 数组（每个元素含 point_id, gateway_name, frequency, device_external_id, function_type, channel_no 等）
get_farmer_bindings() {
  local token="$1"
  local farmer_id="$2"

  local resp
  resp=$(curl -s -w "\n%{http_code}" -X GET "$ADMIN_BINDINGS_API" \
    -H "Authorization: Bearer $token" \
    -H "Content-Type: application/json" 2>&1)

  local http_code
  http_code=$(echo "$resp" | tail -1)
  local resp_body
  resp_body=$(echo "$resp" | sed '$d')

  if [[ "$http_code" != "200" ]]; then
    echo "[ERROR] 获取绑定列表失败 (HTTP $http_code): $resp_body" >&2
    return 1
  fi

  # 过滤该农户的 active 绑定 (bind_status == 1)
  local bindings
  bindings=$(echo "$resp_body" | jq -c --arg uid "$farmer_id" \
    '.data[] | select(.user_id == ($uid | tonumber) and .bind_status == 1) | {
      point_id: .point_id,
      gateway_name: .gateway_name // "",
      frequency: .frequency,
      device_external_id: .device_external_id,
      function_type: .function_type,
      channel_no: .channel_no // 1
    }')

  echo "$bindings"
}

# 获取 function_type 对应的单位
function_type_unit() {
  case "$1" in
    1) echo "C" ;;
    2) echo "%" ;;
    3) echo "" ;;
    4) echo "" ;;
    5) echo "" ;;
    6) echo "" ;;
    *) echo "" ;;
  esac
}

# 获取 function_type 对应的基础值
function_type_base_value() {
  case "$1" in
    1) echo "26.5" ;;  # 温度
    2) echo "55.2" ;;  # 墒情
    3) echo "0" ;;     # 限位
    4) echo "0" ;;     # 继电器
    5) echo "0" ;;     # 水泵
    6) echo "0" ;;     # 风机
    *) echo "25.0" ;;
  esac
}

# ─── 打印帮助 ───
print_usage() {
  cat <<EOF
涅凰智农 IoT — Mock 设备事件上报脚本

用法:
  $0 seed                          种子模式：发送一次 heartbeat + telemetry，在数据库中创建设备
  $0 emit --farmer-user-id <ID>    发射模式：仅为指定农户绑定的功能点生成 telemetry 数据
  $0 [--rounds N] [--interval S]   循环模式（默认）：模拟多轮设备事件上报

选项:
  --farmer-user-id <ID>   农户 user_id（emit 模式必需）
  --rounds <N>            循环轮数（默认 20）
  --interval <S>          每轮间隔秒数（默认 2）
  --base-url <URL>        覆盖 API_BASE（默认 http://localhost:8080）
  --token <TOKEN>         覆盖 Adapter Token
  -h, --help              显示帮助

环境变量:
  API_BASE         默认 http://localhost:8080
  ADAPTER_TOKEN    默认 nehuang-adapter-dev-token
  ADMIN_USERNAME   默认 admin
  ADMIN_PASSWORD   默认 admin123
EOF
}

# ─── 子命令: seed ───
cmd_seed() {
  echo "============================================"
  echo "涅凰智农 Mock — Seed 模式"
  echo "============================================"
  echo "API:    $ADAPTER_API"
  echo "Gateway: $GATEWAY_NAME ($GATEWAY_EXT_ID)"
  echo "============================================"
  echo ""

  # 构建 events 数组：heartbeat + 所有节点所有功能点的 telemetry
  EVENTS="["
  EVENTS+='{"event_type":"heartbeat"},'

  for node_info in "${NODES[@]}"; do
    IFS='|' read -r freq devid <<< "$node_info"
    for func_info in "${FUNCTION_TYPES[@]}"; do
      IFS='|' read -r ftype unit fname <<< "$func_info"
      base_val=$(function_type_base_value "$ftype")
      val=$(generate_value "$ftype" "$base_val")
      EVENTS+="{\"event_type\":\"telemetry\",\"frequency\":${freq},\"device_external_id\":\"${devid}\",\"function_type\":${ftype},\"channel_no\":1,\"value\":${val},\"unit\":\"${unit}\"},"
    done
  done
  EVENTS="${EVENTS%,}]"

  MSG_ID="seed-$(date +%Y%m%d-%H%M%S)"
  echo "发送 seed 事件 (message_id: $MSG_ID)..."

  result=$(send_device_events "$MSG_ID" "$EVENTS")
  IFS='|' read -r http_code resp_body <<< "$result"

  if [[ "$http_code" == "200" ]]; then
    echo "✅ Seed 请求成功 (HTTP $http_code)"
    echo ""

    # 打印创建结果
    gateway_id=$(echo "$resp_body" | jq -r '.data.gateway_id // "N/A"')
    echo "────────────────────────────────────────────"
    echo "  云边设备 (Gateway)"
    echo "────────────────────────────────────────────"
    echo "  gateway_id:         $gateway_id"
    echo "  gateway_name:       $GATEWAY_NAME"
    echo "  gateway_external_id: $GATEWAY_EXT_ID"
    echo ""

    echo "────────────────────────────────────────────"
    echo "  采集执行器节点 (Collector Nodes)"
    echo "────────────────────────────────────────────"
    for node_info in "${NODES[@]}"; do
      IFS='|' read -r freq devid <<< "$node_info"
      echo "  • frequency=$freq  device_external_id=$devid"
    done
    echo ""

    echo "────────────────────────────────────────────"
    echo "  功能点 (Device Points)"
    echo "────────────────────────────────────────────"
    for node_info in "${NODES[@]}"; do
      IFS='|' read -r freq devid <<< "$node_info"
      for func_info in "${FUNCTION_TYPES[@]}"; do
        IFS='|' read -r ftype unit fname <<< "$func_info"
        echo "  • freq=$freq  devid=$devid  func=$ftype ($fname)  unit=$unit  channel=1"
      done
    done
    echo ""

    echo "============================================"
    echo "Seed 完成！以上设备和功能点已在数据库中创建。"
    echo "============================================"
  else
    echo "❌ Seed 请求失败 (HTTP $http_code)"
    echo "  响应: $resp_body"
    exit 1
  fi
}

# ─── 子命令: emit ───
cmd_emit() {
  local farmer_user_id=""

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --farmer-user-id) farmer_user_id="$2"; shift 2 ;;
      --base-url) API_BASE="$2"
        ADAPTER_API="${API_BASE}/api/v1/adapter/device-events"
        ADMIN_LOGIN_API="${API_BASE}/api/v1/admin/login"
        ADMIN_BINDINGS_API="${API_BASE}/api/v1/admin/bindings"
        shift 2 ;;
      --token) ADAPTER_TOKEN="$2"; shift 2 ;;
      -h|--help) print_usage; exit 0 ;;
      *) echo "未知参数: $1"; print_usage; exit 1 ;;
    esac
  done

  if [[ -z "$farmer_user_id" ]]; then
    echo "[ERROR] emit 模式需要 --farmer-user-id 参数" >&2
    echo "用法: $0 emit --farmer-user-id <ID>" >&2
    exit 1
  fi

  echo "============================================"
  echo "涅凰智农 Mock — Emit 模式"
  echo "============================================"
  echo "API:       $ADAPTER_API"
  echo "农户 ID:   $farmer_user_id"
  echo "============================================"
  echo ""

  # 1. Admin 登录获取 token
  echo "[1/3] Admin 登录..."
  admin_token=$(admin_login) || exit 1
  echo "  ✅ 登录成功"

  # 2. 获取该农户的 active 绑定
  echo "[2/3] 获取农户绑定信息..."
  bindings=$(get_farmer_bindings "$admin_token" "$farmer_user_id") || exit 1

  binding_count=$(echo "$bindings" | grep -c '{' 2>/dev/null || echo "0")
  if [[ "$binding_count" == "0" ]]; then
    echo ""
    echo "⚠️  该农户 (user_id=$farmer_user_id) 没有 active 绑定。"
    echo "   请先在管理后台为该农户绑定设备功能点。"
    echo ""
    exit 0
  fi
  echo "  ✅ 找到 ${binding_count} 个 active 绑定"

  # 3. 收集涉及的 gateway，为每个绑定生成 telemetry
  echo "[3/3] 生成并发送 telemetry 事件..."

  # 收集唯一的 gateway_name
  # 用换行分隔的字符串存储已见过的 gateway
  local gw_seen=$'\n'

  while IFS= read -r binding; do
    [[ -z "$binding" ]] && continue
    gw_name=$(echo "$binding" | jq -r '.gateway_name // empty')
    if [[ -n "$gw_name" ]]; then
      # 检查是否已见过
      if [[ "$gw_seen" != *$'\n'"$gw_name"$'\n'* ]]; then
        gw_seen+="${gw_name}"$'\n'
      fi
    fi
  done <<< "$bindings"

  # 提取唯一 gateway 列表
  local unique_gateways=()
  while IFS= read -r gw; do
    [[ -n "$gw" ]] && unique_gateways+=("$gw")
  done <<< "$(echo "$gw_seen" | sed '/^$/d')"

  if [[ ${#unique_gateways[@]} -eq 0 ]]; then
    echo "  ⚠️  绑定数据中没有 gateway_name，使用默认 gateway"
    unique_gateways+=("$GATEWAY_NAME")
  fi

  # 对每个 gateway 构建并发送事件
  local total_success=0
  local total_fail=0

  for gw_name in "${unique_gateways[@]}"; do
    # 筛选属于该 gateway 的绑定
    local gw_bindings
    gw_bindings=$(echo "$bindings" | jq -c --arg gn "$gw_name" 'select(.gateway_name == $gn)')

    if [[ -z "$gw_bindings" || "$gw_bindings" == "null" ]]; then
      continue
    fi

    # 查找 gateway_external_id（通过 cloud_gateways 表查询）
    # 简化：使用 gateway_name 推导 external_id
    # 如果 gateway_name 与默认一致，使用默认 external_id
    local gw_ext_id="$GATEWAY_EXT_ID"
    if [[ "$gw_name" != "$GATEWAY_NAME" ]]; then
      gw_ext_id="$gw_name"
    fi

    # 临时覆盖全局变量
    local orig_gw_name="$GATEWAY_NAME"
    local orig_gw_ext_id="$GATEWAY_EXT_ID"
    GATEWAY_NAME="$gw_name"
    GATEWAY_EXT_ID="$gw_ext_id"

    # 构建 events
    EVENTS="["
    EVENTS+='{"event_type":"heartbeat"},'

    while IFS= read -r binding; do
      [[ -z "$binding" ]] && continue

      freq=$(echo "$binding" | jq -r '.frequency // 1')
      devid=$(echo "$binding" | jq -r '.device_external_id // empty')
      ftype=$(echo "$binding" | jq -r '.function_type // 1')
      ch_no=$(echo "$binding" | jq -r '.channel_no // 1')
      unit=$(function_type_unit "$ftype")
      base_val=$(function_type_base_value "$ftype")
      val=$(generate_value "$ftype" "$base_val")

      EVENTS+="{\"event_type\":\"telemetry\",\"frequency\":${freq},\"device_external_id\":\"${devid}\",\"function_type\":${ftype},\"channel_no\":${ch_no},\"value\":${val},\"unit\":\"${unit}\"},"
    done <<< "$gw_bindings"

    EVENTS="${EVENTS%,}]"

    MSG_ID="emit-${farmer_user_id}-$(date +%Y%m%d-%H%M%S)"
    echo "  发送到 gateway \"$gw_name\"..."

    result=$(send_device_events "$MSG_ID" "$EVENTS")
    IFS='|' read -r http_code resp_body <<< "$result"

    if [[ "$http_code" == "200" ]]; then
      total_success=$((total_success + 1))
      echo "  ✅ HTTP $http_code — $MSG_ID"
    else
      total_fail=$((total_fail + 1))
      echo "  ❌ HTTP $http_code — $MSG_ID"
      echo "     响应: $resp_body"
    fi

    # 恢复
    GATEWAY_NAME="$orig_gw_name"
    GATEWAY_EXT_ID="$orig_gw_ext_id"
  done

  echo ""
  echo "============================================"
  echo "Emit 完成！成功: $total_success, 失败: $total_fail"
  echo "为农户 $farmer_user_id 的 ${binding_count} 个绑定生成了 telemetry"
  echo "============================================"
}

# ─── 子命令: loop (默认循环模式) ───
cmd_loop() {
  echo "============================================"
  echo "涅凰智农 Mock 设备事件上报"
  echo "============================================"
  echo "API:    $ADAPTER_API"
  echo "轮数:   $ROUNDS"
  echo "间隔:   ${INTERVAL}s"
  echo "============================================"

  # 基础值查找函数（替代关联数组，兼容 bash 3.2）
  get_base_value() {
    local f="$1"
    local d="$2"
    local t="$3"
    case "${f}|${d}|${t}" in
      "1|1001|1") echo "26.5" ;;  # 节点1 温度
      "1|1001|2") echo "55.2" ;;  # 节点1 墒情
      "1|1001|3") echo "0" ;;     # 节点1 限位
      "2|1002|1") echo "28.3" ;;  # 节点2 温度
      "2|1002|2") echo "62.1" ;;  # 节点2 墒情
      "2|1002|3") echo "0" ;;     # 节点2 限位
      *) echo "25.0" ;;
    esac
  }

  local success_count=0
  local fail_count=0

  for ((round=1; round<=ROUNDS; round++)); do
    MSG_ID="mock-$(date +%Y%m%d)-$(printf '%04d' "$round")"

    # 构建 events 数组
    EVENTS="["
    EVENTS+='{"event_type":"heartbeat"},'

    for node_info in "${NODES[@]}"; do
      IFS='|' read -r freq devid <<< "$node_info"
      for func_info in "${FUNCTION_TYPES[@]}"; do
        IFS='|' read -r ftype unit fname <<< "$func_info"
        key="${freq}|${devid}|${ftype}"
        base=$(get_base_value "$freq" "$devid" "$ftype")
        val=$(generate_value "$ftype" "$base")

        EVENTS+="{\"event_type\":\"telemetry\",\"frequency\":${freq},\"device_external_id\":\"${devid}\",\"function_type\":${ftype},\"channel_no\":1,\"value\":${val},\"unit\":\"${unit}\"},"
      done
    done
    EVENTS="${EVENTS%,}]"

    result=$(send_device_events "$MSG_ID" "$EVENTS")
    IFS='|' read -r http_code resp_body <<< "$result"

    if [[ "$http_code" == "200" ]]; then
      success_count=$((success_count + 1))
      echo "[第 ${round}/${ROUNDS} 轮] ✅ HTTP $http_code — $MSG_ID"
    else
      fail_count=$((fail_count + 1))
      echo "[第 ${round}/${ROUNDS} 轮] ❌ HTTP $http_code — $MSG_ID"
      echo "  响应: $resp_body"
    fi

    if [[ $round -lt $ROUNDS ]]; then
      sleep "$INTERVAL"
    fi
  done

  echo ""
  echo "============================================"
  echo "完成！成功: $success_count, 失败: $fail_count"
  echo "============================================"
}

# ─── 主入口：命令分发 ───

# 检查依赖
if ! command -v curl &>/dev/null; then
  echo "[ERROR] 缺少依赖: curl。请先安装。" >&2
  exit 1
fi
if ! command -v jq &>/dev/null; then
  echo "[ERROR] 缺少依赖: jq。请先安装。" >&2
  exit 1
fi

# 第一个参数决定子命令
case "${1:-}" in
  seed)
    shift
    # seed 模式下也可以覆盖 --base-url / --token
    while [[ $# -gt 0 ]]; do
      case "$1" in
        --base-url) API_BASE="$2"
          ADAPTER_API="${API_BASE}/api/v1/adapter/device-events"
          shift 2 ;;
        --token) ADAPTER_TOKEN="$2"; shift 2 ;;
        -h|--help) print_usage; exit 0 ;;
        *) echo "未知参数: $1"; print_usage; exit 1 ;;
      esac
    done
    cmd_seed
    ;;

  emit)
    shift
    cmd_emit "$@"
    ;;

  -h|--help)
    print_usage
    exit 0
    ;;

  *)
    # 解析可选参数（循环模式 / 向后兼容）
    while [[ $# -gt 0 ]]; do
      case "$1" in
        --base-url) API_BASE="$2"
          ADAPTER_API="${API_BASE}/api/v1/adapter/device-events"
          shift 2 ;;
        --token)    ADAPTER_TOKEN="$2"; shift 2 ;;
        --rounds)   ROUNDS="$2"; shift 2 ;;
        --interval) INTERVAL="$2"; shift 2 ;;
        -h|--help)  print_usage; exit 0 ;;
        *) echo "未知参数: $1"; print_usage; exit 1 ;;
      esac
    done
    cmd_loop
    ;;
esac
