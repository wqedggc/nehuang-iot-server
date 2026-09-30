# 2026-10-02 一期硬件联调验收

核对日期：2026-09-30。范围为现有用户平台硬件联调；付款后再迁移甲方腾讯云。本次未部署、迁移、推送、连接真实设备或发送控制；证书和云端权限未修改。

## 当前代码事实

| 能力 | 已有证据 | 缺口/验收边界 |
|---|---|---|
| TCP 原始字节留存 | `internal/gatewayadapter/server.go`、`journal/` 与相应测试；每次 TCP Read 写 journal，转发 raw-frames，失败重试 | TCP Read 不保证一帧；真实半包/粘包协议重组需确认，不应把一次Read直接视为完整MCU消息 |
| 管理端原始报文 | `#payload-events`、GET `/api/v1/admin/payload-events`，默认 source_type=1 | 结构化 device-events 可以没有 payload_hex，不是数据丢失；线上数据库/版本尚未核验 |
| 操作审计 | `#operation-logs`、GET `/api/v1/admin/operation-logs`、`op_records` | 两页离线空响应均可打开；实际用户后台故障未复现，不能宣称根治 |
| 一期协议解释 | CRC/Tag 与旧MCU1字段摘要，原始值和未知温度编码分别显示 | 新MCU0上行04/05未支持；自动历史只识别10或45/48/51/54字节；温度真实解析入库/完整ACK闭环待确认和实现 |
| 参数编辑 | 功能点弹窗可改名称、单位、上下阈值、模式、轮询间隔、poll_cmd、备注；PATCH device-points | 保存参数不等于MCU收到阈值；尚无证据证明定时采样/上传或温控闭环已实现；指标类别仍固定枚举，七组含义扩展尚未交付 |
| 下行入口 | 管理后台“设备管理”选网关/采集执行器，功能点“发送下行Hex指令”；`openPointControlModal` | 提交创建异步命令；本次未点击发送。Hex模板与协议确认、设备回执必须核验 |
| 下行状态 | Admin POST `/api/v1/admin/device-points/{point_id}/control`→control_commands pending→Adapter拉取/写TCP→sent；控制日志详情与原始报文关联 | sent仅表示写TCP，不等同执行；真实ACK归属与succeeded条件、重试/超时需联调验证 |

## 本次本地修改及验证

`web/static/admin/app.js` 仅修改操作日志、原始报文两页：显示 HTML 转义后的错误信息；响应缺少 data 时报错，原始报文不再误显示为空列表。5 行新增、4 行删除。没有猜字段或新增二期逻辑。

新增明确缺陷修复：Adapter在同一连接成功写TCP后、sent回写暂时失败时，仅重试状态回写，避免持续pending导致重复TCP发送。新增测试原代码发送3次、修复后1次，状态回写3次后恢复。新增raw API测试确认CRC损坏和未知包原始hex/信封/capture身份准确保留，不投影猜测数据。相关4个包竞态测试通过。ACK后TTL与迟到终态覆盖已按既有数据模型修复；跨连接执行幂等及硬件回应关联仍待确认，见 [本机检查](hardware_local_preflight.md)。

- 修改前：离线空数据，两页均渲染正常，未复现用户平台打不开。
- 修改后：6 个 JSDOM 场景通过（两页分别测试空列表、缺data、接口错误与script字符串转义），原始报文默认请求 source_type=1。
- `node --check web/static/admin/app.js` 通过。
- `go test ./...` 在允许本机临时 mock 监听的离线环境通过；首次受沙箱限制无法bind，未作为代码失败处理。测试不使用真实微信/硬件/云端。
- 修改后 `go test ./web ./internal/repository/mysql ./internal/protocolsummary ./internal/gatewayadapter/protocol` 通过；`git diff --check` 通过。
- 真浏览器本地 file 页面被浏览器URL安全策略拒绝，未尝试绕过。JSDOM 不代表真实浏览器或线上数据库验收。

## 10/1–10/2 验收清单

以下为待执行验收，不因代码测试通过而自动勾选。

- [ ] 确认使用现有用户试验平台、后端及Adapter版本，Admin凭据合法有效，两页接口200及data分页形态正确；若失败记录URL/状态/错误，不泄露token。
- [ ] 硬件注册/心跳身份一致，网关在线，原始字节在journal与source_type=1列表/详情可查，CRC错误/未知包仍保留。
- [ ] 人工确认最新MCU0协议采样/上传周期、编码、ACK例外、TCP分帧；取得带CRC的真实样例。
- [ ] 一个温度包解析为正确设备、时间、单位和值，原始包可反查；正/负温、跨午夜、无历史/1/12/15组、重复上报覆盖；入库后才能回复清缓存ACK。
- [ ] 配置阈值保存、正确组包、下发到目标MCU、回执归属、运行状态反馈完整；核对云端参数和硬件实际生效值。
- [ ] 手动开/关/停及限位/故障拒绝、故障恢复04、阈值状态05，控制日志能区分pending/sent/ack/succeeded/failed/timeout。
- [ ] 断网重连、中心API不可用、raw重试/幂等、控制超时、半包/粘包回放不丢数据、不重复副作用。
- [ ] 指标名称/单位/阈值可编辑；二期七组指标只确认扩展边界，具体逻辑另行提供。
- [ ] 付款后迁移阶段单独处理甲方SSL与权限，本次不执行。

