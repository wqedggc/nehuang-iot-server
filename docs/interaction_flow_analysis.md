# 涅凰智农 IoT — 交互流程说明书 & 问题分析

> 生成日期：2026-07-05  
> 分析范围：`nehuang-iot-server` 全部 Go 源码、Web 前端、文档、Schema  
> 分析者：齐活林（交付总监）

---

## 一、系统架构全景

```
┌──────────────────────────────────────────────────────────────┐
│                    涅凰智农 IoT 云平台 V1                       │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌──────────┐    TCP(:8081)     ┌───────────────────┐        │
│  │ 硬件网关  │ ────────────────> │  gateway-adapter  │        │
│  │ (TCP客户端)│                  │  (独立 Go 进程)    │        │
│  └──────────┘                   │                   │        │
│                                 │  - 收 raw bytes   │        │
│                                 │  - 写 raw journal │        │
│                                 │  - 最小协议分类    │        │
│                                 │  - 拉取/下发命令   │        │
│                                 └────────┬──────────┘        │
│                                          │                   │
│                           POST /api/v1/adapter/*             │
│                           (X-Adapter-Token 认证)              │
│                                          │                   │
│                                 ┌────────▼──────────┐        │
│                                 │  nehuang-server    │        │
│                                 │  (独立 Go 进程)    │        │
│                                 │  HTTP :8080        │        │
│                                 │                    │        │
│                                 │  ┌──────────────┐ │        │
│                                 │  │ Admin H5 页面  │ │        │
│                                 │  │ /admin/       │ │        │
│                                 │  ├──────────────┤ │        │
│                                 │  │ Mini H5 页面   │ │        │
│                                 │  │ /mini/        │ │        │
│                                 │  ├──────────────┤ │        │
│                                 │  │ REST API      │ │        │
│                                 │  │ /api/v1/*     │ │        │
│                                 │  └──────┬───────┘ │        │
│                                 └─────────┼─────────┘        │
│                                           │                  │
│                                    ┌──────▼──────┐           │
│                                    │   MySQL      │           │
│                                    │  nehuang_db  │           │
│                                    └─────────────┘           │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐    │
│  │  微信小程序 (Taro, 独立仓库 shoreos-taro)              │    │
│  │  通过 /api/v1/mini/* 与服务端交互                      │    │
│  └──────────────────────────────────────────────────────┘    │
└──────────────────────────────────────────────────────────────┘
```

### 双进程职责

| 进程 | 端口 | 职责 |
|------|------|------|
| `nehuang-server` | `:8080` (HTTP) | REST API、Admin/Mini H5、MySQL 业务读写、离线扫描、Dashboard |
| `gateway-adapter` | `:8081` (TCP)、`:18081` (Health) | 硬件 TCP 接入、raw journal、协议分类、命令拉取/下发、raw frame 转发 |

---

## 二、四大交互流程

### 2.1 上行数据流（硬件 → 云端）

```
硬件网关 TCP ──bytes──> gateway-adapter :8081
                         │
                         ├──> raw frame journal (本地 JSONL 文件)
                         │
                         ├──> 协议分类 (ClassifyPayload)
                         │    ├── FP_xxx → MessageRegister
                         │    ├── HEARTBEAT/HB/PING → MessageHeartbeat
                         │    ├── AA AA ... CRC → MessageWorkData
                         │    ├── AA AA ... FF FF → MessageAlarm
                         │    └── 其他 → MessageUnknown
                         │
                         └──> POST /api/v1/adapter/raw-frames ──> nehuang-server
                              │
                              ├──> 写入 raw_device_events 表 (parse_status=pending)
                              │
                              ├──> parseRawFrame() 最小解析
                              │    ├── register → UpsertGatewayByExternalID (刷新在线状态)
                              │    ├── heartbeat → UpsertGatewayByExternalID (刷新在线状态)
                              │    └── 其他 → 保持 pending (不解析)
                              │
                              └──> 返回 {raw_event_id, parse_status}
```

