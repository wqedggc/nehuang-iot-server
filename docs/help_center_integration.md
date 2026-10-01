# 帮助中心内容集成交接

2026-10-02，仅个人项目。内容文件 `web/static/admin/help-center-content.js` 无网络请求、发送动作或业务状态修改；本任务未改app.js/index.html，避免覆盖开发正在处理的页面。

开发集成两处：

1. `web/static/admin/index.html` 在 `app.js` 之前加载 `<script src="help-center-content.js"></script>`。
2. `renderHelpCenter(container)` 的现有主体改为：

```js
var activeSection = normalizeHelpSection(getHashParam('section'));
container.innerHTML = window.NHHelpCenter.render(activeSection);
```

替换整套旧帮助内容，避免仍显示旧MCU1/BCD/固定12组/4小时读取等口径。保留路由和normalizeHelpSection；无需改业务收发代码。目录链接使用现有hash路由。

集成后验证五个section、返回其他页面、长文滚动和窄屏；从原始报文页面打开调试窗口核对按钮。原始报文页获取数据成功后才渲染“报文调试窗口”，若查询超时按钮可能不可见，需开发修复/实测；文档不宣称超时已经解决。

客户说明书以 `docs/product_manual.md` 为准。50帧来源保留在docs/protocols；说明不会新造可实发样例。线上登录UI与硬件验收仍待开发核对，尤其当前用户报告原始报文超时。不要发送BB02或实发硬件命令来验证文案。