## 需甲方确认

详见 [MCU0阅读版](protocols/mcu0-0910/reading.md)“时序与确认项”。优先：4/5分钟采样和即时/批次上传、200/240秒边缘表盘、6字节ACK是否免CRC、2字节阈值/温度的端序及符号、手动温度宽度、时间编码/日期、历史序号与清缓存幂等。

## 历史入口只读记录（已停止调查）

用户最新明确此入口是甲方环境问题，本次不再调查HTTP/HTTPS、证书或管理员会话；它们不作为用户自己环境开发阻塞。以下仅保留历史证据，不认定IP属于用户环境。

仓库 `docs/gateway_adapter.md:81` 明确入口为 `http://43.143.208.153:8080/admin/`。在用户 Mac 的 Chrome 打开 `#operation-logs`，标题和导航加载，操作日志显示“加载失败”；切换到原始报文排查后跳转 `#login` 并提示“无管理员权限，请使用管理员账号登录”。没有读取 token、输入凭据或尝试登录。当时缺少有效管理员会话使受保护接口无法继续核验，不能据此断定最初失败唯一由认证引起。

公开只读请求：`/healthz` 200，`/admin/app.js` 200；`/version.js` 显示 0.6.0（20260827）。不带凭据请求操作日志/原始报文接口均401，符合保护路由预期，不代表带合法凭据的接口已验收。线上 `/admin/app.js` SHA-256 为 `9cc88c967a302c34c585a1776e822a762526a005ba56aefa89d862e9936b6eed`，与本次修复前本地文件完全相同；版本标签不能单独证明服务端 commit 或数据库迁移状态。尚未读取线上日志/数据库，未改变任何云端状态。

## 无网络离线回放步骤

在项目根目录运行以下命令，仅写临时测试文件并读取分类结果；不启动server/adapter、不读取.env、不访问数据库、不发TCP控制。ASCII注册/心跳本身没有Modbus CRC，统计中的crc_invalid不等于注册/心跳不可用。

```sh
fixture_path=$(mktemp /tmp/nehuang-replay.XXXXXX)
cat > "$fixture_path" <<'JSONL'
{"raw_seq":1,"payload_hex":"46505F544553540A"}
{"raw_seq":2,"payload_hex":"6B656570616C697665"}
{"raw_seq":3,"payload_hex":"DEADBEEF"}
JSONL
GOPROXY=off go run ./cmd/gateway-adapter-replay -raw-log "$fixture_path"
rm "$fixture_path"
```

已运行结果：total=3，register=1，heartbeat=1，unknown=1；crc_valid=0，crc_invalid=3。验证留存后的分类回放可用，不证明MCU0编码与温度业务闭环。

本地模拟器已存在 `cmd/gateway-adapter-mock`，支持指定 `-addr`、`-collector-id-hex`、`-frame`、`-wait-downlink`；其默认仍是旧MCU1样例。运行该模拟器会连接TCP并发送上行，必须另行确保目标是隔离本机测试Adapter；本次未运行它连接任何平台。真实控制只能在协议及安全入口确认后按验收计划进行。云端命令页已有Hex发送入口，但5字节初始化、6字节ACK及阈值/温度端序尚未冻结，不能把现有样例直接用在硬件上。

甲方环境入口、证书与鉴权验收留付款后迁移阶段。本次不再请求登录。

## 2026-09-30甲方回复更新（仅文档）

最新权威答复见[确认来源](protocols/mcu0-0910/confirmation-20260930.md)：历史温度每5分钟记录、整点BB01一次12组，仅手动模式4分钟回显；温度int16大端、实际×10；历史ACK为8字节且带CRC。BB03进入手动，BB01自动温控并停止4分钟回显。开关无即时返回，不能将之前一般状态机ACK测试理解成硬件必有即时ACK；应等周期回显并在关联规则确认后判断结果。原有5分钟TTL为现有软件配置，不代表硬件执行期限已验收，不能据timeout断言未执行或自动重发。

本轮没有功能改动。此前列出的周期、温度编码和历史ACK长度矛盾已获答复；TCP分帧、清缓存/重发、时间/跨天、地址/身份及完整样例尚未解决。原文与旧检查项保留追溯，冲突时以本段及最新阅读版为准。MCU0负责硬件调度，云端不新增时隙实现。
