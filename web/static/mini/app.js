/**
 * 涅凰智农 C 端 — 应用逻辑
 * Hash 路由： #login | #devices | #device/:id | #history/:id | #settings | #help-center
 */

(function () {
  'use strict';

  // ============================================================
  // 状态
  // ============================================================
  let currentUser = null;
  let currentPage = '';

  // ============================================================
  // 工具函数
  // ============================================================

  function $(sel) { return document.querySelector(sel); }
  function $$(sel) { return document.querySelectorAll(sel); }

  function showToast(msg, type) {
    type = type || 'info';
    const el = $('#toast');
    el.textContent = msg;
    el.className = 'toast ' + type + ' show';
    clearTimeout(el._timeout);
    el._timeout = setTimeout(function () { el.className = 'toast'; }, 2500);
  }

  function formatTime(iso) {
    if (!iso) return '--';
    try {
      const d = new Date(iso);
      const pad = function (n) { return n.toString().padStart(2, '0'); };
      return pad(d.getMonth() + 1) + '-' + pad(d.getDate()) + ' ' + pad(d.getHours()) + ':' + pad(d.getMinutes());
    } catch (e) { return iso; }
  }

  function formatFullTime(iso) {
    if (!iso) return '--';
    try {
      const d = new Date(iso);
      const pad = function (n) { return n.toString().padStart(2, '0'); };
      return d.getFullYear() + '-' + pad(d.getMonth() + 1) + '-' + pad(d.getDate()) + ' ' + pad(d.getHours()) + ':' + pad(d.getMinutes()) + ':' + pad(d.getSeconds());
    } catch (e) { return iso; }
  }

  function valueStatusClass(valueStatus) {
    var map = { 1: 'normal', 2: 'low', 3: 'high', 4: 'on', 5: 'off', 6: 'offline' };
    return map[Number(valueStatus)] || 'normal';
  }

  function devicePointID(d) {
    return d && d.point_id != null ? String(d.point_id) : '';
  }

  function toText(value) {
    if (value == null) return '';
    return String(value);
  }

  function groupDevicesByCollector(devices) {
    var map = {};
    var groups = [];
    devices.forEach(function (device) {
      var nodeID = toText(device.node_id || device.nodeId || '');
      var fallbackKey = 'point-' + devicePointID(device);
      var key = nodeID || fallbackKey;
      if (!map[key]) {
        map[key] = {
          key: key,
          nodeId: nodeID,
          nodeName: toText(device.node_name || device.nodeName || ''),
          gatewayName: toText(device.gateway_name || device.gatewayName || ''),
          gatewayAccessModeLabel: toText(device.gateway_access_mode_label || device.gatewayAccessModeLabel || ''),
          items: []
        };
        groups.push(map[key]);
      }
      map[key].items.push(device);
    });
    return groups;
  }

  // ============================================================
  // 导航
  // ============================================================

  function navigate(route) {
    window.location.hash = '#' + route;
  }

  function goBack() {
    window.history.back();
  }

  function updateTopbar(title, showBack) {
    $('#topbar-title').textContent = title;
    $('#topbar-back').style.visibility = showBack ? 'visible' : 'hidden';
    $('#topbar-back').style.pointerEvents = showBack ? '' : 'none';
    $('#btn-settings').style.display = currentPage === 'devices' ? '' : 'none';
  }

  function parseHash() {
    var hash = window.location.hash.replace('#', '') || 'devices';
    var questionIndex = hash.indexOf('?');
    var routePart = questionIndex >= 0 ? hash.substring(0, questionIndex) : hash;
    var parts = routePart.split('/');
    return { page: parts[0], params: parts.slice(1), query: questionIndex >= 0 ? hash.substring(questionIndex + 1) : '' };
  }

  function getHashParam(name) {
    var route = parseHash();
    if (!route.query) return null;
    var params = new URLSearchParams(route.query);
    return params.get(name);
  }

  function normalizeHelpSection(section) {
    var value = toText(section || '').toLowerCase();
    if (value === 'admin' || value === 'manager') return 'admin';
    if (value === 'mini' || value === 'c' || value === 'user' || value === 'c端') return 'mini';
    if (value === 'hardware' || value === 'adapter') return 'hardware';
    if (value === 'flow' || value === 'diagram') return 'flow';
    if (value === 'checklist' || value === 'verify') return 'checklist';
    return 'mini';
  }

  function helpTab(section, label, activeSection) {
    return '<button class="help-tab ' + (section === activeSection ? 'active' : '') + '" onclick="navigate(\'help-center?section=' + section + '\')">' + escHtml(label) + '</button>';
  }

  function helpList(items) {
    var html = '<ol class="help-list">';
    items.forEach(function (item) {
      html += '<li>' + item + '</li>';
    });
    return html + '</ol>';
  }

  function renderDataFlowDiagram() {
    var nodes = [
      { title: '采集执行器 / MCU', sub: '传感器、继电器、告警源' },
      { title: '设备网关', sub: '透传、聚合、维持硬件侧时序' },
      { title: '云端接入层', sub: 'TCP 接入、原始报文留痕、下行写回' },
      { title: '业务平台', sub: '管理端 / C 端展示、绑定、控制日志' }
    ];
    var html = '<div class="help-flow-board">';
    nodes.forEach(function (node, idx) {
      html += '<div class="help-flow-node"><div class="help-flow-index">' + (idx + 1) + '</div><div class="help-flow-title">' + escHtml(node.title) + '</div><div class="help-flow-sub">' + escHtml(node.sub) + '</div></div>';
      if (idx < nodes.length - 1) html += '<div class="help-flow-arrow">→</div>';
    });
    html += '</div>';
    html += '<div class="help-flow-note">上行按 1 → 2 → 3 → 4 进入平台；下行控制按 4 → 3 → 2 → 1 返回硬件。接入层只保证收发与留痕，业务解释以平台解析结果为准。</div>';
    return html;
  }

  function renderPacketMatrix() {
    var rows = [
      ['注册包', '设备网关 → 云端接入层 → 业务平台', '上电、重连或联调初始化时发送', '识别设备网关身份，建立或更新在线基础信息。'],
      ['心跳包', '设备网关 → 云端接入层 → 业务平台', '按硬件侧配置周期发送；联调时可用 keepalive', '刷新最后上报时间；离线判断以平台口径为准。'],
      ['工作数据包', '采集执行器 → 设备网关 → 云端接入层 → 业务平台', '标准周期包；甲方最新口径倾向 2 小时或按最终文档确认', '先完整保存原始报文；字段冻结后再投影到业务数据。'],
      ['告警包', '采集执行器 → 设备网关 → 云端接入层 → 业务平台', '出现异常时主动上报，是否重复发送按硬件最终规则', '优先留痕与展示类型；去重和告警等级按最终样例补齐。'],
      ['未知包', '设备网关或采集执行器 → 云端接入层 → 业务平台', '任意联调阶段都可能出现', '不丢弃；进入原始报文排查，等待补解析规则。'],
      ['下行控制包', '管理端 / C 端 → 业务平台 → 云端接入层 → 设备网关 → 采集执行器', '用户点击控制或联调人员手动输入 Hex 时触发', '先生成 pending，接入层写回 TCP 后变 sent；未收到明确 ACK 前不自动判成功。'],
      ['报文时间字段', 'MCU1 → 设备网关 → 云端接入层 → 业务平台', '最新版 MCU1 说明书定义为 HH / MM / SS 共 3 字节', '字段位置已明确；BCD/二进制编码、时区和 CRC 覆盖仍待硬件确认。联调可暂按 BCD、北京时间处理，但须双方书面确认后再作为正式协议。']
    ];
    var html = '<div class="help-packet-table"><div class="help-packet-row help-packet-head"><div>报文</div><div>方向</div><div>触发/周期</div><div>平台处理</div></div>';
    rows.forEach(function (row) {
      html += '<div class="help-packet-row">';
      row.forEach(function (cell) { html += '<div>' + escHtml(cell) + '</div>'; });
      html += '</div>';
    });
    return html + '</div>';
  }

  function renderTimeline() {
    var items = [
      ['1. 注册', '设备网关先让平台知道"我是谁"，后台才能显示并绑定给农户。'],
      ['2. 心跳', '注册后持续上报 keepalive，平台用它刷新在线状态。'],
      ['3. 业务包', '工作数据、告警、未知包都先进入原始报文排查，确认字段后再解析成业务数据。'],
      ['4. 绑定与查看', '管理员把设备网关绑定给农户，C 端看到该网关下授权的采集执行器和功能点。'],
      ['5. 下行控制', '管理员或 C 端创建控制命令，接入层在设备连接存在时写回 TCP。'],
      ['6. 状态回填', '控制日志展示 pending / sent / timeout 等状态；真实 ACK 语义等硬件样例确认。']
    ];
    var html = '<div class="help-timeline">';
    items.forEach(function (item) {
      html += '<div class="help-timeline-item"><div class="help-timeline-dot"></div><div><div class="help-timeline-title">' + escHtml(item[0]) + '</div><div class="help-timeline-body">' + escHtml(item[1]) + '</div></div></div>';
    });
    return html + '</div>';
  }

  function renderValidationChecklist() {
    var items = [
      ['登录', '管理员账号能进入管理端；C 端农户账号能进入我的设备。'],
      ['跳转', '管理端侧边栏可进入帮助中心；C 端设置页可进入使用说明；各帮助目录点击后高亮并滚动到对应章节。'],
      ['搜索', '原始报文排查可按设备网关、消息 ID、原始 Hex 片段搜索，默认最近 24 小时。'],
      ['滚动', '帮助页、原始报文详情、设备详情在桌面和手机宽度下均可滚动，不遮挡按钮。'],
      ['绑定', '管理员绑定设备网关给农户后，C 端设备列表能展开到采集执行器和功能点。'],
      ['联调报文', '注册、心跳、工作数据、告警、未知包都能在原始报文排查中看到类型或待解析状态。'],
      ['下行命令', '手动 Hex 控制创建后进入控制日志；设备连接存在时状态至少能到 sent，否则按 TTL 超时。']
    ];
    var html = '<div class="help-checklist">';
    items.forEach(function (item) {
      html += '<div class="help-check-item"><span class="help-check-box">□</span><div><div class="help-check-title">' + escHtml(item[0]) + '</div><div class="help-check-body">' + escHtml(item[1]) + '</div></div></div>';
    });
    return html + '</div>';
  }

  // ============================================================
  // 页面渲染
  // ============================================================

  function renderPage() {
    var route = parseHash();
    currentPage = route.page;
    var container = $('#main-content');
    window.scrollTo(0, 0);

    // 未登录则跳转登录
    if (route.page !== 'login' && !localStorage.getItem('nh_mini_token')) {
      navigate('login');
      return;
    }

    switch (route.page) {
      case 'login': renderLogin(container); updateTopbar('涅凰智农', false); break;
      case 'devices': renderDeviceList(container); updateTopbar('我的设备', false); break;
      case 'device': renderDeviceDetail(container, route.params[0]); updateTopbar('设备详情', true); break;
      case 'history': renderHistory(container, route.params[0]); updateTopbar('历史曲线', true); break;
      case 'settings': renderSettings(container); updateTopbar('设置', true); break;
      case 'help-center': renderHelpCenter(container); updateTopbar('使用说明', true); break;
      default: renderDeviceList(container); updateTopbar('我的设备', false);
    }
  }

  // ---------- 登录页 ----------
  function renderLogin(container) {
    container.innerHTML = '<div class="login-page">' +
      '<div class="login-card">' +
        '<div class="login-logo">' +
          '<div class="login-mark" aria-hidden="true">N</div>' +
          '<span class="login-kicker">FIELD OPERATIONS</span>' +
          '<h2>连接田间工作台</h2>' +
          '<p>查看设备状态、实时数据与控制结果</p>' +
        '</div>' +
        '<div class="login-notice" role="note">' +
          '<strong>浏览器调试入口</strong>' +
          '<p>微信真实登录请在小程序内完成；这里仅校验已有的 Mini Token。</p>' +
        '</div>' +
        '<div class="form-group">' +
          '<label class="form-label" for="login-token">H5 调试 Token</label>' +
          '<input class="form-input" id="login-token" type="password" autocomplete="off" spellcheck="false" aria-describedby="login-token-hint" placeholder="粘贴真实 nh_mini_token">' +
          '<p class="form-hint form-hint-left" id="login-token-hint">仅用于本次浏览器调试，不会调用开发登录接口。</p>' +
        '</div>' +
        '<button class="btn btn-primary btn-block" id="login-submit">校验并进入</button>' +
      '</div>' +
    '</div>';

    var tokenInput = $('#login-token');

    $('#login-submit').onclick = function () {
      var token = tokenInput.value.trim();
      if (!token) { showToast('请粘贴真实 mini token', 'error'); return; }
      saveMiniTokenForDebug(token);
    };
  }

  async function saveMiniTokenForDebug(token) {
    var btn = $('#login-submit');
    btn.disabled = true;
    btn.textContent = '校验中...';
    try {
      localStorage.setItem('nh_mini_token', token);
      var res = await NHApi.getMe('mini');
      if (res.data) {
        localStorage.setItem('nh_mini_user', JSON.stringify(res.data));
        showToast('登录成功', 'success');
        navigate('devices');
      } else {
        localStorage.removeItem('nh_mini_token');
        localStorage.removeItem('nh_mini_user');
        showToast(res.message || '登录失败', 'error');
      }
    } catch (e) {
      localStorage.removeItem('nh_mini_token');
      localStorage.removeItem('nh_mini_user');
      showToast(e.message || 'Token 无效', 'error');
    } finally {
      btn.disabled = false;
      btn.textContent = '校验并进入';
    }
  }

  // ---------- 设备列表 ----------
  async function renderDeviceList(container) {
    container.innerHTML = '<div class="loading-state"><div class="loading-spinner"></div><p>加载设备列表...</p></div>';

    try {
      // 确认 JWT 存在
      var token = localStorage.getItem('nh_mini_token');
      if (!token) {
        container.innerHTML = '<div class="empty-state">' +
          '<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M12 15v2m0-8v4m0 0a9 9 0 1 0 0-18 9 9 0 0 0 0 18z"/></svg>' +
          '<p>请先登录</p><button class="btn btn-outline btn-sm" onclick="navigate(\'login\')" style="margin-top:12px">去登录</button></div>';
        return;
      }

      var res = await NHApi.getDevices();

      // 处理空绑定状态
      if (!res.data || res.data.length === 0) {
        container.innerHTML = '<div class="empty-state">' +
          '<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><rect x="3" y="3" width="18" height="18" rx="2"/><line x1="9" y1="9" x2="15" y2="15"/><line x1="15" y1="9" x2="9" y2="15"/></svg>' +
          '<p>暂无绑定设备</p>' +
          '<p style="font-size:12px;color:var(--color-text-muted);margin-top:4px">请联系管理员为您绑定设备</p>' +
        '</div>';
        return;
      }

      var devices = res.data;
      var groups = groupDevicesByCollector(devices);
      var attentionCount = 0;
      var offlineCount = 0;
      devices.forEach(function (device) {
        var status = valueStatusClass(device.value_status);
        if (status === 'offline') offlineCount++;
        else if (status === 'low' || status === 'high') attentionCount++;
      });
      var healthyCount = devices.length - attentionCount - offlineCount;

      var html = '<section class="overview-card" aria-labelledby="device-overview-title">' +
        '<span class="overview-kicker">DEVICE OVERVIEW</span>' +
        '<h2 id="device-overview-title">田间运行概览</h2>' +
        '<p class="overview-copy">按采集执行器查看功能点状态，异常与离线设备会优先显示明确状态。</p>' +
        '<div class="overview-metrics">' +
          '<div class="overview-metric"><strong class="overview-metric-value">' + healthyCount + '</strong><span class="overview-metric-label">状态正常</span></div>' +
          '<div class="overview-metric"><strong class="overview-metric-value">' + attentionCount + '</strong><span class="overview-metric-label">需要关注</span></div>' +
          '<div class="overview-metric"><strong class="overview-metric-value">' + offlineCount + '</strong><span class="overview-metric-label">当前离线</span></div>' +
        '</div>' +
      '</section>';
      html += '<div class="device-groups">';
      groups.forEach(function (group) {
        html += '<section class="device-group">' +
          '<header class="device-group-header">' +
            '<div>' +
              '<span class="group-kicker">COLLECTOR NODE</span>' +
              '<h3 class="device-group-title">' + escHtml(group.nodeName || '采集执行器') + '</h3>' +
              '<p class="device-group-meta">' +
                '接入网关：' + escHtml(group.gatewayName || '--') +
                (group.gatewayAccessModeLabel ? ' · ' + escHtml(group.gatewayAccessModeLabel) : '') +
                (group.nodeId ? ' · ID ' + escHtml(group.nodeId) : '') +
              '</p>' +
            '</div>' +
            '<span class="device-group-count">' + group.items.length + ' 个功能点</span>' +
          '</header>' +
          '<div class="device-list">';
        group.items.forEach(function (d) {
          var pointID = devicePointID(d);
          var status = valueStatusClass(d.value_status);
          var statusClass = 'status-' + status;
          var statusTextClass = 'status-text-' + status;
          var title = escHtml(d.shed_no || '未设置棚号') + ' · 频点' + escHtml(d.freq);
          var deviceNo = 'DevID ' + escHtml(d.devid || '--');
          var statusLabel = escHtml(d.value_status_label || '--');
          var valueText = d.real_value != null ? String(d.real_value) : '--';
          var valueLabel = escHtml(valueText);
          var ariaLabel = escHtml((d.func_name || '设备') + '，' + (d.shed_no || '未设置棚号') + '，当前值 ' + valueText + (d.unit || '') + '，状态 ' + (d.value_status_label || '未知'));
          html += '<button type="button" class="device-card ' + statusClass + '" onclick="navigate(\'device/' + pointID + '\')" aria-label="' + ariaLabel + '">' +
            '<div class="device-card-header">' +
              '<span class="device-card-title">' + title + '</span>' +
              '<span class="device-card-func">' + escHtml(d.func_name) + '</span>' +
            '</div>' +
            '<div class="device-card-main">' +
              '<span class="device-card-reading"><span class="device-card-value">' + valueLabel + '</span><span class="device-card-unit">' + escHtml(d.unit) + '</span></span>' +
              '<span class="status-badge ' + statusClass + '"><span class="status-dot"></span><span class="' + statusTextClass + '">' + statusLabel + '</span></span>' +
            '</div>' +
            '<div class="device-card-meta">' +
              '<span>' + deviceNo + '</span>' +
              '<span>更新于 ' + formatTime(d.updated_at) + '</span>' +
            '</div>' +
            '<svg class="device-card-chevron" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><polyline points="9 18 15 12 9 6"/></svg>' +
          '</button>';
        });
        html += '</div></section>';
      });
      html += '</div>';

      container.innerHTML = html;
    } catch (e) {
      console.error('[C端] 设备列表加载失败:', e);
      container.innerHTML = '<div class="error-state"><p>加载失败：' + escHtml(e.message || '网络异常') + '</p><button class="btn btn-outline btn-sm" onclick="renderPage()">重试</button></div>';
    }
  }

  // ---------- 设备详情 ----------
  var deviceDetailData = null;
  var deviceDetailPointId = null;
  var deviceDetailActiveTab = 'info';
  var deviceDetailHistoryRange = '1d';       // 历史数据 tab 当前时间范围
  var deviceDetailHistoryLoaded = false;    // 历史数据 tab 是否已加载过

  /**
   * 判断设备是否离线 (value_status === 6)
   */
  function isDeviceOffline(d) {
    return Number(d.value_status) === 6;
  }

  /**
   * 生成离线提示条 HTML
   */
  function offlineBanner() {
    return '<div class="offline-banner">' +
      '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>' +
      '<span>设备离线，数据可能不是最新的</span>' +
    '</div>';
  }

  async function renderDeviceDetail(container, id) {
    if (!id) { navigate('devices'); return; }
    container.innerHTML = '<div class="loading-state"><div class="loading-spinner"></div><p>加载设备详情...</p></div>';

    try {
      var res = await NHApi.getDeviceRealtime(id);
      if (!res.data) {
        container.innerHTML = '<div class="error-state"><p>设备不存在</p><button class="btn btn-outline btn-sm" onclick="navigate(\'devices\')">返回列表</button></div>';
        return;
      }
      var d = res.data;
      var pointID = devicePointID(d) || String(id);
      deviceDetailData = d;
      deviceDetailPointId = pointID;
      deviceDetailActiveTab = 'info';
      deviceDetailHistoryRange = '1d';
      deviceDetailHistoryLoaded = false;

      var status = valueStatusClass(d.value_status);
      var statusClass = 'status-' + status;
      var statusTextClass = 'status-text-' + status;
      var offline = isDeviceOffline(d);
      var offlineHtml = offline ? offlineBanner() : '';

      // ── 设备上下文 + Tab Bar ──
      var heroValue = d.real_value != null ? escHtml(d.real_value) : '--';
      var heroContext = escHtml(d.shed_no || '未设置棚号') + ' · ' + escHtml(d.gateway_name || '未识别网关');
      var html = '<section class="device-hero ' + statusClass + '">' +
        '<div class="device-hero-top">' +
          '<div><span class="device-hero-kicker">DEVICE POINT</span><h2>' + escHtml(d.func_name || '设备') + '</h2><p class="device-hero-context">' + heroContext + '</p></div>' +
          '<span class="status-badge ' + statusClass + '"><span class="status-dot"></span><span class="' + statusTextClass + '">' + escHtml(d.value_status_label || '--') + '</span></span>' +
        '</div>' +
        '<div class="device-hero-reading"><strong class="device-hero-value">' + heroValue + '</strong><span class="device-hero-unit">' + escHtml(d.unit) + '</span></div>' +
        '<div class="device-hero-meta"><span>采集执行器 ' + escHtml(d.devid || '--') + '</span><span>频点 ' + escHtml(d.freq) + '</span><span>更新于 ' + formatTime(d.updated_at) + '</span></div>' +
      '</section>' +
      '<div class="tab-shell"><div class="tab-bar" role="tablist" aria-label="设备详情视图">' +
        '<button id="tab-btn-info" class="tab-btn active" data-tab="info" role="tab" aria-selected="true" aria-controls="tab-info">基本信息</button>' +
        '<button id="tab-btn-realtime" class="tab-btn" data-tab="realtime" role="tab" aria-selected="false" aria-controls="tab-realtime" tabindex="-1">实时数据</button>' +
        '<button id="tab-btn-history-table" class="tab-btn" data-tab="history-table" role="tab" aria-selected="false" aria-controls="tab-history-table" tabindex="-1">历史数据</button>' +
        '<button id="tab-btn-chart" class="tab-btn" data-tab="chart" role="tab" aria-selected="false" aria-controls="tab-chart" tabindex="-1">数据曲线</button>' +
        '<button id="tab-btn-control" class="tab-btn" data-tab="control" role="tab" aria-selected="false" aria-controls="tab-control" tabindex="-1">控制</button>' +
      '</div>';

      // ════════════════════════════════════════
      // Tab 1: 基本信息
      // ════════════════════════════════════════
      html += '<div class="tab-panel active" id="tab-info" role="tabpanel" aria-labelledby="tab-btn-info">' +
        offlineHtml +
        '<div class="detail-section">' +
          '<h3>基本信息</h3>' +
          '<div class="detail-grid">' +
            '<div class="detail-item"><div class="detail-item-label">设备名称</div><div class="detail-item-value">' + escHtml(d.func_name) + '</div></div>' +
            '<div class="detail-item"><div class="detail-item-label">设备网关</div><div class="detail-item-value">' + escHtml(d.gateway_name || '--') + '</div></div>' +
            '<div class="detail-item"><div class="detail-item-label">采集执行器</div><div class="detail-item-value">' + escHtml(d.devid || '--') + '</div></div>' +
            '<div class="detail-item"><div class="detail-item-label">棚号</div><div class="detail-item-value">' + escHtml(d.shed_no || '--') + '</div></div>' +
            '<div class="detail-item"><div class="detail-item-label">频点</div><div class="detail-item-value">' + escHtml(d.freq) + '</div></div>' +
            '<div class="detail-item"><div class="detail-item-label">在线状态</div><div class="detail-item-value"><span class="device-card-status ' + statusClass + '"><span class="status-dot"></span><span class="' + statusTextClass + '">' + escHtml(d.value_status_label || '--') + '</span></span></div></div>' +
            '<div class="detail-item"><div class="detail-item-label">最近更新</div><div class="detail-item-value">' + formatFullTime(d.updated_at) + '</div></div>' +
            '<div class="detail-item"><div class="detail-item-label">运行模式</div><div class="detail-item-value">' + escHtml(d.run_mode_label) + '</div></div>' +
          '</div>' +
        '</div>' +
      '</div>';

      // ════════════════════════════════════════
      // Tab 2: 实时数据
      // ════════════════════════════════════════
      var rangeHint = '';
      if (d.best_min != null && d.best_max != null) {
        rangeHint = '适宜范围：' + escHtml(d.best_min) + ' ~ ' + escHtml(d.best_max) + ' ' + escHtml(d.unit);
      }

      html += '<div class="tab-panel" id="tab-realtime" role="tabpanel" aria-labelledby="tab-btn-realtime" hidden>' +
        offlineHtml +
        '<div class="detail-section">' +
          '<h3>实时数据</h3>';

      if (d.real_value == null) {
        // 空数据状态
        html += '<div class="empty-state" style="padding:36px 24px">' +
          '<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z"/></svg>' +
          '<p>暂无实时数据</p>' +
          '<p style="font-size:12px;color:var(--color-text-muted);margin-top:4px">设备尚未上报实时数据，请稍后再试</p>' +
        '</div>';
      } else {
        html += '<div class="detail-real-value">' +
            '<span class="big-value">' + heroValue + '</span>' +
            '<span class="big-unit">' + escHtml(d.unit) + '</span>' +
            '<p class="range-hint">' + rangeHint + '</p>' +
            '<p style="margin-top:4px"><span class="device-card-status ' + statusClass + '"><span class="status-dot"></span><span class="' + statusTextClass + '">' + escHtml(d.value_status_label || '--') + '</span></span></p>' +
          '</div>';
      }

      html += '</div></div>';

      // ════════════════════════════════════════
      // Tab 3: 历史数据 (table)
      // ════════════════════════════════════════
      html += '<div class="tab-panel" id="tab-history-table" role="tabpanel" aria-labelledby="tab-btn-history-table" hidden>' +
        offlineHtml +
        '<div class="loading-state"><div class="loading-spinner"></div><p>点击标签加载历史数据...</p></div>' +
      '</div>';

      // ════════════════════════════════════════
      // Tab 4: 数据曲线 (chart)
      // ════════════════════════════════════════
      html += '<div class="tab-panel" id="tab-chart" role="tabpanel" aria-labelledby="tab-btn-chart" hidden>' +
        offlineHtml +
        '<div class="loading-state"><div class="loading-spinner"></div><p>点击标签加载数据曲线...</p></div>' +
      '</div>';

      // ════════════════════════════════════════
      // Tab 5: 控制
      // ════════════════════════════════════════
      html += '<div class="tab-panel" id="tab-control" role="tabpanel" aria-labelledby="tab-btn-control" hidden>' +
        offlineHtml;

      // 判断是否有控制功能 (func 1/2 是传感器, 4/5/6 是控制器)
      if (d.func === 4 || d.func === 5 || d.func === 6) {
        html += '<div class="detail-section">' +
          '<h3>设备控制</h3>' +
          '<div class="detail-controls">' +
            '<button class="btn btn-primary" onclick="doControl(\'' + pointID + '\',\'start\')">开启</button>' +
            '<button class="btn btn-outline" onclick="doControl(\'' + pointID + '\',\'stop\')">停止</button>' +
            '<button class="btn btn-danger" onclick="doControl(\'' + pointID + '\',\'close\')">关闭</button>' +
          '</div>' +
        '</div>';
      } else if (d.func === 1 || d.func === 2) {
        html += '<div class="detail-section">' +
          '<h3>设备状态</h3>' +
          '<p style="font-size:13px;color:var(--color-text-muted)">' +
            '运行模式：<strong>' + escHtml(d.run_mode_label) + '</strong> · 采集周期：' + (d.poll_interval || '--') + 's' +
          '</p>' +
        '</div>';
      } else {
        // 无控制功能，也不属于传感器 → 显示"不支持控制功能"
        html += '<div class="detail-section">' +
          '<h3>设备控制</h3>' +
          '<div class="empty-state" style="padding:36px 24px">' +
            '<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>' +
            '<p>此设备不支持控制功能</p>' +
            '<p style="font-size:12px;color:var(--color-text-muted);margin-top:4px">当前设备类型暂不支持远程控制操作</p>' +
          '</div>' +
        '</div>';
      }
      html += '</div></div>';

      container.innerHTML = html;

      // Bind tab clicks
      $$('.tab-btn').forEach(function (btn) {
        btn.onclick = function () {
          var tab = this.dataset.tab;
          switchTab(tab);
        };
        btn.onkeydown = function (event) {
          var tabs = Array.from($$('.tab-btn'));
          var currentIndex = tabs.indexOf(this);
          var nextIndex = currentIndex;
          if (event.key === 'ArrowRight') nextIndex = (currentIndex + 1) % tabs.length;
          else if (event.key === 'ArrowLeft') nextIndex = (currentIndex - 1 + tabs.length) % tabs.length;
          else if (event.key === 'Home') nextIndex = 0;
          else if (event.key === 'End') nextIndex = tabs.length - 1;
          else return;
          event.preventDefault();
          switchTab(tabs[nextIndex].dataset.tab);
          tabs[nextIndex].focus();
        };
      });
    } catch (e) {
      container.innerHTML = '<div class="error-state"><p>加载失败</p><button class="btn btn-outline btn-sm" onclick="renderPage()">重试</button></div>';
    }
  }

  function switchTab(tab) {
    deviceDetailActiveTab = tab;
    // Update tab buttons
    $$('.tab-btn').forEach(function (btn) {
      var active = btn.dataset.tab === tab;
      btn.classList.toggle('active', active);
      btn.setAttribute('aria-selected', active ? 'true' : 'false');
      btn.tabIndex = active ? 0 : -1;
    });
    // Update tab panels
    $$('.tab-panel').forEach(function (panel) {
      var active = panel.id === 'tab-' + tab;
      panel.classList.toggle('active', active);
      panel.hidden = !active;
    });

    // Lazy load: 历史数据 tab (table)
    if (tab === 'history-table' && deviceDetailPointId && !deviceDetailHistoryLoaded) {
      deviceDetailHistoryLoaded = true;
      loadHistoryTableInTab(deviceDetailPointId, deviceDetailHistoryRange);
    }

    // Lazy load: 数据曲线 tab (chart)
    if (tab === 'chart' && deviceDetailPointId) {
      var chartPanel = document.getElementById('tab-chart');
      if (chartPanel && chartPanel.querySelector('.loading-state')) {
        loadChartInTab(deviceDetailPointId);
      }
    }
  }

  // ════════════════════════════════════════
  // 历史数据 Tab — 表格形式
  // ════════════════════════════════════════

  async function loadHistoryTableInTab(pointID, range) {
    var panel = document.getElementById('tab-history-table');
    if (!panel) return;

    // 保留离线提示条
    var offlineHtml = (deviceDetailData && isDeviceOffline(deviceDetailData)) ? offlineBanner() : '';
    panel.innerHTML = offlineHtml + '<div class="loading-state"><div class="loading-spinner"></div><p>加载历史数据...</p></div>';

    try {
      var res = await NHApi.getDeviceHistory(pointID, range);
      var d = res.data;
      var points = (d && d.points) ? d.points : [];

      var html = offlineHtml;

      // Range tabs
      html += '<div class="range-tabs">' +
        '<button class="range-tab' + (range === '1d' ? ' active' : '') + '" data-range="1d">近1天</button>' +
        '<button class="range-tab' + (range === '3d' ? ' active' : '') + '" data-range="3d">近3天</button>' +
        '<button class="range-tab' + (range === '7d' ? ' active' : '') + '" data-range="7d">近7天</button>' +
        '<button class="range-tab' + (range === '30d' ? ' active' : '') + '" data-range="30d">近30天</button>' +
      '</div>';

      if (!d || points.length === 0) {
        // 无历史数据
        html += '<div class="detail-section">' +
          '<div class="empty-state" style="padding:36px 24px">' +
            '<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><rect x="3" y="3" width="18" height="18" rx="2"/><line x1="9" y1="9" x2="15" y2="15"/><line x1="15" y1="9" x2="9" y2="15"/></svg>' +
            '<p>暂无历史数据</p>' +
            '<p style="font-size:12px;color:var(--color-text-muted);margin-top:4px">该时间范围内没有历史数据记录</p>' +
          '</div>' +
        '</div>';
      } else {
        // 构建表格
        html += '<div class="detail-section">' +
          '<div class="history-table-wrap">' +
            '<table class="history-table">' +
              '<thead><tr>' +
                '<th>时间</th>' +
                '<th>实时值</th>' +
                '<th>单位</th>' +
                '<th>状态</th>' +
              '</tr></thead>' +
              '<tbody>';

        points.forEach(function (p) {
          var pStatus = valueStatusClass(p.value_status);
          var pStatusClass = 'status-' + pStatus;
          var pStatusTextClass = 'status-text-' + pStatus;
          html += '<tr>' +
            '<td class="history-time">' + formatFullTime(p.time) + '</td>' +
            '<td class="history-value">' + (p.real_temp != null ? escHtml(p.real_temp) : '--') + '</td>' +
            '<td class="history-unit">' + escHtml((d && d.unit) || (deviceDetailData && deviceDetailData.unit) || '--') + '</td>' +
            '<td><span class="device-card-status ' + pStatusClass + '"><span class="status-dot"></span><span class="' + pStatusTextClass + '">' + escHtml(p.value_status_label || '--') + '</span></span></td>' +
          '</tr>';
        });

        html += '</tbody></table></div></div>';
      }

      panel.innerHTML = html;

      // Bind range tab clicks
      panel.querySelectorAll('.range-tab').forEach(function (tab) {
        tab.onclick = function () {
          var newRange = this.dataset.range;
          if (newRange === deviceDetailHistoryRange) return;
          deviceDetailHistoryRange = newRange;
          loadHistoryTableInTab(pointID, newRange);
        };
      });
    } catch (e) {
      panel.innerHTML = offlineHtml + '<div class="error-state"><p>加载失败</p><button class="btn btn-outline btn-sm" onclick="switchTab(\'history-table\')">重试</button></div>';
    }
  }

  // ════════════════════════════════════════
  // 数据曲线 Tab — Canvas 图表
  // ════════════════════════════════════════

  async function loadChartInTab(pointID) {
    var panel = document.getElementById('tab-chart');
    if (!panel) return;

    var offlineHtml = (deviceDetailData && isDeviceOffline(deviceDetailData)) ? offlineBanner() : '';
    panel.innerHTML = offlineHtml + '<div class="loading-state"><div class="loading-spinner"></div><p>加载数据曲线...</p></div>';

    try {
      var res = await NHApi.getDeviceHistory(pointID, '1d');
      if (!res.data) {
        panel.innerHTML = offlineHtml + '<div class="detail-section"><div class="empty-state" style="padding:36px 24px">' +
          '<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>' +
          '<p>暂无数据曲线</p>' +
          '<p style="font-size:12px;color:var(--color-text-muted);margin-top:4px">暂无历史数据可用于绘制曲线</p>' +
        '</div></div>';
        return;
      }

      var d = res.data;
      var points = d.points || [];
      var activeRange = '1d';
      var rangeText = '';
      if (d.best_min != null && d.best_max != null) {
        rangeText = ' · 适宜范围：' + escHtml(d.best_min) + ' ~ ' + escHtml(d.best_max) + ' ' + escHtml(d.unit);
      }

      if (points.length === 0) {
        panel.innerHTML = offlineHtml + '<div class="detail-section"><div class="empty-state" style="padding:36px 24px">' +
          '<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>' +
          '<p>暂无数据曲线</p>' +
          '<p style="font-size:12px;color:var(--color-text-muted);margin-top:4px">暂无历史数据可用于绘制曲线</p>' +
        '</div></div>';
        return;
      }

      var html = offlineHtml +
        '<div class="range-tabs">' +
          '<button class="range-tab active" data-range="1d">近1天</button>' +
          '<button class="range-tab" data-range="3d">近3天</button>' +
          '<button class="range-tab" data-range="7d">近7天</button>' +
          '<button class="range-tab" data-range="30d">近30天</button>' +
        '</div>' +
        '<div class="chart-container">' +
          '<canvas id="history-chart"></canvas>' +
        '</div>' +
        '<p style="text-align:center;margin-top:12px;font-size:13px;color:var(--color-text-muted)">' +
          '设备：' + escHtml(d.func_name || '功能点') + ' · 单位：' + escHtml(d.unit || '--') +
          rangeText +
        '</p>';

      panel.innerHTML = html;

      // Bind range tabs
      panel.querySelectorAll('.range-tab').forEach(function (tab) {
        tab.onclick = function () {
          var range = this.dataset.range;
          if (range === activeRange) return;
          activeRange = range;
          panel.querySelectorAll('.range-tab').forEach(function (t) { t.classList.remove('active'); });
          this.classList.add('active');
          loadChartDataInTab(pointID, range, d);
        };
      });

      drawChart(points, d);
    } catch (e) {
      panel.innerHTML = offlineHtml + '<div class="error-state"><p>加载失败</p><button class="btn btn-outline btn-sm" onclick="switchTab(\'chart\')">重试</button></div>';
    }
  }

  async function loadChartDataInTab(pointID, range, info) {
    try {
      var res = await NHApi.getDeviceHistory(pointID, range);
      if (res.data && res.data.points && res.data.points.length > 0) {
        drawChart(res.data.points, info);
      } else {
        var container = $('.chart-container');
        if (container) {
          container.innerHTML = '<div class="empty-state"><p>暂无数据曲线</p></div>';
        }
      }
    } catch (e) {
      showToast('加载数据曲线失败', 'error');
    }
  }

  // ---------- 历史曲线（独立页面路由保留，重定向到设备详情页的 chart tab） ----------
  async function renderHistory(container, id) {
    // Redirect to device detail with chart tab active
    if (!id) { navigate('devices'); return; }
    deviceDetailActiveTab = 'chart';
    navigate('device/' + id);
  }

  async function doControl(id, action) {
    var actionLabels = { start: '开启', stop: '停止', close: '关闭' };
    showToast('正在' + (actionLabels[action] || action) + '...', 'info');
    try {
      var res = await NHApi.controlDevice(id, action);
      if (res.data && (res.data.status === 'pending' || res.data.status === 'sent' || res.data.status === 'ack' || res.data.status === 'succeeded' || res.data.status === 'created')) {
        showToast(res.data.message || '控制命令已创建，等待设备执行', res.data.status === 'succeeded' ? 'success' : 'info');
        // 刷新详情
        setTimeout(function () { renderPage(); }, 500);
      } else {
        showToast(res.data ? res.data.message : '操作失败', 'error');
      }
    } catch (e) {
      showToast('操作异常，请重试', 'error');
    }
  }

  function drawChart(points, info) {
    var canvas = document.getElementById('history-chart');
    if (!canvas) return;

    if (!points || points.length === 0) {
      var container = $('.chart-container');
      if (container) {
        container.innerHTML = '<div class="empty-state"><p>暂无数据曲线</p></div>';
      }
      return;
    }

    var ctx = canvas.getContext('2d');
    var rect = canvas.getBoundingClientRect();
    var width = Math.max(rect.width || 320, 320);
    var height = Math.max(rect.height || 220, 220);
    var ratio = window.devicePixelRatio || 1;
    canvas.width = Math.floor(width * ratio);
    canvas.height = Math.floor(height * ratio);
    ctx.setTransform(ratio, 0, 0, ratio, 0, 0);
    ctx.clearRect(0, 0, width, height);

    var padding = { left: 44, right: 18, top: 26, bottom: 36 };
    var plotW = width - padding.left - padding.right;
    var plotH = height - padding.top - padding.bottom;
    var values = points.map(function (p) { return Number(p.real_temp); }).filter(function (v) { return Number.isFinite(v); });
    if (values.length === 0) {
      var chartContainer = $('.chart-container');
      if (chartContainer) {
        chartContainer.innerHTML = '<div class="empty-state"><p>暂无数据曲线</p></div>';
      }
      return;
    }

    var min = Math.min.apply(null, values);
    var max = Math.max.apply(null, values);
    if (info.best_min != null) min = Math.min(min, Number(info.best_min));
    if (info.best_max != null) max = Math.max(max, Number(info.best_max));
    if (min === max) {
      min -= 1;
      max += 1;
    }
    var padY = (max - min) * 0.12;
    min -= padY;
    max += padY;

    function xAt(i) {
      return padding.left + (points.length === 1 ? plotW / 2 : (plotW * i / (points.length - 1)));
    }
    function yAt(value) {
      return padding.top + plotH - ((value - min) / (max - min)) * plotH;
    }
    function drawGuide(value, label) {
      if (value == null || !Number.isFinite(Number(value))) return;
      var y = yAt(Number(value));
      ctx.save();
      ctx.strokeStyle = 'rgba(74,124,63,0.35)';
      ctx.setLineDash([5, 5]);
      ctx.beginPath();
      ctx.moveTo(padding.left, y);
      ctx.lineTo(width - padding.right, y);
      ctx.stroke();
      ctx.setLineDash([]);
      ctx.fillStyle = '#4a7c3f';
      ctx.font = '11px sans-serif';
      ctx.fillText(label, padding.left + 4, y - 4);
      ctx.restore();
    }

    ctx.fillStyle = '#f8faf5';
    ctx.fillRect(0, 0, width, height);

    ctx.strokeStyle = '#e2e6da';
    ctx.lineWidth = 1;
    ctx.fillStyle = '#9ca393';
    ctx.font = '11px sans-serif';
    for (var g = 0; g <= 4; g++) {
      var gy = padding.top + plotH * g / 4;
      var gv = max - (max - min) * g / 4;
      ctx.beginPath();
      ctx.moveTo(padding.left, gy);
      ctx.lineTo(width - padding.right, gy);
      ctx.stroke();
      ctx.fillText(gv.toFixed(1), 4, gy + 4);
    }

    drawGuide(info.best_max, '上限');
    drawGuide(info.best_min, '下限');

    ctx.strokeStyle = '#c04a3a';
    ctx.lineWidth = 2;
    ctx.beginPath();
    points.forEach(function (p, i) {
      var value = Number(p.real_temp);
      if (!Number.isFinite(value)) return;
      var x = xAt(i);
      var y = yAt(value);
      if (i === 0) ctx.moveTo(x, y);
      else ctx.lineTo(x, y);
    });
    ctx.stroke();

    ctx.fillStyle = '#5f6758';
    ctx.font = '11px sans-serif';
    ctx.fillText(formatTime(points[0].time), padding.left, height - 10);
    ctx.textAlign = 'right';
    ctx.fillText(formatTime(points[points.length - 1].time), width - padding.right, height - 10);
    ctx.textAlign = 'left';
    ctx.fillStyle = '#2f3a2a';
    ctx.font = '12px sans-serif';
    ctx.fillText((info.func_name || '采集值') + (info.unit ? ' (' + info.unit + ')' : ''), padding.left, 16);
  }

  // ---------- 使用说明 ----------
  function renderHelpCenter(container) {
    var activeSection = normalizeHelpSection(getHashParam('section'));

    container.innerHTML = '<section class="help-intro" aria-labelledby="help-intro-title">' +
      '<span class="section-kicker">FIELD GUIDE</span>' +
      '<h2 id="help-intro-title">从设备接入到现场排障</h2>' +
      '<p>按角色查阅操作路径、报文流向和上线验证标准。</p>' +
      '<span class="help-intro-meta">管理员 · C 端 · 硬件联调</span>' +
    '</section>' +
      '<div class="help-tabs">' +
        helpTab('mini', 'C 端', activeSection) +
        helpTab('flow', '流程图', activeSection) +
        helpTab('admin', '管理员', activeSection) +
        helpTab('hardware', '硬件联调', activeSection) +
        helpTab('checklist', '验证清单', activeSection) +
      '</div>' +
      '<div class="settings-section help-section" id="help-mini">' +
        '<div style="padding:14px 16px;border-bottom:1px solid var(--color-border);font-weight:600;color:var(--color-text)">C 端使用说明</div>' +
        '<div style="padding:12px 16px;color:var(--color-text);font-size:13px;line-height:1.7">' +
          '<p style="margin-top:0">本页只描述"怎么用"，不展示实现代码、服务地址、数据库字段和密钥。</p>' +
          '<p>核心术语：<strong>设备网关</strong> / <strong>采集执行器</strong> / <strong>功能点</strong> / <strong>原始报文</strong>。</p>' +
          '<p>路径建议：设置页 → 使用说明，出现问题后先核对"设备网关是否在线、原始报文是否有 register/keepalive、再看控制命令状态"。</p>' +
        '</div>' +
        '<div style="padding:14px 16px;border-bottom:1px solid var(--color-border);font-weight:600;color:var(--color-text)">C 端操作流程</div>' +
        '<div style="padding:12px 16px;color:var(--color-text-secondary);font-size:13px;line-height:1.8">' +
          '<ol style="padding-left:20px;margin:0">' +
            '<li>打开我的设备查看绑定的采集执行器与功能点</li>' +
            '<li>点击设备查看实时值、状态与更新时间</li>' +
            '<li>可用控制的功能点点击控制后会创建异步命令</li>' +
            '<li>命令状态逐步更新为 pending / sent / ack / succeeded / failed / timeout</li>' +
            '<li>原始报文异常优先到"原始报文排查"做分类与二次回放</li>' +
          '</ol>' +
        '</div>' +
      '</div>' +
      '<div class="settings-section help-section" id="help-flow">' +
        '<div style="padding:14px 16px;border-bottom:1px solid var(--color-border);font-weight:600;color:var(--color-text)">最小流程图</div>' +
        '<div style="padding:12px 16px">' +
          renderDataFlowDiagram() +
          '<div style="height:12px"></div>' +
          renderPacketMatrix() +
          '<div style="height:12px"></div>' +
          renderTimeline() +
        '</div>' +
      '</div>' +
      '<div class="settings-section help-section" id="help-admin">' +
        '<div style="padding:14px 16px;border-bottom:1px solid var(--color-border);font-weight:600;color:var(--color-text)">管理员操作说明</div>' +
        '<div style="padding:12px 16px;color:var(--color-text-secondary);font-size:13px;line-height:1.8">' +
          helpList([
            '先在设备管理中确认设备网关已注册并在线。',
            '把设备网关绑定给农户，再补齐采集执行器和功能点。',
            '联调控制优先使用自定义 Hex，不把未确认协议做成固定按钮。',
            '异常排查先看原始报文，再看控制日志状态。'
          ]) +
        '</div>' +
      '</div>' +
      '<div class="settings-section help-section" id="help-hardware">' +
        '<div style="padding:14px 16px;border-bottom:1px solid var(--color-border);font-weight:600;color:var(--color-text)">硬件联调说明</div>' +
        '<div style="padding:12px 16px;color:var(--color-text-secondary);font-size:13px;line-height:1.8">' +
          helpList([
            '设备网关作为 TCP 客户端接入云端接入层；云端不要求硬件直连 HTTP。',
            '联调先发注册和心跳，再发工作数据、告警、未知包样例。',
            '原始报文会完整留痕，解析字段以最终协议样例为准。',
            '下行控制由平台创建命令，接入层在设备连接存在时写回 TCP。'
          ]) +
        '</div>' +
      '</div>' +
      '<div class="settings-section help-section" id="help-checklist">' +
        '<div style="padding:14px 16px;border-bottom:1px solid var(--color-border);font-weight:600;color:var(--color-text)">上线验证清单</div>' +
        '<div style="padding:12px 16px">' + renderValidationChecklist() + '</div>' +
      '</div>' +
      '<div class="settings-section">' +
        '<div style="padding:14px 16px;border-bottom:1px solid var(--color-border);font-weight:600;color:var(--color-text)">常见问题</div>' +
        '<div style="padding:12px 16px;color:var(--color-text);font-size:13px;line-height:1.7">' +
          '<p style="margin-top:0">设备网关不在线、离线判断按服务端状态心跳时间进行更新；请确认至少有一次近似 24 小时内的 keepalive 上报。</p>' +
          '<p>控制命令未生效时，先确认网关是否从 Adapter 拉取到 pending 并成功写回 TCP。</p>' +
        '</div>' +
      '</div>';

    var sections = container.querySelectorAll('.help-section');
    for (var i = 0; i < sections.length; i++) {
      sections[i].style.display = sections[i].id === 'help-' + activeSection ? '' : 'none';
    }

  }

  // ---------- 设置 ----------
  async function renderSettings(container) {
    container.innerHTML = '<div class="loading-state"><div class="loading-spinner"></div><p>加载用户信息...</p></div>';

    try {
      var res = await NHApi.getMe('mini');
      if (!res.data) {
        container.innerHTML = '<div class="error-state"><p>加载失败</p></div>';
        return;
      }
      var u = res.data;
      currentUser = u;

      var html = '<section class="settings-intro" aria-labelledby="settings-intro-title">' +
        '<span class="section-kicker">ACCOUNT & ACCESS</span>' +
        '<h2 id="settings-intro-title">账户与服务</h2>' +
        '<p>维护个人资料，并进入现场使用说明。</p>' +
      '</section>';

      // ── 我的资料卡片 ──
      var nameChar = (u.display_name || '?').charAt(0);
      html += '<div class="profile-card">' +
        '<div class="profile-card-top">' +
          '<div class="profile-avatar">' + escHtml(nameChar) + '</div>' +
          '<div class="profile-card-meta">' +
            '<div class="profile-card-name">' + escHtml(u.display_name) + '</div>' +
            '<div class="profile-card-bindings">已绑定 ' + u.binding_count + ' 台设备</div>' +
          '</div>' +
          '<button class="btn-pill-edit" id="btn-edit-profile" type="button">编辑资料</button>' +
        '</div>' +
        '<div class="profile-card-divider"></div>' +
        '<div class="profile-card-fields">' +
          profileField('手机号', u.mobile) +
          profileField('地址', u.address) +
          profileField('备注', u.remark) +
        '</div>' +
      '</div>';

      // ── 编辑表单（默认隐藏）──
      html += '<div class="settings-section" id="profile-edit-form" style="display:none">' +
        '<div class="edit-form-header">' +
          '<button class="btn-icon" id="btn-cancel-edit" type="button" aria-label="取消编辑"><svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg></button>' +
          '<h3>编辑资料</h3>' +
          '<div class="edit-form-spacer" aria-hidden="true"></div>' +
        '</div>' +
        '<div class="edit-form-body">' +
          '<div class="form-group">' +
            '<label class="form-label" for="edit-display-name">名称</label>' +
            '<input class="form-input" id="edit-display-name" type="text" placeholder="请输入名称" maxlength="64" value="' + escHtml(u.display_name) + '">' +
            '<p class="form-hint form-hint-left">1-64 个字符</p>' +
          '</div>' +
          '<div class="form-group">' +
            '<label class="form-label" for="edit-mobile">手机号</label>' +
            '<input class="form-input" id="edit-mobile" type="text" placeholder="请输入手机号" maxlength="512" value="' + escHtml(u.mobile) + '">' +
          '</div>' +
          '<div class="form-group">' +
            '<label class="form-label" for="edit-address">地址</label>' +
            '<input class="form-input" id="edit-address" type="text" placeholder="请输入地址" maxlength="512" value="' + escHtml(u.address) + '">' +
          '</div>' +
          '<div class="form-group">' +
            '<label class="form-label" for="edit-remark">备注</label>' +
            '<input class="form-input" id="edit-remark" type="text" placeholder="请输入备注" maxlength="512" value="' + escHtml(u.remark) + '">' +
          '</div>' +
          '<button class="btn btn-primary btn-block" id="btn-save-profile" type="button">保存修改</button>' +
          '<p class="form-hint form-error" id="profile-edit-error" role="alert"></p>' +
        '</div>' +
      '</div>';

      // ── 快速入口：使用说明 ──
      html += '<div class="settings-section settings-actions">' +
        '<button class="settings-action" id="btn-open-help-center" type="button">' +
          '<span class="settings-action-icon" aria-hidden="true">?</span>' +
          '<span class="settings-action-copy"><strong>现场使用说明</strong><small>操作路径、硬件联调与验证清单</small></span>' +
          '<span class="settings-action-chevron" aria-hidden="true">›</span>' +
        '</button>' +
      '</div>';

      // ── 退出登录 ──
      html += '<div class="settings-section settings-actions">' +
        '<button class="settings-action settings-action-danger" type="button" onclick="doLogout()">' +
          '<span class="settings-action-icon" aria-hidden="true">↪</span>' +
          '<span class="settings-action-copy"><strong>退出登录</strong><small>清除当前浏览器的登录状态</small></span>' +
        '</button>' +
      '</div>';

      container.innerHTML = html;

      // 绑定编辑/保存/取消事件
      $('#btn-edit-profile').onclick = function () {
        $('#profile-edit-form').style.display = '';
        this.style.display = 'none';
        $('#edit-display-name').focus();
      };

      $('#btn-open-help-center').onclick = function () {
        navigate('help-center?section=mini');
      };

      $('#btn-cancel-edit').onclick = function () {
        $('#profile-edit-form').style.display = 'none';
        $('#btn-edit-profile').style.display = '';
        $('#btn-edit-profile').focus();
        var errEl = $('#profile-edit-error');
        errEl.style.display = 'none';
        errEl.textContent = '';
        // 恢复原始值
        $('#edit-display-name').value = u.display_name || '';
        $('#edit-mobile').value = u.mobile || '';
        $('#edit-address').value = u.address || '';
        $('#edit-remark').value = u.remark || '';
      };

      $('#btn-save-profile').onclick = async function () {
        var displayName = $('#edit-display-name').value.trim();
        var mobile = $('#edit-mobile').value.trim();
        var address = $('#edit-address').value.trim();
        var remark = $('#edit-remark').value.trim();

        // 前端校验
        if (!displayName || displayName.length < 1 || displayName.length > 64) {
          showToast('名称需要 1-64 个字符', 'error');
          return;
        }
        if (mobile.length > 512) { showToast('手机号最多 512 个字符', 'error'); return; }
        if (address.length > 512) { showToast('地址最多 512 个字符', 'error'); return; }
        if (remark.length > 512) { showToast('备注最多 512 个字符', 'error'); return; }

        var btn = $('#btn-save-profile');
        btn.disabled = true;
        btn.textContent = '保存中...';

        try {
          var saveRes = await NHApi.updateMyProfile({
            display_name: displayName,
            mobile: mobile,
            address: address,
            remark: remark,
          });

          if (saveRes.data) {
            // 更新本地缓存
            u.display_name = saveRes.data.display_name;
            u.mobile = saveRes.data.mobile;
            u.address = saveRes.data.address;
            u.remark = saveRes.data.remark;

            // 更新卡片展示
            var nameChar = (saveRes.data.display_name || '?').charAt(0);
            var avatarEl = document.querySelector('.profile-avatar');
            if (avatarEl) avatarEl.textContent = nameChar;
            var nameEl = document.querySelector('.profile-card-name');
            if (nameEl) nameEl.textContent = saveRes.data.display_name || '';

            // 更新字段
            var fields = document.querySelectorAll('.profile-field-value');
            var fieldValues = [saveRes.data.mobile, saveRes.data.address, saveRes.data.remark];
            for (var i = 0; i < fields.length && i < fieldValues.length; i++) {
              var v = fieldValues[i];
              var hasV = v && v.trim();
              fields[i].textContent = hasV ? v : '未设置';
              if (hasV) { fields[i].classList.remove('is-empty'); }
              else { fields[i].classList.add('is-empty'); }
            }

            // 隐藏编辑表单
            $('#profile-edit-form').style.display = 'none';
            $('#btn-edit-profile').style.display = '';

            showToast('资料已更新', 'success');
          } else {
            var errEl = $('#profile-edit-error');
            errEl.textContent = saveRes.message || '保存失败';
            errEl.style.display = '';
            showToast(saveRes.message || '保存失败', 'error');
          }
        } catch (e) {
          showToast('网络异常，请重试', 'error');
        } finally {
          btn.disabled = false;
          btn.textContent = '保存修改';
        }
      };
    } catch (e) {
      container.innerHTML = '<div class="error-state"><p>加载失败</p><button class="btn btn-outline btn-sm" onclick="renderPage()">重试</button></div>';
    }
  }

  function doLogout() {
    localStorage.removeItem('nh_mini_token');
    localStorage.removeItem('nh_mini_user');
    currentUser = null;
    showToast('已退出登录', 'info');
    navigate('login');
  }

  // ============================================================
  // 辅助
  // ============================================================

  function escHtml(str) {
    if (str == null) return '';
    return String(str).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
  }

  function profileField(label, value) {
    var hasVal = value && value.trim();
    return '<div class="profile-field">' +
      '<span class="profile-field-label">' + escHtml(label) + '</span>' +
      '<span class="profile-field-value' + (hasVal ? '' : ' is-empty') + '">' + (hasVal ? escHtml(value) : '未设置') + '</span>' +
    '</div>';
  }

  // ============================================================
  // 初始化
  // ============================================================

  window.addEventListener('hashchange', renderPage);
  window.addEventListener('load', function () {
    // 版本号
    console.info('%c涅凰智农 IoT (C端) v' + (window.APP_VERSION_STRING || '?'), 'color:#2ecc71;font-weight:bold');

    // 清理旧版共享登录态，C 端只使用 nh_mini_*。
    localStorage.removeItem('user_id');

    renderPage();
  });

  // 暴露到全局
  window.navigate = navigate;
  window.goBack = goBack;
  window.renderPage = renderPage;
  window.doControl = doControl;
  window.doLogout = doLogout;
  window.switchTab = switchTab;

})();