**详细步骤：**

1. **硬件网关** 作为 TCP Client 连接 `gateway-adapter :8081`
2. **gateway-adapter** 收到 bytes 后：
   - 生成 `connection_id`（随机 8 字节 hex）
   - 立即写入 raw frame journal（本地 `data/gateway-adapter/raw-frames/raw-frames-YYYYMMDD.jsonl`）
   - 调用 `protocol.ClassifyPayload()` 做最小分类
   - 构造 `RawFrameEnvelope` JSON，POST 到 `nehuang-server /api/v1/adapter/raw-frames`
   - 如果识别到注册包 → 记录 `gatewayName`，提取 `gatewayExternalID`
   - 如果识别到 `gatewayExternalID` → 启动 `startPendingCommandLoop`（定时拉取待下发命令）
3. **nehuang-server** `handleRawFrames` 收到后：
   - 校验 `X-Adapter-Token`
   - 解码 `payload_hex`
   - 写 `raw_device_events` 表（`parse_status=pending`）
   - 调用 `parseRawFrame()`：
     - 注册包 → `UpsertGatewayByExternalID`（刷新在线）→ `parse_status=parsed`
     - 心跳包 → `UpsertGatewayByExternalID`（刷新在线）→ `parse_status=parsed`
     - 其他 → `parse_status=pending`（不解析，等真实协议稳定）
   - 返回 `{raw_event_id, parse_status}`

### 2.2 下行控制流（云端 → 硬件）

```
Admin/Mini 控制操作
     │
     ▼
POST /api/v1/admin/device-points/{point_id}/control
或 POST /api/v1/mini/devices/{point_id}/control
     │
     ├──> 校验权限 (JWT auth + 绑定关系)
     ├──> buildControlCommandInput (action → command_hex)
     │    ├── open/start → hex "F0"
     │    ├── close → hex "0F"
     │    └── stop → hex "00"
     ├──> CreateControlCommandForPoint → control_commands (status=created/pending)
     └──> 返回 {command_id, status:"pending"}
     
     
          gateway-adapter 命令轮询 (每 2s)
                    │
                    ▼
          GET /api/v1/adapter/commands/pending?gateway_external_id=xxx
                    │
                    ├──> ListPendingControlCommands (WHERE status=created)
                    │
                    ▼
          逐个处理 pending commands:
                    │
                    ├──> decodeCommandHex → []byte
                    ├──> conn.Write(payload) → TCP 下发到硬件网关
                    ├──> POST /api/v1/adapter/commands/{id}/sent (标记已下发)
                    │
                    ▼
          (未来) 硬件 MCU 返回 ACK/结果
                    │
                    ▼
          POST /api/v1/adapter/commands/{id}/result
                    │
                    ├──> ApplyControlCommandResult
                    │    ├──> UPDATE control_commands (status=ack/succeeded/failed/timeout)
                    │    └──> INSERT control_results
                    └──> 返回 {command_id, status}
```

### 2.3 Admin 管理端交互流

```
浏览器 → /admin/ (H5 SPA, 嵌入二进制)
    │
    ├──> POST /api/v1/admin/login (用户名+密码 → JWT token, aud=admin)
    │
    ├──> [Dashboard] GET /api/v1/admin/dashboard?range=1d
    │
    ├──> [用户管理] GET /api/v1/admin/users
    │
    ├──> [农户管理]
    │    ├── GET /api/v1/admin/farmers?keyword=
    │    ├── GET /api/v1/admin/farmers/{id}
    │    ├── PATCH /api/v1/admin/farmers/{id}
    │    └── GET /api/v1/admin/farmers/{id}/bindings
    │
    ├──> [设备管理]
    │    ├── GET /api/v1/admin/device-tree (网关→节点→点位 三层)
    │    ├── GET /api/v1/admin/device-points?keyword=&bind_status=
    │    ├── PATCH /api/v1/admin/device-points/{id} (配置点位)
    │    └── POST /api/v1/admin/device-points/{id}/control (控制)
    │
    ├──> [绑定管理]
    │    ├── GET /api/v1/admin/bindings
    │    ├── POST /api/v1/admin/bindings (binding_type=gateway/point)
    │    ├── DELETE /api/v1/admin/bindings/{id}
    │    └── GET /api/v1/admin/gateway-bind-candidates?keyword=
    │
    ├──> [报文排查]
    │    ├── GET /api/v1/admin/payload-events (分页, 筛选)
    │    └── GET /api/v1/admin/payload-events/{id} (含协议摘要)
    │
    ├──> [控制日志] GET /api/v1/admin/control-logs
    │
    └──> [原始事件] GET /api/v1/admin/raw-events
```

