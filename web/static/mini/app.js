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
        '<div class="form-group">' +
          '<label class="form-label">手机号 / 用户名</label>' +
          '<input class="form-input" id="login-phone" type="text" placeholder="请输入手机号或用户名" value="mock_user_001">' +
        '</div>' +
        '<button class="btn btn-primary btn-block" id="login-submit">登录</button>' +
        '<div class="divider-text">或</div>' +
        '<button class="btn btn-outline btn-block" id="login-mock">模拟登录（测试用）</button>' +
        '<p class="form-hint">测试阶段可使用模拟登录快速进入</p>' +
      '</div>' +
    '</div>';

    var phoneInput = $('#login-phone');
    phoneInput.value = 'mock_user_001';

    $('#login-submit').onclick = function () {
      var phone = phoneInput.value.trim();
      if (!phone) { showToast('请输入手机号或用户名', 'error'); return; }
      doMiniLogin({ mock_openid: phone, mock_nickname: phone });
    };

    $('#login-mock').onclick = function () {
      doMiniLogin({ mock_openid: 'mock_user_001', mock_nickname: '测试农户' });
    };
  }

  async function doMiniLogin(credentials) {
    var btn = $('#login-submit');
    btn.disabled = true;
    btn.textContent = '登录中...';
    try {
      var res = await MockAPI.login('mini', credentials);
      if (res.data && res.data.token) {
        localStorage.setItem('nh_mini_token', res.data.token);
        localStorage.setItem('user_id', res.data.user_id);
        MockAPI.setUserId(res.data.user_id);
        showToast('登录成功', 'success');
        navigate('devices');
      } else {
        showToast(res.message || '登录失败', 'error');
      }
    } catch (e) {
      showToast('网络异常，请重试', 'error');
    } finally {
      btn.disabled = false;
      btn.textContent = '登录';
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

      var res = await MockAPI.getDevices();

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
        var statusClass = 'status-' + d.status;
        var statusTextClass = 'status-text-' + d.status;
        html += '<div class="device-card ' + statusClass + '" onclick="navigate(\'device/' + d.de_re_id + '\')">' +
          '<div class="device-card-header">' +
            '<span class="device-card-title">' + escHtml(d.shed_no) + ' · ' + escHtml(d.collector_no) + '</span>' +
            '<span class="device-card-func">' + escHtml(d.func_name) + '</span>' +
          '</div>' +
          '<div class="device-card-body">' +
            '<span class="device-card-value">' + (d.real_value != null ? d.real_value : '--') + '</span>' +
            '<span class="device-card-unit">' + escHtml(d.unit) + '</span>' +
          '</div>' +
          '<div class="device-card-meta">' +
            '<span class="device-card-status">' +
              '<span class="status-dot"></span>' +
              '<span class="' + statusTextClass + '">' + escHtml(d.status_label) + '</span>' +
            '</span>' +
            '<span>' + formatTime(d.updated_at) + '</span>' +
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
      var res = await MockAPI.getDeviceRealtime(id);
      if (!res.data) {
        container.innerHTML = '<div class="error-state"><p>设备不存在</p><button class="btn btn-outline btn-sm" onclick="navigate(\'devices\')">返回列表</button></div>';
        return;
      }
      var d = res.data;

      var controlHtml = '';
      if (d.func === 4 || d.func === 5 || d.func === 6) {
        // 水泵、风机、继电器：开启/关闭
        controlHtml = '<div class="detail-section">' +
          '<h3>设备控制</h3>' +
          '<div class="detail-controls">' +
            '<button class="btn btn-primary" onclick="doControl(' + d.de_re_id + ',\'start\')" ' + (d.status === 'on' ? 'disabled' : '') + '>开启</button>' +
            '<button class="btn btn-outline" onclick="doControl(' + d.de_re_id + ',\'stop\')" ' + (d.status === 'off' ? 'disabled' : '') + '>停止</button>' +
            '<button class="btn btn-danger" onclick="doControl(' + d.de_re_id + ',\'close\')" ' + (d.status === 'off' ? 'disabled' : '') + '>关闭</button>' +
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

      var statusClass = 'status-' + d.status;
      var statusTextClass = 'status-text-' + d.status;

      var html = '<div class="detail-section">' +
        '<h3>实时数据</h3>' +
        '<div class="detail-real-value">' +
          '<span class="big-value">' + (d.real_value != null ? d.real_value : '--') + '</span>' +
          '<span class="big-unit">' + escHtml(d.unit) + '</span>' +
          '<p class="range-hint">' + rangeHint + '</p>' +
          '<p style="margin-top:4px"><span class="device-card-status"><span class="status-dot"></span><span class="' + statusTextClass + '">' + escHtml(d.status_label) + '</span></span></p>' +
        '</div>' +
      '</div>' +

      '<div class="detail-section">' +
        '<h3>基本信息</h3>' +
        '<div class="detail-grid">' +
          '<div class="detail-item"><div class="detail-item-label">棚号</div><div class="detail-item-value">' + escHtml(d.shed_no) + '</div></div>' +
          '<div class="detail-item"><div class="detail-item-label">设备序号</div><div class="detail-item-value">' + escHtml(d.collector_no) + '</div></div>' +
          '<div class="detail-item"><div class="detail-item-label">频点</div><div class="detail-item-value">' + d.freq + '</div></div>' +
          '<div class="detail-item"><div class="detail-item-label">设备 ID</div><div class="detail-item-value">' + d.devid + '</div></div>' +
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
        '<button class="btn btn-outline btn-block" onclick="navigate(\'history/' + d.de_re_id + '\')">查看历史曲线</button>' +
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
      var res = await MockAPI.controlDevice(id, action);
      if (res.data && res.data.status === 'succeeded') {
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
  var chartInstance = null;

  async function renderHistory(container, id) {
    if (!id) { navigate('devices'); return; }
    container.innerHTML = '<div class="loading-state"><div class="loading-spinner"></div><p>加载历史数据...</p></div>';

    // 加载 Chart.js
    if (typeof Chart === 'undefined') {
      await loadScript('https://cdn.jsdelivr.net/npm/chart.js@4.4.0/dist/chart.umd.min.js');
    }

    try {
      var res = await MockAPI.getDeviceHistory(id, '24h');
      if (!res.data) {
        container.innerHTML = '<div class="error-state"><p>设备不存在</p><button class="btn btn-outline btn-sm" onclick="navigate(\'device/' + id + '\')">返回详情</button></div>';
        return;
      }

      var d = res.data;
      var points = d.points || [];
      var activeRange = '24h';

      var html = '<div class="range-tabs">' +
        '<button class="range-tab" data-range="1h">近1小时</button>' +
        '<button class="range-tab active" data-range="24h">近24小时</button>' +
        '<button class="range-tab" data-range="7d">近7天</button>' +
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
      var res = await MockAPI.getDeviceHistory(id, range);
      if (res.data && res.data.points) {
        drawChart(res.data.points, info);
      }
    } catch (e) {
      showToast('加载历史数据失败', 'error');
    }
  }

  function drawChart(points, info) {
    if (chartInstance) { chartInstance.destroy(); chartInstance = null; }

    var ctx = document.getElementById('history-chart');
    if (!ctx) return;

    if (!points || points.length === 0) {
      var container = $('.chart-container');
      container.innerHTML = '<div class="empty-state"><p>暂无历史数据</p></div>';
      return;
    }

    var labels = points.map(function (p) { return formatTime(p.time); });
    var temps = points.map(function (p) { return p.real_temp; });
    var humidities = points.map(function (p) { return p.real_humidity; });

    var datasets = [{
      label: '温度 (°C)',
      data: temps,
      borderColor: '#c04a3a',
      backgroundColor: 'rgba(192,74,58,0.08)',
      fill: true,
      tension: 0.3,
      pointRadius: 0,
      borderWidth: 1.5,
    }];

    if (info.func_name === '墒情') {
      datasets = [{
        label: '墒情 (%)',
        data: humidities,
        borderColor: '#3a6b8c',
        backgroundColor: 'rgba(58,107,140,0.08)',
        fill: true,
        tension: 0.3,
        pointRadius: 0,
        borderWidth: 1.5,
      }];
    }

    // 添加适宜范围线
    if (info.best_min != null && info.best_max != null) {
      datasets.push({
        label: '适宜上限',
        data: points.map(function () { return info.best_max; }),
        borderColor: 'rgba(74,124,63,0.4)',
        borderDash: [4, 4],
        borderWidth: 1,
        pointRadius: 0,
        fill: false,
      });
      datasets.push({
        label: '适宜下限',
        data: points.map(function () { return info.best_min; }),
        borderColor: 'rgba(74,124,63,0.4)',
        borderDash: [4, 4],
        borderWidth: 1,
        pointRadius: 0,
        fill: false,
      });
    }

    chartInstance = new Chart(ctx, {
      type: 'line',
      data: { labels: labels, datasets: datasets },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        interaction: { intersect: false, mode: 'index' },
        plugins: {
          legend: {
            display: true,
            position: 'bottom',
            labels: { boxWidth: 12, padding: 16, font: { size: 11 }, usePointStyle: true },
          },
        },
        scales: {
          x: {
            ticks: { maxTicksLimit: 8, font: { size: 10 }, color: '#9ca393' },
            grid: { display: false },
          },
          y: {
            ticks: { font: { size: 10 }, color: '#9ca393' },
            grid: { color: '#e2e6da' },
          },
        },
      },
    });
  }

  // ---------- 设置 ----------
  async function renderSettings(container) {
    container.innerHTML = '<div class="loading-state"><div class="loading-spinner"></div><p>加载用户信息...</p></div>';

    try {
      var res = await MockAPI.getMe('mini');
      if (!res.data) {
        container.innerHTML = '<div class="error-state"><p>加载失败</p></div>';
        return;
      }
      var u = res.data;
      currentUser = u;

      var html = '';

      // ── 我的资料卡片 ──
      html += '<div class="settings-section">' +
        '<h3 style="margin:0 0 12px 0;font-size:16px;font-weight:600">我的资料</h3>' +
        '<div class="settings-item"><span class="settings-item-label">名称</span><span class="settings-item-value" id="profile-display-name">' + escHtml(u.display_name) + '</span></div>' +
        '<div class="settings-item"><span class="settings-item-label">手机号</span><span class="settings-item-value" id="profile-mobile">' + escHtml(u.mobile) + '</span></div>' +
        '<div class="settings-item"><span class="settings-item-label">地址</span><span class="settings-item-value" id="profile-address">' + escHtml(u.address) + '</span></div>' +
        '<div class="settings-item"><span class="settings-item-label">备注</span><span class="settings-item-value" id="profile-remark">' + escHtml(u.remark) + '</span></div>' +
        '<div class="settings-item"><span class="settings-item-label">角色</span><span class="settings-item-value">' + (u.role === 3 ? '农户' : '其他') + '</span></div>' +
        '<div class="settings-item"><span class="settings-item-label">已绑定设备</span><span class="settings-item-value">' + u.binding_count + ' 台</span></div>' +
        '<button class="btn btn-outline btn-block" id="btn-edit-profile" style="margin-top:12px">编辑资料</button>' +
      '</div>';

      // ── 编辑表单（默认隐藏）──
      html += '<div class="settings-section" id="profile-edit-form" style="display:none">' +
        '<h3 style="margin:0 0 12px 0;font-size:16px;font-weight:600">编辑资料</h3>' +
        '<div class="form-group">' +
          '<label class="form-label">名称</label>' +
          '<input class="form-input" id="edit-display-name" type="text" placeholder="请输入名称" maxlength="64" value="' + escHtml(u.display_name) + '">' +
          '<p class="form-hint">1-64 个字符</p>' +
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
        '<div style="display:flex;gap:8px;margin-top:12px">' +
          '<button class="btn btn-primary" id="btn-save-profile" style="flex:1">保存</button>' +
          '<button class="btn btn-outline" id="btn-cancel-edit" style="flex:1">取消</button>' +
        '</div>' +
        '<p class="form-hint" id="profile-edit-error" style="color:var(--color-danger);display:none"></p>' +
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
          var saveRes = await MockAPI.updateMyProfile({
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
            $('#profile-display-name').textContent = saveRes.data.display_name || '';
            $('#profile-mobile').textContent = saveRes.data.mobile || '';
            $('#profile-address').textContent = saveRes.data.address || '';
            $('#profile-remark').textContent = saveRes.data.remark || '';

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
    localStorage.removeItem('user_id');
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

  function loadScript(src) {
    return new Promise(function (resolve, reject) {
      var s = document.createElement('script');
      s.src = src;
      s.onload = resolve;
      s.onerror = reject;
      document.head.appendChild(s);
    });
  }

  // ============================================================
  // 初始化
  // ============================================================

  window.addEventListener('hashchange', renderPage);
  window.addEventListener('load', function () {
    // 版本号
    console.info('%c涅凰智农 IoT (C端) v' + (window.APP_VERSION_STRING || '?'), 'color:#2ecc71;font-weight:bold');

    // 恢复 token
    var token = localStorage.getItem('nh_mini_token');
    var uid = localStorage.getItem('user_id');
    if (token && uid) {
      MockAPI.setUserId(parseInt(uid));
    }
    renderPage();
  });

  // 暴露到全局
  window.navigate = navigate;
  window.goBack = goBack;
  window.renderPage = renderPage;
  window.doControl = doControl;
  window.doLogout = doLogout;

})();
