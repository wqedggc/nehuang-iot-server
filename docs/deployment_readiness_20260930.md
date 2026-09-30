# 2026-09-30部署准备核对

依据[既有部署流程](deploy_guide.md)及deploy.sh静态审计。尚未远程执行、commit/push或部署。部署授权仅用于用户自己的服务器，目标归属尚待确认；runbook唯一IP与此前环境说法存在冲突，不能自动认定归属。

## 既有流程与风险

流程为CNB git commit/push →目标服务器pull →bash deploy.sh →独立构建/重启gateway-adapter；不使用scp覆盖。前端嵌入Go二进制。remote可能含token，不输出原始URL；不输出.env、密码/哈希或完整线上日志。

- Step4创建数据库/schema_migrations，按版本跳过SQL；旧版本只按表/列/索引存在补记已执行，不证明所有结构完整。迁移含ALTER/DROP INDEX与UPDATE；MySQL DDL非事务可回滚。须只读比较已应用版本和结构，并在任何待执行DDL前取得一致备份及恢复方案；本次温度代码没有新增schema migration。
- Step5固定upsert管理员10001，已有用户也会更新password_hash；摘要打印密码。原脚本没有跳过管理员步骤的参数。部署授权不等于重置凭据授权，不运行这一步。
- Step6先停止HTTP再编译，编译失败导致停机；脚本无自动回滚。只检查healthz且端口固定，不能代替readyz、页面和版本验收。
- Adapter独立重启会中断设备TCP连接；必须保留journal与ACK目录，不能为发布清空pending。systemd未完成。

## 拟最小安全修复（尚未实施）

保持CNB发布路径。日常部署默认跳过管理员创建/密码更新且不打印secret；初始化另设显式入口，现有凭据不改。先构建新HTTP/Adapter产物再停止旧进程，保留旧二进制及版本；新进程健康失败恢复旧产物与原启动配置。若数据库需不可逆变更则二进制回滚不足，先单独评估迁移并验证备份，不自动执行DDL。

目标确认后：只读确认目标目录、版本、未提交改动、DB迁移状态与监听；保存旧commit/产物校验、进程启动方式、私有配置（限制权限、不向工具输出）及journal；验证新构建再切换；检查HTTP healthz/readyz、Adapter healthz/readyz、监听、admin/mini静态资源与版本。受保护页面由合法管理员会话验收，不能用未登录401当功能通过。不修改TLS/权限或新建凭据，不发硬件控制。

本次温度增量的自动入库/ACK发送默认未启用，详见[温度接入边界](temperature_integration_status_20260930.md)；发布不能宣称完整温控闭环已就绪。
