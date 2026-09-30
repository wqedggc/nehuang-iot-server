# 涅凰智农 Docs 入口

> 2026-09-30 一期联调补充：最新 MCU0 0910 附件的来源、按页原文与结构化阅读版见 [协议资料入口](protocols/README.md)，实测范围与未完成项见 [10/2 联调验收](hardware_integration_20261002.md)。本文件原有 MCU1/历史周期口径不能自动替代 MCU0 附件；4/5分钟、ACK与编码矛盾仍待确认。


> 更新日期：2026-07-05

本目录同时包含当前事实文档和早期方案材料。涉及实现、联调、对外说明时，优先阅读本文件列出的当前入口。

## 当前事实入口

| 文档 | 用途 |
|------|------|
| `product_manual.md` | 运营与用户侧可用手册（管理员、C 端、联调说明入口） |
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
