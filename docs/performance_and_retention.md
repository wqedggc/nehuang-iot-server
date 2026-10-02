# 真实性能与数据保留：实施检查点

2026-10-02；用户个人测试服务器，当前46b4702。复用shared development-harness、当前仓库离线测试和CNB PR→master→deploy，不另造执行框架。索引已执行并核验；归档和删除尚未执行。

## 实证与测量边界

raw_device_events精确来源计数：TCP1191983、结构化255。InnoDB容量估计：数据789577728B、索引207257600B，行数估计1114960（估计不等于COUNT）。每天约29000条；mock PID16259持续发register/heartbeat/mcu1-auto-history/mcu1-manual/mcu1-fault，heartbeat30s/work10s。1161476条同测试网关，1160472条loopback来源；这两个数量单独不能证明全为mock，更不能据此删除。账户、设备配置、绑定、命令审计保留。control_results.raw_event_id引用原记录，NO ACTION。23命令均现有timeout，未发控制。45.3GiB可用空间（df约4754万KiB），目前磁盘空间非首要瓶颈。

串行5样本、单SQL5秒上限、当前真实数据量、暖缓存，包含mysql客户端开销，非API/浏览器百分位：

|阶段|P50 ms|P95 ms（n=5最大值）|
|---|---:|---:|
|默认TCP近24h精确COUNT|1316|1408|
|默认TCP近24h第一页ID分页|1375|1393|
|dashboard范围聚合|1294|1368|
|dashboard心跳聚合|2303|2342|

Adapter healthy/pending0/write_failures0/forward_failures0；连接3、历史最大4，配置池上限10，暂未证明池耗尽。上行吞吐和下行真实执行延迟尚未测，不把健康计数称为吞吐验收。此前真实Chrome请求默认列表2.67–2.69s，dashboard3.60s；点击与视觉受工具权限限制。

重跑：在已明确目标的现有项目目录执行 `python3 scripts/performance_baseline.py --samples 5`。仅读取现有CNF，凭据不输出；串行、无写请求/设备命令。结果记录到私有任务证据，不上传真实行数据。

## 切片1：已执行的可回退索引

目标：用时间范围索引替代119万行扫描，保留精确总数/过滤/分页/报文原文与capture幂等契约。候选SQL：

```sql
SET SESSION lock_wait_timeout=5;
ALTER TABLE raw_device_events
 ADD INDEX idx_raw_source_received (source_type,received_at_ms,raw_event_id),
 ADD INDEX idx_raw_received_type_status (received_at_ms,event_type,parse_status),
 ALGORITHM=INPLACE, LOCK=NONE;
```

实测MySQL8.0.46/InnoDB在线二级索引仍会消耗I/O/临时空间，开始/结束需要metadata lock；先检查长事务和能力，无法满足LOCK=NONE则停止，不退回COPY锁表。两个索引预计数十至约百MB，实际按构建测量，不承诺无阻塞。先私有完整一致性备份并校验隔离恢复；保留旧二进制。回退为只删除这两个新增索引，原数据和原索引不改。DDL失败/中断须核SHOW INDEX实际状态后再续，不能盲重试。本步骤无新权限、无清数据。

候选目标（待真实测量而非硬凑）：百万行且近24h约3万行时，默认列表/API SQL应减少到数十至数百ms；先以真实前后P50/P95确认，再定义页面预算。

## 切片2：冷数据与mock清理

优先保留raw_device_events为全历史身份/metadata权威表，冷表承载旧payload_hex/payload_snapshot；header与全局(source_type,capture_id)唯一性及FK保留。可避免简单热冷UNION导致重复capture入库和下游断引用。详情、Hex搜索及来源判断必须能读取冷表，页面不默默隐藏历史；未改这些读路径前不能清热表payload。冷/热阈值暂以14天作为待验证候选，不能把adapter journal的14天规则自动套作数据库事实。

每批固定cutoff/ID游标、短事务：先复制旧payload并逐行校验hash/字节，再清该行热payload并提交；崩溃前提交保持原状态，崩溃后可幂等按ID/hash续跑；任何不一致保留热原文并报错。保留NULL与空值区别，禁止按温度相同去重。归档读fallback/hash/恢复步骤与边界都要测试后才启用后台。批次/间隔有上限，不与接收事务或设备控制耦合；错误停止，不无限重试。

