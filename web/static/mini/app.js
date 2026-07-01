/**
 * 涅凰智农 C 端 — 应用逻辑
 * Hash 路由： #login | #devices | #device/:id | #history/:id | #settings
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
    $('#topbar-back').style.display = showBack ? '' : 'none';
    $('#btn-settings').style.display = (currentPage === 'devices' || currentPage === 'settings') ? '' : 'none';
  }

  function parseHash() {
    var hash = window.location.hash.replace('#', '') || 'devices';
    var parts = hash.split('/');
    return { page: parts[0], params: parts.slice(1) };
  }

  // ============================================================
  // 页面渲染
  // ============================================================

  function renderPage() {
    var route = parseHash();
    currentPage = route.page;
    var container = $('#main-content');

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
      default: renderDeviceList(container); updateTopbar('我的设备', false);
    }
  }

  // ---------- 登录页 ----------
  function renderLogin(container) {
    container.innerHTML = '<div class="login-page">' +
      '<div class="login-card">' +
        '<div class="login-logo">' +
          '<h2>涅凰智农</h2>' +
          '<p>智慧农业物联网云平台</p>' +
        '</div>' +
        '<div class="empty-state" style="padding:8px 0 18px">' +
          '<p style="font-size:14px;color:var(--color-text-secondary);line-height:1.7">' +
            '浏览器 H5 不支持微信 wx.login，请在微信小程序内完成真实登录。' +
          '</p>' +
        '</div>' +
        '<div class="form-group">' +
          '<label class="form-label">H5 调试 Token</label>' +
          '<input class="form-input" id="login-token" type="text" placeholder="粘贴真实 nh_mini_token">' +
          '<p class="form-hint" style="text-align:left">仅用于浏览器调试真实接口；不会调用开发登录接口，也不会生成本地模拟用户。</p>' +
        '</div>' +
        '<button class="btn btn-primary btn-block" id="login-submit">使用 Token 调试</button>' +
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
      btn.textContent = '使用 Token 调试';
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
      var html = '<div class="page-header"><h2>我的设备</h2><span class="device-count">共 ' + devices.length + ' 台</span></div>';
      html += '<div class="device-list">';
      devices.forEach(function (d) {
        var pointID = devicePointID(d);
        var status = valueStatusClass(d.value_status);
        var statusClass = 'status-' + status;
        var statusTextClass = 'status-text-' + status;
        var title = escHtml(d.shed_no || '未设置棚号') + ' · 频点' + escHtml(d.freq);
        var deviceNo = '设备ID ' + escHtml(d.devid || '--');
        html += '<div class="device-card ' + statusClass + '" onclick="navigate(\'device/' + pointID + '\')">' +
          '<div class="device-card-header">' +
            '<span class="device-card-title">' + title + '</span>' +
            '<span class="device-card-func">' + escHtml(d.func_name) + '</span>' +
          '</div>' +
          '<div class="device-card-body">' +
            '<span class="device-card-value">' + (d.real_value != null ? d.real_value : '--') + '</span>' +
            '<span class="device-card-unit">' + escHtml(d.unit) + '</span>' +
          '</div>' +
          '<div class="device-card-meta">' +
            '<span class="device-card-status">' +
              '<span class="status-dot"></span>' +
              '<span class="' + statusTextClass + '">' + escHtml(d.value_status_label || '--') + '</span>' +
            '</span>' +
            '<span>' + deviceNo + ' · ' + formatTime(d.updated_at) + '</span>' +
          '</div>' +
        '</div>';
      });
      html += '</div>';

      // 添加底部提示
      html += '<p style="text-align:center;margin-top:16px;font-size:11px;color:var(--color-text-muted)">下拉刷新可获取最新数据</p>';

      container.innerHTML = html;
    } catch (e) {
      console.error('[C端] 设备列表加载失败:', e);
      container.innerHTML = '<div class="error-state"><p>加载失败：' + escHtml(e.message || '网络异常') + '</p><button class="btn btn-outline btn-sm" onclick="renderPage()">重试</button></div>';
    }
  }

  // ---------- 设备详情 ----------
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

      var controlHtml = '';
      if (d.func === 4 || d.func === 5 || d.func === 6) {
        // 水泵、风机、继电器：开启/关闭
        controlHtml = '<div class="detail-section">' +
          '<h3>设备控制</h3>' +
          '<div class="detail-controls">' +
            '<button class="btn btn-primary" onclick="doControl(\'' + pointID + '\',\'start\')">开启</button>' +
            '<button class="btn btn-outline" onclick="doControl(\'' + pointID + '\',\'stop\')">停止</button>' +
            '<button class="btn btn-danger" onclick="doControl(\'' + pointID + '\',\'close\')">关闭</button>' +
          '</div>' +
        '</div>';
      } else if (d.func === 1 || d.func === 2) {
        // 温度/墒情：显示状态即可，无控制
        controlHtml = '<div class="detail-section">' +
          '<h3>设备状态</h3>' +
          '<p style="font-size:13px;color:var(--color-text-muted)">' +
            '运行模式：<strong>' + escHtml(d.run_mode_label) + '</strong> · 采集周期：' + (d.poll_interval || '--') + 's' +
          '</p>' +
        '</div>';
      } else {
        // 限位等：只读
        controlHtml = '<div class="detail-section">' +
          '<h3>设备状态</h3>' +
          '<p style="font-size:13px;color:var(--color-text-muted)">当前值：' + (d.real_value != null ? d.real_value : '--') + '</p>' +
        '</div>';
      }

      var rangeHint = '';
      if (d.best_min != null && d.best_max != null) {
        rangeHint = '适宜范围：' + d.best_min + ' ~ ' + d.best_max + ' ' + escHtml(d.unit);
      }

      var status = valueStatusClass(d.value_status);
      var statusClass = 'status-' + status;
      var statusTextClass = 'status-text-' + status;

      var html = '<div class="detail-section">' +
        '<h3>实时数据</h3>' +
        '<div class="detail-real-value">' +
          '<span class="big-value">' + (d.real_value != null ? d.real_value : '--') + '</span>' +
          '<span class="big-unit">' + escHtml(d.unit) + '</span>' +
          '<p class="range-hint">' + rangeHint + '</p>' +
          '<p style="margin-top:4px"><span class="device-card-status ' + statusClass + '"><span class="status-dot"></span><span class="' + statusTextClass + '">' + escHtml(d.value_status_label || '--') + '</span></span></p>' +
        '</div>' +
      '</div>' +

      '<div class="detail-section">' +
        '<h3>基本信息</h3>' +
        '<div class="detail-grid">' +
          '<div class="detail-item"><div class="detail-item-label">棚号</div><div class="detail-item-value">' + escHtml(d.shed_no) + '</div></div>' +
          '<div class="detail-item"><div class="detail-item-label">设备 ID</div><div class="detail-item-value">' + escHtml(d.devid || '--') + '</div></div>' +
          '<div class="detail-item"><div class="detail-item-label">频点</div><div class="detail-item-value">' + d.freq + '</div></div>' +
          '<div class="detail-item"><div class="detail-item-label">Point ID</div><div class="detail-item-value">' + escHtml(pointID) + '</div></div>' +
          '<div class="detail-item"><div class="detail-item-label">功能</div><div class="detail-item-value">' + escHtml(d.func_name) + '</div></div>' +
          '<div class="detail-item"><div class="detail-item-label">运行模式</div><div class="detail-item-value">' + escHtml(d.run_mode_label) + '</div></div>' +
        '</div>' +
      '</div>' +

      controlHtml +

      '<div class="detail-section">' +
        '<h3>最近更新</h3>' +
        '<p style="font-size:13px;color:var(--color-text-secondary)">' + formatFullTime(d.updated_at) + '</p>' +
      '</div>' +

      '<div style="margin-top:8px">' +
        '<button class="btn btn-outline btn-block" onclick="navigate(\'history/' + pointID + '\')">查看历史曲线</button>' +
      '</div>';

      container.innerHTML = html;
    } catch (e) {
      container.innerHTML = '<div class="error-state"><p>加载失败</p><button class="btn btn-outline btn-sm" onclick="renderPage()">重试</button></div>';
    }
  }

  async function doControl(id, action) {
    var actionLabels = { start: '开启', stop: '停止', close: '关闭' };
    showToast('正在' + (actionLabels[action] || action) + '...', 'info');
    try {
      var res = await NHApi.controlDevice(id, action);
      if (res.data && (res.data.status === 'succeeded' || res.data.status === 'created')) {
        showToast(res.data.message || '操作成功', 'success');
        // 刷新详情
        setTimeout(function () { renderPage(); }, 500);
      } else {
        showToast(res.data ? res.data.message : '操作失败', 'error');
      }
    } catch (e) {
      showToast('操作异常，请重试', 'error');
    }
  }

  // ---------- 历史曲线 ----------
  async function renderHistory(container, id) {
    if (!id) { navigate('devices'); return; }
    container.innerHTML = '<div class="loading-state"><div class="loading-spinner"></div><p>加载历史数据...</p></div>';

    try {
      var res = await NHApi.getDeviceHistory(id, '1d');
      if (!res.data) {
        container.innerHTML = '<div class="error-state"><p>设备不存在</p><button class="btn btn-outline btn-sm" onclick="navigate(\'device/' + id + '\')">返回详情</button></div>';
        return;
      }

      var d = res.data;
      var points = d.points || [];
      var activeRange = '1d';

      var html = '<div class="range-tabs">' +
        '<button class="range-tab active" data-range="1d">近1天</button>' +
        '<button class="range-tab" data-range="3d">近3天</button>' +
        '<button class="range-tab" data-range="7d">近7天</button>' +
        '<button class="range-tab" data-range="30d">近30天</button>' +
      '</div>' +
      '<div class="chart-container">' +
        '<canvas id="history-chart"></canvas>' +
      '</div>' +
      '<p style="text-align:center;margin-top:12px;font-size:13px;color:var(--color-text-muted)">' +
        '设备：' + escHtml(d.func_name) + ' · 单位：' + escHtml(d.unit) +
        ' · 适宜范围：' + d.best_min + ' ~ ' + d.best_max + ' ' + escHtml(d.unit) +
      '</p>';

      container.innerHTML = html;

      // 绑定 range tab 点击
      $$('.range-tab').forEach(function (tab) {
        tab.onclick = function () {
          var range = this.dataset.range;
          if (range === activeRange) return;
          activeRange = range;
          $$('.range-tab').forEach(function (t) { t.classList.remove('active'); });
          this.classList.add('active');
          loadHistoryData(id, range, d);
        };
      });

      drawChart(points, d);
    } catch (e) {
      container.innerHTML = '<div class="error-state"><p>加载失败</p><button class="btn btn-outline btn-sm" onclick="renderPage()">重试</button></div>';
    }
  }

  async function loadHistoryData(id, range, info) {
    try {
      var res = await NHApi.getDeviceHistory(id, range);
      if (res.data && res.data.points) {
        drawChart(res.data.points, info);
      }
    } catch (e) {
      showToast('加载历史数据失败', 'error');
    }
  }

  function drawChart(points, info) {
    var canvas = document.getElementById('history-chart');
    if (!canvas) return;

    if (!points || points.length === 0) {
      var container = $('.chart-container');
      container.innerHTML = '<div class="empty-state"><p>暂无历史数据</p></div>';
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
      $('.chart-container').innerHTML = '<div class="empty-state"><p>暂无历史数据</p></div>';
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

      var html = '';

      // ── 我的资料卡片 ──
      var nameChar = (u.display_name || '?').charAt(0);
      html += '<div class="profile-card">' +
        '<div class="profile-card-top">' +
          '<div class="profile-avatar">' + escHtml(nameChar) + '</div>' +
          '<div class="profile-card-meta">' +
            '<div class="profile-card-name">' + escHtml(u.display_name) + '</div>' +
            '<div class="profile-card-bindings">已绑定 ' + u.binding_count + ' 台设备</div>' +
          '</div>' +
          '<button class="btn-pill-edit" id="btn-edit-profile">编辑资料</button>' +
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
          '<button class="btn-icon" id="btn-cancel-edit"><svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg></button>' +
          '<h3 style="font-size:17px;font-weight:600">编辑资料</h3>' +
          '<div style="width:36px"></div>' +
        '</div>' +
        '<div class="edit-form-body">' +
          '<div class="form-group">' +
            '<label class="form-label">名称</label>' +
            '<input class="form-input" id="edit-display-name" type="text" placeholder="请输入名称" maxlength="64" value="' + escHtml(u.display_name) + '">' +
            '<p class="form-hint" style="text-align:left">1-64 个字符</p>' +
          '</div>' +
          '<div class="form-group">' +
            '<label class="form-label">手机号</label>' +
            '<input class="form-input" id="edit-mobile" type="text" placeholder="请输入手机号" maxlength="512" value="' + escHtml(u.mobile) + '">' +
          '</div>' +
          '<div class="form-group">' +
            '<label class="form-label">地址</label>' +
            '<input class="form-input" id="edit-address" type="text" placeholder="请输入地址" maxlength="512" value="' + escHtml(u.address) + '">' +
          '</div>' +
          '<div class="form-group">' +
            '<label class="form-label">备注</label>' +
            '<input class="form-input" id="edit-remark" type="text" placeholder="请输入备注" maxlength="512" value="' + escHtml(u.remark) + '">' +
          '</div>' +
          '<button class="btn btn-primary btn-block" id="btn-save-profile">保存</button>' +
          '<p class="form-hint" id="profile-edit-error" style="color:var(--color-danger);display:none;margin-top:12px"></p>' +
        '</div>' +
      '</div>';

      // ── 退出登录 ──
      html += '<div class="settings-section">' +
        '<div class="settings-item"><span class="settings-item-danger" onclick="doLogout()">退出登录</span></div>' +
      '</div>';

      container.innerHTML = html;

      // 绑定编辑/保存/取消事件
      $('#btn-edit-profile').onclick = function () {
        $('#profile-edit-form').style.display = '';
        this.style.display = 'none';
      };

      $('#btn-cancel-edit').onclick = function () {
        $('#profile-edit-form').style.display = 'none';
        $('#btn-edit-profile').style.display = '';
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
          btn.textContent = '保存';
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

})();