### 2.4 Mini 农户端交互流

```
浏览器/微信小程序 → /mini/ (H5 SPA, 嵌入二进制)
    │
    ├──> POST /api/v1/mini/login (微信 code → JWT token, aud=mini)
    │    或 POST /api/v1/mini/mock-login (仅开发环境)
    │
    ├──> GET /api/v1/mini/me (查看/编辑个人资料)
    ├──> PATCH /api/v1/mini/me
    │
    ├──> [设备列表] GET /api/v1/mini/devices (仅返回已绑定点位)
    │
    ├──> [实时数据] GET /api/v1/mini/devices/{point_id}/realtime
    │
    ├──> [历史曲线] GET /api/v1/mini/devices/{point_id}/history?range=1d
    │
    └──> [设备控制] POST /api/v1/mini/devices/{point_id}/control
         (仅支持 start/open, close, stop)
```

---

## 三、关键数据模型

```
cloud_gateways (设备网关)
    │  gateway_external_id (硬件唯一标识, 如 FP_xxx 去掉前缀)
    │  online_status (1=在线/0=离线, 由心跳/离线扫描维护)
    │
    └──> collector_nodes (采集执行器)
         │  唯一身份: gateway_id + frequency + device_external_id
         │
         └──> device_points (功能点)
              唯一身份: node_id + function_type + channel_no
              function_type: 1=温度 2=墒情 3=限位 4=继电器 5=水泵 6=风机

bind_relations (绑定关系)
    user_id → gateway_id (网关级绑定, 主流)
    user_id → point_id (点位级绑定, 兼容)

control_commands (控制命令)
    status: 1=created 2=sent 3=ack 4=succeeded 5=failed 6=timeout

raw_device_events (原始报文)
    parse_status: 1=pending 2=parsed 3=failed 4=partial 5=ignored
    event_type: 1=register 2=heartbeat 3=work 4=control_result 5=json 6=alarm 7=unknown
```

---

## 四、认证体系

| 接口范围 | 认证方式 | 说明 |
|----------|----------|------|
| `/healthz`, `/readyz` | 无 | 健康检查 |
| `/api/v1/admin/login` | 无 | Admin 登录 |
| `/api/v1/mini/login` | 无 | 微信小程序登录 |
| `/api/v1/mini/mock-login` | 无 | 仅开发环境可用 |
| `/api/v1/admin/*` | JWT `aud=admin` | Admin token, 角色 super_admin/admin |
| `/api/v1/mini/*` | JWT `aud=mini` | 农户 token, 角色 farmer |
| `/api/v1/adapter/*` | `X-Adapter-Token` header | Adapter 内部接口 |
| `/api/v1/gateways` | JWT `aud=admin` | Admin 可访问 |

**aud 隔离**：`authMiddleware` 强制校验 aud 字段：
- `/api/v1/admin/*` 只接受 `aud=admin`
- `/api/v1/mini/*` 只接受 `aud=mini`
- 交叉使用返回 403

---

## 五、当前仍有效的交互风险

本章只保留当前仍需要决策或实现的事项。