mock永久删除和冷归档分开。先依据进程配置、明确生成报文与来源证明选定集合；不按source_type=1或名字含TEST整表判断。备份仅私有原服务器、校验行数/hash与恢复演练；FK相关/真实命令审计/账号配置/未知来源不删。暂停确认的mock生成器须记录并可恢复原参数；不暂停真实设备。raw 单表一致性备份已通过隔离恢复；它不包含关联表，不支持整库回滚或删关联数据。未删除、未暂停mock。

## 验收

先保留真实百万规模前后对照；不空表自证、不仅延长timeout。覆盖默认和历史窗口、多来源、精确总数/稳定分页、详情/冷payload、关键字过滤、捕获唯一幂等、崩溃重启/回退、持续上行/队列观察。API/页面、SQL、真实硬件执行分别报告，不混用。浏览器受阻不伪称全页验收。

## 在线索引执行条件补查

实测server/mysqldump均8.0.46；innodb_file_per_table=1；online alter log上限128MiB；数据库/tmp/项目同文件系统，可用45.3GiB。当前长事务0、raw表待决metadata lock0，此状态须执行前重查。两个索引已按下方执行回执核验。

保留所有现有索引：capture唯一索引负责去重；gateway/node/point+received按对象查询；type/status不能覆盖时间范围；updated_at不等于received_at。新source+received+ID服务列表，新received+type/status覆盖仪表盘；有首列部分重叠但职责不同，不因重叠删除约束或对象查询索引。

预算：新索引数十至约百MiB是估算而非实测；预留至少3GiB用于私有备份、索引排序和在线日志，若能力/空间门禁不满足则停止。在线DDL不保证零metadata阻塞，锁等待限5秒；LOCK=NONE/INPLACE被拒就停止，不自动降级为COPY/阻塞DDL。会话断连后的SQL结果未知，必须先核索引定义、DDL实际状态，不重复动作。

回退精确步骤（核对确由本切片创建后）：`ALTER TABLE raw_device_events DROP INDEX idx_raw_source_received, DROP INDEX idx_raw_received_type_status, ALGORITHM=INPLACE, LOCK=NONE;`。无行删除，capture唯一和原索引保留；移除性能索引可能恢复慢查询，需回滚对应代码/部署验收一起记录。

备份和 DDL 的实际结果见下方回执。禁止以旧整表恢复覆盖持续入库；恢复演练仅在临时隔离库进行，未覆盖热库、未创建凭据。

## 2026-10-02 执行回执与证据边界

MySQL 8.0.46，INPLACE/LOCK=NONE 两索引构建成功，14.647 秒；原索引与 capture 唯一约束保留，删除行数 0。迁移文件为 `schema/mysql/008_raw_event_time_indexes.sql`，先核验两个已存在同名索引的列顺序、非唯一性与无前缀；任一不匹配先失败，不能用 --force 继续。版本登记须在实际定义核验后进行，deploy 不自动执行 DDL。

原服务器私有备份 raw_device_events：gzip 76,562,199 字节，SHA256 `30687035876bba3932c855b8d5c8e26d564aca92985cf2b8a242618e08bde2ed`。固定最大 ID 的一致性快照与隔离恢复均 1,192,581 行；gzip 完整性与 128 行不可变原文字段 hash 一致。抽样不等于全表逐行校验，也不包含关联表；临时恢复库已移除。

相同串行暖缓存 SQL 测量 n=5，单位 ms，包含客户端：

|阶段|索引后 P50|索引后 P95（5 样本最大值）|
|---|---:|---:|
|COUNT|34.37|35.00|
|第一页 ID|18.09|18.24|
|范围聚合|39.82|40.07|
|心跳聚合|27.14|27.18|

完整 EXPLAIN FORMAT=JSON 应保留排序/派生节点； rows_examined_per_scan 是优化器估计，不能称实测扫描行。ID 查询不含 payload 或完整 API。

08:58:07–08:58:27 已有 Chrome 会话导航对应服务器日志：10 次 payload 请求明确 status=200，GET 服务端耗时 18.82–20.26 ms；5 次 dashboard GET 耗时 8.23–12.19 ms，但该通用日志没有状态码，不能据此宣称 dashboard 200。前一轮窗口筛选日志为空，后续扩大实际时间窗口才找到上述记录。以上均非网络、DOM 或页面视觉验收，短样本不作为生产 API P95。

冷数据尚未写入。主行及 capture 唯一性必须保留；需要同时适配列表、详情、keyword COUNT/分页、MCU0 FOR UPDATE 校验的冷 payload fallback 后才可清热 payload。mock 清理还须排除 control_results、sensor_data、cloud_gateways/device_realtime_maps/collector_nodes/device_points 的 raw 引用，固定 ID 与关联备份恢复未齐全前禁止删行。
