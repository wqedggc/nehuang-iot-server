# 一期硬件接入前本机检查

更新：2026-09-30。范围为用户自己的环境；以下已执行命令只使用临时文件、net.Pipe 和本机测试 HTTP，不连接实际设备。停止甲方 HTTP/HTTPS、证书和管理员会话调查，它们不作为本次开发阻塞。不得把历史 IP `43.143.208.153` 自动当作用户自己的环境。

## 环境与地址确认

代码目录为 `/Users/shore/Desktop/ShoreOS/services/nehuang-iot-server`，独立个人 Git 仓库；不是公司 server worktree。服务配置入口为 `internal/config/config.go` 和 `cmd/gateway-adapter/main.go`，默认 HTTP `:8080`、TCP `:8081`、Adapter 健康端口 `:18081`、中心API `http://127.0.0.1:8080`。这些是代码默认值，不证明已运行服务归属或批准的目标地址。数据库默认文件为 `configs/mysql/nehuang_agent.cnf`；密钥不写入本清单。2026-09-30只读白名单核对本机配置：host=localhost、port=3306、socket=/private/tmp/mysql.sock、database=nehuang_db；当前执行shell没有HTTP/TCP/API地址覆盖。这只能证明开发配置，不能证明服务已启动或云端归属。

接真实设备前由用户确认：用户自己的 HTTP/API 目标、TCP 主机/端口、网关名与设备ID、隔离测试库、Adapter raw目录、服务版本。未确认前不启动服务、不运行deploy.sh、不运行init_mysql脚本、不运行连接TCP的mock。命令行server启动会连接数据库且启动后台扫描，不属于纯检查。

## 步骤一：语法与无外部依赖构建

```sh
cd /Users/shore/Desktop/ShoreOS/services/nehuang-iot-server
node --check web/static/admin/app.js
GOPROXY=off go test ./internal/domain ./internal/protocolsummary ./internal/gatewayadapter/protocol ./internal/repository/mysql ./web
git diff --check
```

预期全通过。repository测试为当前离线测试，不代表用户数据库表/索引或事务已验收。

## 步骤二：追踪与原文保留

```sh
GOPROXY=off go test ./internal/api/http -run 'TestRawCaptureRetainsDamagedCRCAndUnknownBytesWithoutProjection|TestHandleRawFramesDoesNotProjectDuplicateCapture' -v -count=1
GOPROXY=off go test ./internal/gatewayadapter -run 'TestHandleConnForwardsEveryReadAsRawCapture|TestHandleConnKeepsJournalingWhileUpstreamIsBlocked|TestReplayPendingBatchForwardsAfterUpstreamRecovery|TestReplayPendingBatchDoesNotBlockFollowingCaptureAfterOneFailure' -v -count=1
GOPROXY=off go test ./internal/gatewayadapter/journal -run 'TestWriterRestoresPendingAndForwardedState|TestPruneForwardedSegments' -v -count=1
```

预期：所有TCP字节顺序不变；capture_id/message_id/connection_id/raw_seq可追踪；CRC损坏和未知包保留原文及完整信封、不投影猜测数据；中心不可用时继续journal；重试使用原capture身份；重启恢复pending；只有关闭且已获durable确认的过期分段才可回收。

重复capture的API投影幂等已有测试和唯一键；它不是硬件重复业务包幂等。不同capture_id携带相同业务内容不能按hex简单去重，否则可能删除合法重复采样；真实业务序号/时间确认后再定义幂等键。

## 步骤三：发送、回应与失败区分

```sh
GOPROXY=off go test ./internal/gatewayadapter -run 'TestPendingCommandRetriesSentWithoutRepeatingTCPWrite|TestHandleConnDispatchesPendingCommandsAndMarksSent|TestHandleConnDoesNotMarkSentAfterDisconnect' -v -count=1
GOPROXY=off go test ./internal/service -run TestControlTimeoutScannerScanOnceUsesTTLCutoff -v -count=1
```

预期：下行write attempt先journal；TCP写失败不标sent；成功写后仅标sent，不自动ack/succeeded。新增修复：同一连接的命令持续列为pending时，若sent回写失败，重试状态回写而不重复写TCP。复现测试原代码3次TCP发送，修复后1次；同一命令3次状态回写，第三次恢复。缓存随pending列表退出回收，不增加数据库schema。

限制：该修复不是跨连接/重启的执行幂等；partial-write+error、Adapter进程崩溃、sent提交响应丢失等仍需要协议回执/稳定命令身份和对账。journal outbound记录为`tcp_write_attempt`，仅凭这条记录不能确认全部字节发送成功。

UI入口：设备管理→目标网关/采集执行器→功能点“发送下行Hex指令”，创建命令后在控制日志查看pending/sent/ack/succeeded/failed/timeout，原始报文可按网关和message_id反查。本次没有点击发送；Hex结构/ACK字节仍按协议阅读版待确认。