### 5.1 工作包、告警包仍只保留原始报文

当前 `gateway-adapter` 可以稳定接入 TCP bytes，`nehuang-server` 可以写入 `raw_device_events`，并能识别注册、心跳、Modbus 读寄存器摘要等基础类型。

仍未进入业务库的内容：

- 真实工作包字段解析。
- 真实告警包字段解析与去重。
- 上行 6 字节时分秒字段解析。
- 按硬件实际采样时间反填 `sensor_data`。
- 工作包解析后的 `device_realtime_maps` 投影。

影响：

- Mini 历史曲线仍不能依赖真实硬件工作包生成。
- 告警仍不能进入正式告警业务和通知。
- 原始报文排查是当前定位真实协议的主要入口。

### 5.2 ACK、清缓存和执行结果语义未冻结

当前下行链路已经具备基础闭环：

```text
Admin/Mini 创建控制命令
  -> control_commands(status=created)
  -> gateway-adapter 拉取 pending
  -> TCP 写回 command_hex
  -> control_commands(status=sent)
  -> 超过 TTL 未回执则 timeout
```

仍未实现：

- MCU ACK 包解析。
- ACK 与 `succeeded/failed` 的映射。
- 读取缓存成功后的清缓存命令。
- 清缓存命令的 ACK 与重试策略。
- 硬件返回包和具体 `command_id` 的关联规则。

当前控制日志可以展示 `pending/sent/timeout`，但不能承诺硬件已经执行成功。

### 5.3 TDMA 时间同步字段尚未进入协议实现

硬件端最新口径已经明确：200 秒时隙表盘由硬件侧新增 MCU 维护，云端不再按公网延迟卡 2 秒窗口。

云端后续新增职责：

- 下行包在包头地址信息后、正式内容前增加 6 字节时分秒。
- 上行包在地址/频点 ID 后、正式内容前解析 6 字节时分秒。
- 按该时间字段倒排写入业务数据。

仍需硬件端确认：

- 6 字节字段编码格式。
- 时区是否固定北京时间。
- 字段是否参与 CRC。
- 字段插入后，工作包长度和 CRC 覆盖范围是否变化。

### 5.4 Mini 与 Admin 的控制入口职责不同

当前产品口径：

- Admin 支持自定义 `command_hex`，用于联调和管理员高级操作。
- Mini 只暴露 `start/open/close/stop` 等明确动作，不开放自定义 Hex。

这是当前的角色边界，不作为 Bug。后续如果 C 端也需要临时读取、清缓存、同步时间等操作，需要先定义哪些操作允许农户触发，再增加模板化按钮，而不是直接开放任意 Hex。

### 5.5 TCP 连接依赖硬件心跳维持

`gateway-adapter` 的下行命令写回依赖硬件网关保持同一条 TCP 连接可写。硬件端如果长时间静默，连接可能被读超时关闭，Adapter 也就无法写回 pending command。

当前工程口径：

- 硬件端应持续发送心跳或业务包维持连接。
- 服务端已有命令 timeout 兜底，连接不可写时命令不会被误标成功。
- 若硬件端心跳周期未来大于 Adapter read timeout，需要调整 Adapter 超时配置。

### 5.6 生产环境 CORS 仍需收敛

当前 HTTP 中间件仍允许宽松跨域。联调阶段可以接受，但正式域名、微信小程序域名和 Admin 域名确定后，应改成白名单。

## 六、当前交互结论

- 注册网关、原始报文留痕、农户绑定设备网关、联调点位创建、Mini 展示绑定功能点、Admin/Mini 创建异步控制命令、Adapter 下行写回、控制命令超时兜底，这条联调主链路已经具备。
- 真实业务数据闭环仍卡在硬件协议：工作包、告警包、ACK、清缓存、6 字节时分秒字段。
- 后续开发优先级应先落真实 parser 和 ACK/result 语义，再做更完整的告警通知、历史曲线和 C 端控制体验。
