# 涅凰智农 Docs 入口

> 硬件人员先看 [接云与报文操作说明](product_manual.md)。最新客户原始包、来源和已验证字段见 [10/1报文阅读版](protocols/mcu0-20261001/README.md)；实测边界见 [10/2联调验收](hardware_integration_20261002.md)。历史5分钟一组、手动约4分钟回显；BB02清缓存当前禁止实发，5种故障样本仍待客户。旧MCU1/4小时读取/BCD建议不替代最新MCU0来源。


> 更新日期：2026-10-02

本目录同时包含当前事实文档和早期方案材料。涉及实现、联调、对外说明时，优先阅读本文件列出的当前入口。

## 当前事实入口

| 文档 | 用途 |
|------|------|
| [product_manual.md](product_manual.md) | 给硬件人员：连云、发包、收发结果、正常表现和排查步骤 |
| [raw_debug_console.md](raw_debug_console.md) | 报文调试窗口当前实现与维护边界 |
| [help_center_integration.md](help_center_integration.md) | 独立帮助内容文件的开发集成交接 |
| `current_product_status.md` | 当前已实现能力、Gap、待确认问题 |
| `current_api.md` | 当前代码真实注册的 HTTP API |
| `gateway_adapter.md` | 硬件端联调指南、TCP 接入方式、下行命令链路和云端 Adapter 运行说明 |
| `hardware_protocol_current.md` | 根据硬件端最新逻辑说明书整理的云端相关协议边界 |
| `customer_questions.md` | 待硬件端和业务需求明确答复的问题，覆盖后台口径、报文、ACK、时隙和联调 |
| `architecture_v1.md` | 当前四层架构、数据流、控制链路和发布边界 |
| `data_model_v1.md` | 设备身份、主模型、兼容投影和写库口径 |
| `deploy_guide.md` | 部署、迁移、线上验证 |

## 阅读顺序

1. 看 `current_product_status.md`，确认当前实现和未实现范围。
2. 看 `architecture_v1.md`，确认系统分层和端到端链路。
3. 看 `gateway_adapter.md`，确认硬件如何连接云端 Adapter。
4. 看 `hardware_protocol_current.md`，确认硬件端文档中与云端有关的报文、周期和待确认点。
5. 看 `current_api.md`，确认真实 HTTP 接口。

## 文档维护规则

- `current_*` 文档必须以当前代码事实为准，不以历史设想为准。
- 硬件端新发协议、报文格式、时隙规则后，先更新 `hardware_protocol_current.md`，再同步 `architecture_v1.md` 和 `current_product_status.md`。
- 未在代码中实现的能力必须标明“未实现”或“待确认”，不能写成已交付。
- 旧 Python、`devices.json`、历史图表可以作为背景，但不再作为当前正式架构默认路径。