依据docs/data_model_v1.md（设备可先ACK再返回最终执行状态），本次修复云端生命周期：ACK不写finished_at；pending按请求时间、sent/ack按首次发送时间（缺失时按请求时间）使用既有TTL，重复ACK不刷新TTL。结果在FOR UPDATE锁下仅推进非终态；重复和迟到回应仍逐条写control_results，成功/失败/超时终态不被自动覆盖。超时表示等待最终结果超时，不证明设备未执行；迟到成功只作审计，需人工对账，不自动重发。没有新增TTL配置、数据库schema或猜测硬件ACK字节。

跨连接/进程恢复尚无exactly-once承诺。建议后续将已尝试发送但结果不明的命令显式标为UNKNOWN并要求人工确认后重新创建命令，代价是故障恢复可能暂停控制且需新增状态/API/UI兼容；本次没有实行这项业务行为变化。硬件接入前应避免对执行结果不明的命令盲目重复提交。

```sh
GOPROXY=off go test ./internal/domain -run TestControlResultOrdering -v
GOPROXY=off go test ./internal/repository/mysql -run 'TestControlResultsPreserveTerminalStateAndAudit|TestControlTimeoutIncludesAckWithOriginalSendCutoff' -v
```

持久化测试使用内存SQL驱动，验证现有Repository方法的锁查询、更新参数与审计插入；没有接MySQL，不代表实际数据库锁竞争已验收。

## 步骤四：并发与全量离线回归

```sh
GOPROXY=off go test -race ./internal/gatewayadapter ./internal/gatewayadapter/journal ./internal/api/http ./internal/service
GOPROXY=off go test ./...
```

这些测试开本机临时mock监听端口，受沙箱限制时需允许本机测试监听。无需实际微信、云端、数据库或设备。运行结果应保存到任务证据，不能把测试通过称为真实硬件验收通过。

## 最小温度解码方案（历史方案，9/30已有实现）

复用现有protocolsummary字段切分和CRC/Tag函数，不新增协议引擎。确认温度宽度后增加一个纯函数：输入温度原始字节和显式编码配置（端序、有无符号、缩放）；任一项未知即返回待确认，不设默认猜测。输出值与原始字节关联，测试覆盖正/负温、边界和长度错误。先做单条值解码，不自动生成历史日期、序号幂等键或清缓存ACK；这些需要独立协议答案。云端参数编辑本身不能证明MCU阈值已生效。

完整协议确认项与温度闭环验收见 [MCU0阅读版](protocols/mcu0-0910/reading.md) 和 [10/2验收](hardware_integration_20261002.md)。二期七组指标只保留扩展边界，不实现未提供业务逻辑。

## 完成与剩余分类

- 已完成：两页错误诊断、原始CRC/未知包留存测试、同连接发送回写重试修复、ACK后TTL与终态保护、协议资料和离线运行清单；全量与竞态回归通过。
- 可继续（不需云环境）：localhost mock与离线回放、补充真实样例fixture、页面响应诊断。跨连接UNKNOWN为后续方案，本次不新增业务状态。
- 需用户协议：温度宽度/端序/符号/缩放、采样与上传周期、分帧、ACK/清缓存与命令关联、时间/日期编码；温度真实入库和温控闭环尚未完成。
- 需目标环境（只在真实接入前）：用户自己的API/TCP地址、隔离测试库和网关/设备身份、运行版本；不要求先搭云。真实测试需确认温控动作边界和结果不明的人工对账方式，付款后才迁移甲方。

## 2026-09-30甲方回复更新（仅文档）

最新权威答复见[确认来源](protocols/mcu0-0910/confirmation-20260930.md)：历史温度每5分钟记录、整点BB01一次12组，仅手动模式4分钟回显；温度int16大端、实际×10；历史ACK为8字节且带CRC。BB03进入手动，BB01自动温控并停止4分钟回显。开关无即时返回，不能将之前一般状态机ACK测试理解成硬件必有即时ACK；应等周期回显并在关联规则确认后判断结果。原有5分钟TTL为现有软件配置，不代表硬件执行期限已验收，不能据timeout断言未执行或自动重发。

本轮没有功能改动。此前列出的周期、温度编码和历史ACK长度矛盾已获答复；TCP分帧、清缓存/重发、时间/跨天、地址/身份及完整样例尚未解决。原文与旧检查项保留追溯，冲突时以本段及最新阅读版为准。MCU0负责硬件调度，云端不新增时隙实现。

最新温度实现与合成测试见[温度增量状态](temperature_integration_status_20260930.md)，本页早前列出的温度编码待确认项已由甲方回复解决；身份/时钟/完整帧仍未解决。
