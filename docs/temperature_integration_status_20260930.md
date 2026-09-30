# 2026-09-30 MCU0温度增量与启用边界

协议依据：[甲方9/30确认](protocols/mcu0-0910/confirmation-20260930.md)。本次在个人项目实现，不含MCU0时隙、真实设备连接或控制、云端部署。

## 已实现

- `internal/gatewayadapter/protocol/mcu0_temperature.go`：实际×10有符号int16大端解码；编码输入精确十分之一度int16，避免隐式浮点舍入；CRC和Tag验证；自动历史/手动实时区分。历史正常12组按PPT的每组序号1字节+温度2字节布局处理，该布局仍要求显式验证，不称真实样例验收。
- 帧解析必须显式声明完整帧边界。拒绝半包、粘包、长度错误、CRC错误和未知BB；支持45字节12组、11字节手动、10字节无历史候选，未猜测补传/短历史变体。ID和时钟保留原字节。
- `internal/service/mcu0_temperature.go`：解析、验证显式设备/时间/布局策略，构造关联raw_event_id的sensor_data批次；不以到达时间代替历史时间，不推断ID端序。NTC故障拒绝有效温度投影。
- `internal/repository/mysql/mcu0_temperature.go`：锁定原始事件并核对payload_hex，整批事务写sensor_data与parse状态；同raw_event重复调用不再次投影；校验已存在功能点身份。历史不覆盖实时点；手动更新device_points且迟到值不覆盖较新last_report_at。不创建未知设备、不变更控制命令、阈值或继电器。
- 历史ACK纯函数为8字节ID+02+时钟+CRC。服务仅在存储接口成功返回（真实Repository为事务提交成功）且ACK批次时钟明确确认后返回候选字节；没有自动发送/清缓存。手动实时不产生ACK。
- 管理端原始报文摘要展示已确认温度、模式、8字节历史ACK含CRC；明确标为候选，时间与ID仍待确认，不据摘要入库。旧MCU1摘要保留独立函数。控制入口/日志说明无即时开关返回，sent是等待观测，timeout是结果未确认，勿盲重发。

## 安全禁用与剩余接入

真实TCP raw handler目前仍保留原文，**未自动调用新的温度入库服务**。因为一次Read不是一帧，尚无正式分帧和身份/日期策略，不能拿长度匹配启用真实业务投影。服务接口已可供离线/localhost harness调用；生产启用需后续在可信配置入口提供验证策略并接入完整帧处理，不能让上报方自行声明为已确认。

现有兼容表device_realtime_maps不在本次事务更新范围；真实端到端业务接入时需核对需要它的旧接口。传感器事务测试使用内存SQL驱动，不代表生产MySQL锁、索引和迁移状态已验证。相同业务包不同capture_id的幂等键仍未确认，不能按hex盲去重。

需协议输入：TCP分帧；地址端序/网关身份；时钟编码、时区和跨天历史归属；历史序号/补传；BB02批次时钟/清缓存/丢ACK重发；阈值编码；真实完整CRC报文。4分钟周期回显是设备状态观察，不是某个commandId的因果确认。

## 可运行离线验收

所有新增样例为合成测试，不来自真实硬件。

```sh
GOPROXY=off go test ./internal/gatewayadapter/protocol -run TestMCU0 -v
GOPROXY=off go test ./internal/service -run TestMCU0IngestGatesPersistenceAndACK -v
GOPROXY=off go test ./internal/repository/mysql -run TestMCU0TemperatureTransactionAndDuplicateProjection -v
GOPROXY=off go test ./internal/protocolsummary -run TestCurrentMCU0SummaryKeepsUnresolvedFields -v
GOPROXY=off go test ./...
```

覆盖25.3/-5和int16边界、损坏CRC、未确认边界、粘包、12组/实时分流、缺策略不写库、提交失败不生成ACK、历史不覆盖实时、事务中途失败与重复原始事件、候选摘要保留未知字段。真实报文到达后应逐字节核对原文，再开启业务投影。

## 第二次回复增量

来源及限制见[第二次回复](protocols/mcu0-0910/confirmation-20260930-followup.md)。已增加binary北京时间（显式日期）、集群+已解析1–100 ID组合、受限连续无延迟历史逻辑窗口纯函数及测试。没有新增0x7E分帧，没有启用真实投影，没有将逻辑窗口映射到实际数组，没有实现短历史/15组生产解码或自动ACK。

## 本地续接增量（稳定迁移快照之后）

候选纯解码扩展1–15组，保留实际线序，不生成序号时间含义；半条记录和超过15组拒绝。管理端候选摘要同步显示实际组数。真实投影仍禁用，时间映射仍显式验证。最新[截图回复](protocols/mcu0-0910/confirmation-chat-image-20260930.md)要求先不清缓存，BB02自动发送继续关闭。新增[只读调试预览](raw_debug_console.md)，没有任意发送能力。
