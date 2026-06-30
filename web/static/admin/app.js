/**
 * 涅凰智农 管理端 — 应用逻辑
 * Hash 路由： #login | #dashboard | #users | #gateways | #bindings | #logs
 */

(function () {
  'use strict';

  // ============================================================
  // 状态
  // ============================================================
  var currentPage = '';

  // ============================================================
  // 工具
  // ============================================================
  function $(sel) { return document.querySelector(sel); }
  function $$(sel) { return document.querySelectorAll(sel); }

  function showToast(msg, type) {
    type = type || 'info';
    var el = $('#toast');
    el.textContent = msg;
    el.className = 'toast ' + type + ' show';
    clearTimeout(el._timeout);
    el._timeout = setTimeout(function () { el.className = 'toast'; }, 2500);
  }

  function formatTime(iso) {
    if (!iso) return '--';
    try {
      var d = new Date(iso);
      var pad = function (n) { return n.toString().padStart(2, '0'); };
      return pad(d.getMonth() + 1) + '-' + pad(d.getDate()) + ' ' + pad(d.getHours()) + ':' + pad(d.getMinutes());
    } catch (e) { return iso; }
  }

  function formatFullTime(iso) {
    if (!iso) return '--';
    try {
      var d = new Date(iso);
      var pad = function (n) { return n.toString().padStart(2, '0'); };
      return d.getFullYear() + '-' + pad(d.getMonth() + 1) + '-' + pad(d.getDate()) + ' ' + pad(d.getHours()) + ':' + pad(d.getMinutes()) + ':' + pad(d.getSeconds());
    } catch (e) { return iso; }
  }

  function escHtml(str) {
    if (str == null) return '';
    return String(str).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
  }

  // ============================================================
  // 导航
  // ============================================================
  function navigate(page) {
    window.location.hash = '#' + page;
  }

  function setActiveNav(page) {
    $$('.nav-item').forEach(function (item) {
      item.classList.toggle('active', item.dataset.page === page);
    });
  }

  function updatePageTitle(title) {
    $('#topbar-page-title').textContent = title;
  }

  function parseHash() {
    var hash = window.location.hash.replace('#', '') || 'dashboard';
    var idx = hash.indexOf('?');
    var page = idx >= 0 ? hash.substring(0, idx) : hash;
    return { page: page, raw: hash };
  }

  function getHashParam(name) {
    var hash = location.hash;
    var idx = hash.indexOf('?');
    if (idx < 0) return null;
    var params = new URLSearchParams(hash.substring(idx + 1));
    return params.get(name);
  }

  function goFarmerDetail(userId) {
    location.hash = 'farmer-detail?user_id=' + userId;
  }

  // ============================================================
  // 页面渲染
  // ============================================================
  function renderPage() {
    var route = parseHash();
    currentPage = route.page;
    var container = $('#main-content');

    if (route.page !== 'login' && !localStorage.getItem('nh_admin_token')) {
      // 检测是否有 mini token 但没有 admin token → 提示权限不足
      if (localStorage.getItem('nh_mini_token')) {
        showToast('无管理员权限，请使用管理员账号登录', 'error');
      }
      navigate('login');
      return;
    }

    setActiveNav(route.page);

    switch (route.page) {
      case 'login': renderLogin(container); updatePageTitle('登录'); break;
      case 'dashboard': renderDashboard(container); updatePageTitle('Dashboard'); break;
      case 'users': renderUsers(container); updatePageTitle('用户管理'); break;
      case 'gateways': renderGateways(container); updatePageTitle('设备管理'); break;
      case 'bindings': renderBindings(container); updatePageTitle('绑定管理'); break;
      case 'farmer-detail': renderFarmerDetail(container); updatePageTitle('农户详情'); break;
      case 'logs': renderLogs(container); updatePageTitle('控制日志'); break;
      case 'payload-events': renderPayloadEvents(container); updatePageTitle('Payload 排查'); break;
      default: renderDashboard(container); updatePageTitle('Dashboard');
    }
  }

  // ---------- 登录 ----------
  function renderLogin(container) {
    $('#sidebar').style.display = 'none';
    container.innerHTML =
      '<div style="display:flex;align-items:center;justify-content:center;min-height:80vh">' +
        '<div style="width:360px;background:var(--color-surface);border-radius:var(--radius-lg);box-shadow:var(--shadow-md);padding:36px 28px">' +
          '<div style="text-align:center;margin-bottom:24px">' +
            '<h2 style="font-size:22px;font-weight:700;color:var(--color-primary-dark)">涅凰智农</h2>' +
            '<p style="font-size:13px;color:var(--color-text-muted);margin-top:4px">管理后台</p>' +
          '</div>' +
          '<div class="form-group">' +
            '<label class="form-label">用户名</label>' +
            '<input class="form-input" id="login-username" type="text" placeholder="请输入用户名">' +
          '</div>' +
          '<div class="form-group">' +
            '<label class="form-label">密码</label>' +
            '<input class="form-input" id="login-password" type="password" placeholder="请输入密码">' +
          '</div>' +
          '<button class="btn btn-primary btn-block" id="login-submit" style="margin-top:8px">登录</button>' +
        '</div>' +
      '</div>';

    // Safari 安全策略：innerHTML 动态插入的 password input 会丢失 value
    // 必须通过 JS 显式设置，不能依赖 HTML value 属性
    var usernameInput = $('#login-username');
    var passwordInput = $('#login-password');
    usernameInput.value = '';
    passwordInput.value = '';

    $('#login-submit').onclick = function () {
      // 每次点击时重新从 DOM 获取值（避免 Safari 引用丢失问题）
      var username = ($('#login-username') || {}).value || '';
      var password = ($('#login-password') || {}).value || '';
      username = (username || '').trim();
      password = (password || '').trim();
      console.debug('[Login] username=', username, 'password_len=', password.length);
      if (!username || !password) { showToast('请输入用户名和密码', 'error'); return; }
      doAdminLogin(username, password);
    };

    // 回车登录
    passwordInput.onkeydown = function (e) {
      if (e.key === 'Enter') $('#login-submit').click();
    };
  }

  async function doAdminLogin(username, password) {
    var btn = $('#login-submit');
    btn.disabled = true;
    btn.textContent = '登录中...';
    try {
      var res = await NHApi.loginAdmin({ username: username, password: password });
      if (res.data && res.data.token) {
        localStorage.setItem('nh_admin_token', res.data.token);
        localStorage.setItem('nh_admin_user', JSON.stringify(res.data));
        updateSidebarUser(res.data);
        showToast('登录成功', 'success');
        $('#sidebar').style.display = '';
        navigate('dashboard');
      } else {
        showToast(res.message || '登录失败', 'error');
      }
    } catch (e) {
      // 显示具体错误原因，方便调试（CORS/超时/密码错误等）
      var msg = e.message || '网络异常，请重试';
      console.error('[AdminLogin] 登录失败:', e);
      showToast(msg, 'error');
    } finally {
      btn.disabled = false;
      btn.textContent = '登录';
    }
  }

  function updateSidebarUser(user) {
    if (!user) return;
    var initial = (user.display_name || '管').charAt(0);
    $('#sidebar-user-avatar').textContent = initial;
    $('#sidebar-user-name').textContent = user.display_name;
    $('#sidebar-user-role').textContent = user.role === 1 ? '超级管理员' : '管理员';
  }

  function doLogout() {
    localStorage.removeItem('nh_admin_token');
    localStorage.removeItem('nh_admin_user');
    showToast('已退出登录', 'info');
    navigate('login');
  }

  // ---------- Dashboard ----------
  var currentDashboardRange = '24h';
  var currentDashboardData = null;

  async function renderDashboard(container) {
    container.innerHTML = '<div class="loading-state"><div class="loading-spinner"></div><p>加载仪表盘...</p></div>';
    try {
      var res = await NHApi.getDashboard(currentDashboardRange);
      if (!res.data) { container.innerHTML = '<div class="error-state"><p>加载失败</p><button class="btn btn-outline btn-sm" onclick="renderPage()">重试</button></div>'; return; }
      var d = res.data;
      currentDashboardData = d;

      // 后端返回结构：data.users / data.devices / data.ingest / data.control
      var users = d.users || {};
      var devices = d.devices || {};
      var ingest = d.ingest || {};
      var control = d.control || {};

      var rangeLabels = {
        '1h': '1 小时',
        '6h': '6 小时',
        '24h': '24 小时',
        '7d': '一周',
      };
      var ranges = ['1h', '6h', '24h', '7d'];
      var rangeBtns = ranges.map(function (r) {
        var active = r === (d.range || '24h') ? ' btn-primary' : ' btn-outline';
        return '<button class="btn btn-sm' + active + '" onclick="switchDashboardRange(\'' + r + '\')">' + (rangeLabels[r] || r) + '</button>';
      }).join('');

      var html = '<div class="page-header" style="display:flex;align-items:center;justify-content:space-between">' +
        '<h2>Dashboard</h2>' +
        '<div style="display:flex;gap:6px">' + rangeBtns + '</div>' +
      '</div>' +
      '<div class="stat-grid">' +
        '<div class="stat-card"><div class="stat-card-label">在线云边设备</div><div class="stat-card-value">' + (devices.gateway_online || 0) + '<span style="font-size:16px;font-weight:400;color:var(--color-text-muted)"> / ' + (devices.gateway_total || 0) + '</span></div></div>' +
        '<div class="stat-card"><div class="stat-card-label">采集执行器</div><div class="stat-card-value">' + (devices.collector_node_total || 0) + '</div><div class="stat-card-sub">功能点数 ' + (devices.device_point_total || 0) + '</div></div>' +
        '<div class="stat-card"><div class="stat-card-label">农户数量</div><div class="stat-card-value">' + (users.active_farmer_count || 0) + '</div><div class="stat-card-sub">管理员 ' + (users.admin_count || 0) + '</div></div>' +
        '<div class="stat-card"><div class="stat-card-label">数据上报</div><div class="stat-card-value">' + ((ingest.raw_event_count || 0) + (ingest.telemetry_count || 0)).toLocaleString() + '</div><div class="stat-card-sub">最近上报：' + formatTime(devices.latest_report_time) + '</div></div>' +
      '</div>';

      // 第二行：设备状态 + 控制统计
      html += '<div class="stat-grid" style="margin-top:16px">' +
        '<div class="stat-card"><div class="stat-card-label">设备状态</div><div class="stat-card-value">' +
          '<span style="color:var(--color-success)">正常 ' + (devices.normal_point_count || 0) + '</span>' +
          ' / <span style="color:var(--color-danger)">异常 ' + (devices.abnormal_point_count || 0) + '</span>' +
          ' / <span style="color:var(--color-text-muted)">离线 ' + (devices.offline_point_count || 0) + '</span>' +
        '</div><div class="stat-card-sub">已绑定 ' + (devices.bound_point_count || 0) + ' / 未绑定 ' + (devices.unbound_point_count || 0) + '</div></div>' +
        '<div class="stat-card"><div class="stat-card-label">心跳成功率</div><div class="stat-card-value">' + (ingest.heartbeat_success_rate != null ? ingest.heartbeat_success_rate + '%' : '--') + '</div><div class="stat-card-sub">成功 ' + (ingest.heartbeat_success || 0) + ' / 失败 ' + (ingest.heartbeat_failed || 0) + '</div></div>' +
        '<div class="stat-card"><div class="stat-card-label">控制命令</div><div class="stat-card-value">' + (control.control_command_count || 0) + '</div><div class="stat-card-sub">成功 ' + (control.control_success_count || 0) + ' / 失败 ' + (control.control_failed_count || 0) + '</div></div>' +
        '<div class="stat-card"><div class="stat-card-label">注册 / 遥测</div><div class="stat-card-value">' + (ingest.register_count || 0) + ' / ' + (ingest.telemetry_count || 0) + '</div><div class="stat-card-sub">原始事件 ' + (ingest.raw_event_count || 0) + '</div></div>' +
      '</div>';

      container.innerHTML = html;
    } catch (e) {
      container.innerHTML = '<div class="error-state"><p>加载失败</p><button class="btn btn-outline btn-sm" onclick="renderPage()">重试</button></div>';
    }
  }

  async function switchDashboardRange(range) {
    currentDashboardRange = range;
    renderPage();
  }

  // ---------- 用户管理（农户列表） ----------
  async function renderUsers(container) {
    container.innerHTML = '<div class="loading-state"><div class="loading-spinner"></div><p>加载农户列表...</p></div>';
    try {
      var res = await NHApi.getFarmers();
      if (!res.data) {
        container.innerHTML = '<div class="error-state"><p>加载失败</p><button class="btn btn-outline btn-sm" onclick="renderPage()">重试</button></div>';
        return;
      }
      var farmers = res.data;

      var html = '<div class="table-section">' +
        '<div class="table-header">' +
          '<h3>农户列表</h3>' +
          '<div class="table-header-right">' +
            '<input class="table-search" id="user-search" type="text" placeholder="搜索姓名/手机号/微信OpenID...">' +
            '<span class="table-count">共 ' + farmers.length + ' 户</span>' +
          '</div>' +
        '</div>' +
        '<div class="table-wrap"><table>' +
          '<thead><tr><th>ID</th><th>姓名</th><th>手机号</th><th>地址</th><th>微信OpenID</th><th>绑定设备</th><th>状态</th><th>创建时间</th><th>操作</th></tr></thead>' +
          '<tbody id="user-tbody">' +
            farmers.map(function (u) {
              var statusBadge = u.status === 1 ? 'badge-success' : u.status === 2 ? 'badge-warning' : 'badge-muted';
              return '<tr>' +
                '<td>' + u.user_id + '</td>' +
                '<td>' + escHtml(u.display_name) + '</td>' +
                '<td>' + escHtml(u.mobile || '--') + '</td>' +
                '<td>' + escHtml(u.address || '--') + '</td>' +
                '<td><span style="font-size:11px;word-break:break-all">' + escHtml(u.wechat_openid || '--') + '</span></td>' +
                '<td>' + (u.binding_count || 0) + ' 台</td>' +
                '<td><span class="badge ' + statusBadge + '">' + escHtml(u.status_label || '正常') + '</span></td>' +
                '<td>' + formatTime(u.created_at) + '</td>' +
                '<td><button class="btn btn-outline btn-sm" onclick="goFarmerDetail(\'' + u.user_id + '\')">详情</button></td>' +
              '</tr>';
            }).join('') +
          '</tbody>' +
        '</table></div>' +
      '</div>';

      container.innerHTML = html;

      // 搜索：只使用后端 keyword 参数。
      var searchTimer = null;
      $('#user-search').oninput = function () {
        var q = this.value.trim();
        clearTimeout(searchTimer);
        searchTimer = setTimeout(function () {
          if (q) {
            // 优先使用后端搜索
            searchFarmers(q);
          } else {
            // 空关键词：重新加载全部
            renderPage();
          }
        }, 400);
      };
    } catch (e) {
      container.innerHTML = '<div class="error-state"><p>加载失败：' + escHtml(e.message || '网络异常') + '</p><button class="btn btn-outline btn-sm" onclick="renderPage()">重试</button></div>';
    }
  }

  async function searchFarmers(keyword) {
    var tbody = $('#user-tbody');
    if (!tbody) return;
    tbody.innerHTML = '<tr><td colspan="9" class="table-empty">搜索中...</td></tr>';
    try {
      var res = await NHApi.getFarmers(keyword);
      if (res.data && res.data.length > 0) {
        renderFarmerTableBody(res.data);
      } else if (res.data && res.data.length === 0) {
        tbody.innerHTML = '<tr><td colspan="9" class="table-empty">未找到匹配的农户</td></tr>';
      } else {
        tbody.innerHTML = '<tr><td colspan="9" class="table-empty">搜索失败</td></tr>';
      }
    } catch (e) {
      tbody.innerHTML = '<tr><td colspan="9" class="table-empty">搜索失败：' + escHtml(e.message || '网络异常') + '</td></tr>';
    }
  }

  function renderFarmerTableBody(farmers) {
    var tbody = $('#user-tbody');
    if (!tbody) return;
    if (farmers.length === 0) {
      tbody.innerHTML = '<tr><td colspan="9" class="table-empty">未找到匹配的农户</td></tr>';
      return;
    }
    tbody.innerHTML = farmers.map(function (u) {
      var statusBadge = u.status === 1 ? 'badge-success' : u.status === 2 ? 'badge-warning' : 'badge-muted';
      return '<tr>' +
        '<td>' + u.user_id + '</td>' +
        '<td>' + escHtml(u.display_name) + '</td>' +
        '<td>' + escHtml(u.mobile || '--') + '</td>' +
        '<td>' + escHtml(u.address || '--') + '</td>' +
        '<td><span style="font-size:11px;word-break:break-all">' + escHtml(u.wechat_openid || '--') + '</span></td>' +
        '<td>' + (u.binding_count || 0) + ' 台</td>' +
        '<td><span class="badge ' + statusBadge + '">' + escHtml(u.status_label || '正常') + '</span></td>' +
        '<td>' + formatTime(u.created_at) + '</td>' +
        '<td><button class="btn btn-outline btn-sm" onclick="goFarmerDetail(\'' + u.user_id + '\')">详情</button></td>' +
      '</tr>';
    }).join('');
  }

  // ---------- 农户详情页 ----------
  async function renderFarmerDetail(container) {
    var userId = getHashParam('user_id');
    if (!userId) { container.innerHTML = '<div class="error-state"><p>参数错误</p></div>'; return; }

    container.innerHTML = '<div class="loading-state"><div class="loading-spinner"></div><p>加载中...</p></div>';

    try {
      var farmerRes, bindingsRes;
      await Promise.all([
        NHApi.getFarmerDetail(userId),
        NHApi.getFarmerBindings(userId)
      ]).then(function (results) {
        farmerRes = results[0];
        bindingsRes = results[1];
      });

      var farmer = farmerRes.data;
      var bindings = bindingsRes.data || [];

      if (!farmer) {
        container.innerHTML = '<div class="error-state"><p>农户不存在</p></div>';
        return;
      }

      var html = '<div class="page-header">' +
        '<button class="btn btn-outline btn-sm" onclick="navigate(\'users\')">← 返回用户列表</button>' +
        '<h2>农户详情</h2>' +
      '</div>' +
      '<div class="info-card">' +
        '<h3>基本信息</h3>' +
        '<div class="info-grid">' +
          '<div class="info-item"><label>姓名</label><span>' + escHtml(farmer.display_name) + '</span></div>' +
          '<div class="info-item"><label>手机号</label><span>' + escHtml(farmer.mobile || '--') + '</span></div>' +
          '<div class="info-item"><label>地址</label><span>' + escHtml(farmer.address || '--') + '</span></div>' +
          '<div class="info-item"><label>微信OpenID</label><span>' + escHtml(farmer.wechat_openid || '--') + '</span></div>' +
          '<div class="info-item"><label>备注</label><span>' + escHtml(farmer.remark || '--') + '</span></div>' +
          '<div class="info-item"><label>状态</label><span class="badge badge-success">' + escHtml(farmer.status_label || '正常') + '</span></div>' +
          '<div class="info-item"><label>创建时间</label><span>' + formatTime(farmer.created_at) + '</span></div>' +
        '</div>' +
      '</div>';

      // 已绑定设备列表
      html += '<div class="table-section">' +
        '<div class="table-header">' +
          '<h3>已绑定设备 (' + bindings.length + ')</h3>' +
          '<button class="btn btn-primary btn-sm" id="btn-bind-device" onclick="openBindDeviceModal(\'' + userId + '\')">+ 绑定设备</button>' +
        '</div>' +
        '<div class="table-wrap"><table><thead><tr>' +
          '<th>设备名称</th><th>棚号</th><th>云边设备</th><th>频点</th><th>功能</th><th>实时值</th><th>状态</th><th>绑定时间</th>' +
        '</tr></thead><tbody>';

      bindings.forEach(function (b) {
        var col = b.collector || {};
        html += '<tr>' +
          '<td>' + escHtml(col.func_name || b.point_name || '--') + '</td>' +
          '<td>' + escHtml(b.shed_no || '--') + '</td>' +
          '<td>' + escHtml(col.gateway_name || b.gateway_name || '--') + '</td>' +
          '<td>' + (b.freq || '--') + '</td>' +
          '<td>' + escHtml(col.func_name || b.function_name || '--') + '</td>' +
          '<td>' + (col.real_value != null ? col.real_value + ' ' + (col.unit || '') : '--') + '</td>' +
          '<td>' + escHtml(col.status_label || b.status_label || '--') + '</td>' +
          '<td>' + formatTime(b.created_at) + '</td>' +
        '</tr>';
      });

      html += '</tbody></table></div></div>';

      if (bindings.length === 0) {
        html += '<div class="empty-state"><p>暂无绑定设备</p><p style="font-size:12px;color:var(--color-text-muted)">点击"绑定设备"为该农户添加设备</p></div>';
      }

      container.innerHTML = html;
    } catch (e) {
      container.innerHTML = '<div class="error-state"><p>加载失败</p><button class="btn btn-outline btn-sm" onclick="renderPage()">重试</button></div>';
    }
  }

  // ---------- 绑定设备弹窗（农户详情页） ----------
  async function openBindDeviceModal(userId) {
    var overlay = $('#modal-overlay');
    overlay.style.display = 'flex';
    overlay.innerHTML = '<div class="modal" style="max-width:640px">' +
      '<div class="modal-header">' +
        '<h3>绑定设备</h3>' +
        '<button class="modal-close" onclick="closeModal()">×</button>' +
      '</div>' +
      '<div class="modal-body">' +
        '<div class="form-group">' +
          '<label class="form-label">搜索设备</label>' +
          '<input type="text" class="form-input" id="bind-device-search" placeholder="输入设备名称/棚号/云边设备名/功能类型...">' +
        '</div>' +
        '<div id="bind-device-list" style="max-height:360px;overflow-y:auto;margin-top:12px">' +
          '<div class="loading-state"><div class="loading-spinner"></div><p>搜索设备中...</p></div>' +
        '</div>' +
        '<div id="bind-device-selected" style="display:none;margin-top:12px;padding:10px;background:var(--color-bg);border-radius:6px">' +
          '<span style="font-size:13px;color:var(--color-text-muted)">已选择：</span>' +
          '<span id="bind-selected-name" style="font-weight:600"></span>' +
        '</div>' +
      '</div>' +
      '<div class="modal-actions">' +
        '<button class="btn btn-outline" onclick="closeModal()">取消</button>' +
        '<button class="btn btn-primary" id="btn-confirm-bind" disabled onclick="doBindDevice(\'' + userId + '\')">确认绑定</button>' +
      '</div>' +
    '</div>';

    var selectedPointId = null;

    // 初始加载：搜索全部未绑定设备
    loadDevicePoints('');

    // 防抖搜索
    var searchTimer = null;
    $('#bind-device-search').oninput = function () {
      clearTimeout(searchTimer);
      searchTimer = setTimeout(function () {
        loadDevicePoints($('#bind-device-search').value);
      }, 300);
    };

    async function loadDevicePoints(keyword) {
      try {
        var res = await NHApi.searchDevicePoints(keyword, 'unbound');
        var points = res.data || [];
        var listEl = $('#bind-device-list');

        if (points.length === 0) {
          listEl.innerHTML = '<div class="empty-state"><p>没有可绑定的设备</p></div>';
          return;
        }

        var html = '';
        points.forEach(function (p) {
          var isSelected = selectedPointId === p.point_id;
          html += '<div class="bind-point-item' + (isSelected ? ' selected' : '') + '" data-point-id="' + p.point_id + '" data-freq="' + (p.frequency || '') + '" data-gateway-name="' + escHtml(p.gateway_name || '') + '" style="padding:10px 12px;border:1px solid ' + (isSelected ? 'var(--color-primary)' : 'var(--color-border)') + ';border-radius:6px;margin-bottom:8px;cursor:pointer;transition:all 0.2s">' +
            '<div style="display:flex;justify-content:space-between;align-items:center">' +
              '<div>' +
                '<span style="font-weight:600">' + escHtml(p.function_name) + '</span>' +
                '<span style="color:var(--color-text-muted);margin-left:8px;font-size:13px">频点' + p.frequency + ' · DevID:' + escHtml(p.device_external_id) + '</span>' +
              '</div>' +
              '<span style="font-size:12px;color:var(--color-text-muted)">' + escHtml(p.gateway_name) + '</span>' +
            '</div>' +
            '<div style="font-size:12px;color:var(--color-text-muted);margin-top:4px">' +
              escHtml(p.shed_no || '--') + ' · ' + escHtml(p.point_name || '--') +
              (p.is_bound ? '<span class="badge" style="margin-left:8px">已绑定: ' + escHtml(p.bind_user_name) + '</span>' : '<span class="badge badge-success" style="margin-left:8px">未绑定</span>') +
            '</div>' +
          '</div>';
        });

        listEl.innerHTML = html;

        // 点击选中
        listEl.querySelectorAll('.bind-point-item').forEach(function (el) {
          el.onclick = function () {
            selectedPointId = this.dataset.pointId;
            // 高亮选中项
            listEl.querySelectorAll('.bind-point-item').forEach(function (e) {
              e.classList.remove('selected');
              e.style.borderColor = 'var(--color-border)';
            });
            this.classList.add('selected');
            this.style.borderColor = 'var(--color-primary)';

            // 显示已选择
            var selDiv = $('#bind-device-selected');
            selDiv.style.display = 'block';
            var selName = this.querySelector('span').textContent;
            $('#bind-selected-name').textContent = selName + ' · 频点' + (this.dataset.freq || '') + ' · ' + (this.dataset.gatewayName || '');

            $('#btn-confirm-bind').disabled = false;
          };
        });
      } catch (e) {
        $('#bind-device-list').innerHTML = '<div class="error-state"><p>加载失败</p></div>';
      }
    }
  }

  // ---------- 编辑农户 Modal ----------
  function openEditFarmerModal(userId) {
    var overlay = $('#modal-overlay');
    overlay.style.display = 'flex';
    overlay.innerHTML =
      '<div class="modal">' +
        '<h3>编辑农户信息</h3>' +
        '<div class="form-group" style="display:none"><label class="form-label">用户 ID</label><input class="form-input" id="edit-farmer-id" type="number" readonly></div>' +
        '<div class="form-group"><label class="form-label">姓名</label><input class="form-input" id="edit-farmer-name" placeholder="请输入姓名"></div>' +
        '<div class="form-group"><label class="form-label">手机号</label><input class="form-input" id="edit-farmer-mobile" placeholder="请输入手机号"></div>' +
        '<div class="form-group"><label class="form-label">地址</label><input class="form-input" id="edit-farmer-address" placeholder="请输入地址"></div>' +
        '<div class="form-group"><label class="form-label">备注</label><input class="form-input" id="edit-farmer-remark" placeholder="请输入备注"></div>' +
        '<div class="form-group"><label class="form-label">状态</label><select class="form-select" id="edit-farmer-status"><option value="1">正常</option><option value="2">已禁用</option></select></div>' +
        '<div class="modal-actions">' +
          '<button class="btn btn-outline btn-sm" onclick="closeModal()">取消</button>' +
          '<button class="btn btn-primary btn-sm" onclick="doUpdateFarmer()">保存</button>' +
        '</div>' +
      '</div>';

    // 加载农户详情
    (async function () {
      try {
        var res = await NHApi.getFarmerDetail(userId);
        if (res.data) {
          $('#edit-farmer-id').value = res.data.user_id;
          $('#edit-farmer-name').value = res.data.display_name || '';
          $('#edit-farmer-mobile').value = res.data.mobile || '';
          $('#edit-farmer-address').value = res.data.address || '';
          $('#edit-farmer-remark').value = res.data.remark || '';
          $('#edit-farmer-status').value = res.data.status || 1;
        } else {
          showToast('加载农户详情失败', 'error');
        }
      } catch (e) {
        showToast('加载农户详情失败', 'error');
      }
    })();
  }

  async function doUpdateFarmer() {
    var userId = parseInt($('#edit-farmer-id').value);
    if (!userId) { showToast('用户 ID 无效', 'error'); return; }

    var data = {
      display_name: $('#edit-farmer-name').value.trim(),
      mobile: $('#edit-farmer-mobile').value.trim(),
      address: $('#edit-farmer-address').value.trim(),
      remark: $('#edit-farmer-remark').value.trim(),
      status: parseInt($('#edit-farmer-status').value),
    };

    if (!data.display_name) { showToast('请输入姓名', 'error'); return; }

    try {
      var res = await NHApi.updateFarmer(userId, data);
      if (res.data) {
        showToast('保存成功', 'success');
        closeModal();
        renderPage();
      } else {
        showToast(res.message || '保存失败', 'error');
      }
    } catch (e) {
      showToast('操作异常', 'error');
    }
  }

  // ---------- 设备管理 ----------
  async function renderGateways(container) {
    container.innerHTML = '<div class="loading-state"><div class="loading-spinner"></div><p>加载设备信息...</p></div>';
    try {
      var gwRes, treeRes;
      await Promise.all([
        NHApi.getGateways(),
        NHApi.getDeviceTree()
      ]).then(function (results) {
        gwRes = results[0];
        treeRes = results[1];
      });

      var gateways = (gwRes.data && gwRes.data.items) ? gwRes.data.items : [];
      var tree = treeRes.data || [];

      var html = '<div class="table-section">' +
        '<div class="table-header"><h3>云边设备 (' + gateways.length + ')</h3></div>' +
        '<div class="table-wrap"><table><thead><tr>' +
          '<th>ID</th><th>名称</th><th>IP</th><th>端口</th><th>在线状态</th>' +
        '</tr></thead><tbody>';

      gateways.forEach(function (g) {
        html += '<tr>' +
          '<td>' + g.id + '</td>' +
          '<td>' + escHtml(g.dev_name || g.gateway_name) + '</td>' +
          '<td>' + escHtml(g.ip) + '</td>' +
          '<td>' + g.port + '</td>' +
          '<td><span class="badge ' + (g.online ? 'badge-success' : '') + '">' + (g.online ? '在线' : '离线') + '</span></td>' +
        '</tr>';
      });

      html += '</tbody></table></div></div>';

      // 功能点列表
      html += '<div class="table-section" style="margin-top:24px">' +
        '<div class="table-header">' +
          '<h3>功能点列表</h3>' +
          '<input class="table-search" id="device-point-search" type="text" placeholder="搜索功能点..." style="width:240px">' +
        '</div>' +
        '<div class="table-wrap"><table><thead><tr>' +
          '<th>Point ID</th><th>设备名称</th><th>云边设备</th><th>棚号</th><th>频点</th><th>DevID</th><th>功能</th><th>备注</th>' +
        '</tr></thead><tbody id="device-points-tbody"></tbody></table></div></div>';

      container.innerHTML = html;

      // 从 tree 展开所有 point
      var allPoints = [];
      tree.forEach(function (gw) {
        (gw.collectors || []).forEach(function (cl) {
          (cl.points || []).forEach(function (pt) {
            allPoints.push({
              point_id: pt.point_id,
              display_name: pt.display_name || pt.point_name || '--',
              gateway_name: gw.gateway_name,
              shed_no: pt.shed_no || cl.shed_no || '--',
              frequency: cl.frequency,
              device_external_id: cl.device_external_id,
              function_type: pt.function_type,
              function_name: pt.function_name || '--',
              remark: pt.remark || '--'
            });
          });
        });
      });

      function renderPointsTable(filter) {
        var tbody = $('#device-points-tbody');
        if (!tbody) return;

        var filtered = allPoints;
        if (filter) {
          var kw = filter.toLowerCase();
          filtered = allPoints.filter(function (p) {
            return (p.display_name + ' ' + p.gateway_name + ' ' + p.shed_no + ' ' + p.frequency + ' ' + p.device_external_id + ' ' + p.function_name).toLowerCase().indexOf(kw) !== -1;
          });
        }

        tbody.innerHTML = filtered.map(function (p) {
          return '<tr>' +
            '<td style="font-family:monospace;font-size:12px">' + escHtml(String(p.point_id)) + '</td>' +
            '<td>' + escHtml(p.display_name) + '</td>' +
            '<td>' + escHtml(p.gateway_name) + '</td>' +
            '<td>' + escHtml(p.shed_no) + '</td>' +
            '<td>' + p.frequency + '</td>' +
            '<td>' + escHtml(p.device_external_id) + '</td>' +
            '<td>' + escHtml(p.function_name) + '</td>' +
            '<td>' + escHtml(p.remark) + '</td>' +
          '</tr>';
        }).join('');
      }

      renderPointsTable('');

      $('#device-point-search').oninput = function () {
        renderPointsTable(this.value);
      };
    } catch (e) {
      container.innerHTML = '<div class="error-state"><p>加载失败</p><button class="btn btn-outline btn-sm" onclick="renderPage()">重试</button></div>';
    }
  }

  // ---------- 绑定管理 ----------

  async function renderBindings(container) {
    container.innerHTML = '<div class="loading-state"><div class="loading-spinner"></div><p>加载绑定关系...</p></div>';
    try {
      var res = await NHApi.getBindings();
      if (!res.data) { container.innerHTML = '<div class="error-state"><p>加载失败</p><button class="btn btn-outline btn-sm" onclick="renderPage()">重试</button></div>'; return; }
      var bindings = res.data;

      // 筛选器
      var html = '<div class="table-section">' +
        '<div class="table-header">' +
          '<h3>绑定管理</h3>' +
          '<div class="table-header-right">' +
            '<button class="btn btn-primary btn-sm" onclick="openBindingAddModal()">+ 新增绑定</button>' +
            '<span class="table-count">共 ' + bindings.length + ' 条</span>' +
          '</div>' +
        '</div>' +
        '<div class="filter-bar" style="display:flex;gap:12px;margin-bottom:16px;flex-wrap:wrap">' +
          '<input type="text" class="table-search" id="bind-filter-farmer" placeholder="按农户筛选..." style="width:200px">' +
          '<input type="text" class="table-search" id="bind-filter-device" placeholder="按设备筛选..." style="width:200px">' +
          '<select class="form-input" id="bind-filter-status" style="width:140px">' +
            '<option value="">全部状态</option>' +
            '<option value="active">有效</option>' +
            '<option value="unbound">已解绑</option>' +
          '</select>' +
        '</div>' +
        '<div class="table-wrap"><table><thead><tr>' +
          '<th>绑定ID</th><th>农户</th><th>设备点</th><th>功能</th><th>状态</th><th>绑定时间</th><th>操作</th>' +
        '</tr></thead><tbody id="bindings-tbody">' +
        '</tbody></table></div></div>';

      container.innerHTML = html;

      // 渲染表格 + 筛选逻辑
      function renderBindingsTable(filterFarmer, filterDevice, filterStatus) {
        var tbody = $('#bindings-tbody');
        if (!tbody) return;

        var filtered = bindings.filter(function (b) {
          if (filterFarmer && (b.bind_user_name || b.bind_usr_name || '').toLowerCase().indexOf(filterFarmer.toLowerCase()) === -1) return false;
          var col = b.collector || {};
          var deviceStr = (col.func_name || ('功能' + b.func)) + ' ' + (b.shed_no || '') + ' ' + b.freq + ' ' + b.devid;
          if (filterDevice && deviceStr.toLowerCase().indexOf(filterDevice.toLowerCase()) === -1) return false;
          if (filterStatus === 'active' && b.bind_status !== undefined && b.bind_status !== 1) return false;
          if (filterStatus === 'unbound' && b.bind_status !== undefined && b.bind_status !== 0 && b.bind_status !== 2) return false;
          return true;
        });

        tbody.innerHTML = filtered.map(function (b) {
          var col = b.collector || {};
          var funcName = col.func_name || ('功能' + b.func);
          var statusLabel = b.bind_status === 1 ? '<span class="badge badge-success">有效</span>' : '<span class="badge">已解绑</span>';
          return '<tr>' +
            '<td>' + b.bind_id + '</td>' +
            '<td>' + escHtml(b.bind_user_name || b.bind_usr_name || '--') + '</td>' +
            '<td>' + escHtml(funcName) + ' · ' + escHtml(b.shed_no || '--') + ' · 频点' + b.freq + '</td>' +
            '<td>' + escHtml(funcName) + '</td>' +
            '<td>' + statusLabel + '</td>' +
            '<td>' + formatTime(b.created_at) + '</td>' +
            '<td><button class="btn btn-danger btn-sm" onclick="doRemoveBinding(' + b.bind_id + ')">解绑</button></td>' +
          '</tr>';
        }).join('');
      }

      renderBindingsTable('', '', '');

      // 筛选事件
      $('#bind-filter-farmer').oninput = function () {
        renderBindingsTable(this.value, $('#bind-filter-device').value, $('#bind-filter-status').value);
      };
      $('#bind-filter-device').oninput = function () {
        renderBindingsTable($('#bind-filter-farmer').value, this.value, $('#bind-filter-status').value);
      };
      $('#bind-filter-status').onchange = function () {
        renderBindingsTable($('#bind-filter-farmer').value, $('#bind-filter-device').value, this.value);
      };
    } catch (e) {
      container.innerHTML = '<div class="error-state"><p>加载失败</p><button class="btn btn-outline btn-sm" onclick="renderPage()">重试</button></div>';
    }
  }

  function closeModal() {
    $('#modal-overlay').style.display = 'none';
  }

  async function doBindDevice(userId) {
    // 从 DOM 获取选中的 point_id
    var selectedEl = document.querySelector('.bind-point-item.selected');
    if (!selectedEl) { showToast('请选择一个设备', 'error'); return; }
    var pointId = selectedEl.dataset.pointId;

    try {
      var res = await NHApi.addBinding({ user_id: userId, point_id: pointId });
      if (res.code === 409) {
        // 冲突提示
        var existingName = (res.data && res.data.existing_user_name) || '其他农户';
        showToast('该设备已绑定给「' + existingName + '」', 'error');
        return;
      }
      if (res.data) {
        showToast('绑定成功', 'success');
        closeModal();
        // 刷新详情页
        renderPage();
      } else {
        showToast(res.message || '绑定失败', 'error');
      }
    } catch (e) {
      showToast('操作异常', 'error');
    }
  }

  // ---------- 绑定管理页 - 新增绑定弹窗 ----------
  async function openBindingAddModal() {
    var overlay = $('#modal-overlay');
    overlay.style.display = 'flex';

    overlay.innerHTML = '<div class="modal" style="max-width:640px">' +
      '<div class="modal-header">' +
        '<h3>新增绑定</h3>' +
        '<button class="modal-close" onclick="closeModal()">×</button>' +
      '</div>' +
      '<div class="modal-body">' +
        '<div class="form-group">' +
          '<label class="form-label">选择农户</label>' +
          '<select class="form-input" id="bind-add-farmer"><option value="">-- 请选择农户 --</option></select>' +
        '</div>' +
        '<div class="form-group">' +
          '<label class="form-label">搜索设备</label>' +
          '<input type="text" class="form-input" id="bind-add-device-search" placeholder="输入关键词搜索设备点...">' +
        '</div>' +
        '<div id="bind-add-device-list" style="max-height:300px;overflow-y:auto;margin-top:12px">' +
          '<p style="color:var(--color-text-muted);font-size:13px;text-align:center;padding:20px">输入关键词搜索设备</p>' +
        '</div>' +
      '</div>' +
      '<div class="modal-actions">' +
        '<button class="btn btn-outline" onclick="closeModal()">取消</button>' +
        '<button class="btn btn-primary" id="btn-confirm-binding-add" disabled onclick="doBindFromAddModal()">确认绑定</button>' +
      '</div>' +
    '</div>';

    // 加载农户列表
    try {
      var farmersRes = await NHApi.getFarmers();
      var farmers = farmersRes.data || [];
      var select = $('#bind-add-farmer');
      farmers.forEach(function (f) {
        var opt = document.createElement('option');
        opt.value = f.user_id;
        opt.textContent = f.display_name + ' (' + (f.mobile || '--') + ')';
        select.appendChild(opt);
      });
    } catch (e) {}

    var addSelectedPointId = null;

    // 搜索防抖
    var searchTimer = null;
    $('#bind-add-device-search').oninput = function () {
      clearTimeout(searchTimer);
      searchTimer = setTimeout(function () {
        searchPointsForAdd($('#bind-add-device-search').value);
      }, 300);
    };

    async function searchPointsForAdd(keyword) {
      try {
        var res = await NHApi.searchDevicePoints(keyword, 'unbound');
        var points = res.data || [];
        var listEl = $('#bind-add-device-list');

        if (points.length === 0) {
          listEl.innerHTML = '<div class="empty-state"><p>没有匹配的设备</p></div>';
          return;
        }

        listEl.innerHTML = points.map(function (p) {
          var isSelected = addSelectedPointId === p.point_id;
          return '<div class="bind-point-item' + (isSelected ? ' selected' : '') + '" data-point-id="' + p.point_id + '" style="padding:10px 12px;border:1px solid ' + (isSelected ? 'var(--color-primary)' : 'var(--color-border)') + ';border-radius:6px;margin-bottom:8px;cursor:pointer">' +
            '<div style="font-weight:600">' + escHtml(p.function_name) + ' · 频点' + p.frequency + ' · DevID:' + escHtml(p.device_external_id) + '</div>' +
            '<div style="font-size:12px;color:var(--color-text-muted)">' + escHtml(p.shed_no || '--') + ' · ' + escHtml(p.gateway_name) + ' · 未绑定</div>' +
          '</div>';
        }).join('');

        listEl.querySelectorAll('.bind-point-item').forEach(function (el) {
          el.onclick = function () {
            addSelectedPointId = this.dataset.pointId;
            listEl.querySelectorAll('.bind-point-item').forEach(function (e) {
              e.classList.remove('selected');
              e.style.borderColor = 'var(--color-border)';
            });
            this.classList.add('selected');
            this.style.borderColor = 'var(--color-primary)';
            $('#btn-confirm-binding-add').disabled = false;
          };
        });
      } catch (e) {
        $('#bind-add-device-list').innerHTML = '<div class="error-state"><p>搜索失败</p></div>';
      }
    }
  }

  async function doBindFromAddModal() {
    var userId = $('#bind-add-farmer').value;
    var selectedEl = document.querySelector('#bind-add-device-list .bind-point-item.selected');
    if (!userId) { showToast('请选择农户', 'error'); return; }
    if (!selectedEl) { showToast('请选择一个设备', 'error'); return; }
    var pointId = selectedEl.dataset.pointId;

    try {
      var res = await NHApi.addBinding({ user_id: userId, point_id: pointId });
      if (res.code === 409) {
        showToast('该设备已绑定给「' + ((res.data && res.data.existing_user_name) || '其他农户') + '」', 'error');
        return;
      }
      if (res.data) {
        showToast('绑定成功', 'success');
        closeModal();
        renderPage();
      } else {
        showToast(res.message || '绑定失败', 'error');
      }
    } catch (e) {
      showToast('操作异常', 'error');
    }
  }

  async function doRemoveBinding(id) {
    if (!confirm('确认解除该绑定关系？')) return;
    try {
      var res = await NHApi.removeBinding(id);
      if (res.data && res.data.deleted) {
        showToast('解绑成功', 'success');
        renderPage();
      } else {
        showToast(res.message || '解绑失败', 'error');
      }
    } catch (e) {
      showToast('操作异常', 'error');
    }
  }

  // ---------- 控制日志 ----------
  async function renderLogs(container) {
    container.innerHTML = '<div class="loading-state"><div class="loading-spinner"></div><p>加载控制日志...</p></div>';
    try {
      var res = await NHApi.getControlLogs();
      if (!res.data) { container.innerHTML = '<div class="error-state"><p>加载失败</p><button class="btn btn-outline btn-sm" onclick="renderPage()">重试</button></div>'; return; }
      var logs = res.data;

      var html = '<div class="table-section">' +
        '<div class="table-header">' +
          '<h3>控制日志</h3>' +
          '<div class="table-header-right">' +
            '<input class="table-search" id="log-search" type="text" placeholder="搜索操作人/设备...">' +
            '<span class="table-count">共 ' + logs.length + ' 条</span>' +
          '</div>' +
        '</div>' +
        '<div class="table-wrap"><table>' +
          '<thead><tr><th>ID</th><th>操作人</th><th>设备</th><th>功能</th><th>操作类型</th><th>命令状态</th><th>结果</th><th>操作时间</th></tr></thead>' +
          '<tbody id="log-tbody">' +
            logs.map(function (l) {
              var statusBadge = l.cmd_status === 'succeeded' ? 'badge-success' : l.cmd_status === 'failed' ? 'badge-danger' : 'badge-warning';
              return '<tr>' +
                '<td>' + l.op_id + '</td>' +
                '<td>' + escHtml(l.bind_usr_name) + '</td>' +
                '<td>' + escHtml(l.gateway_name) + ' / ' + l.freq + ' / ' + l.devid + '</td>' +
                '<td>' + escHtml(l.func_name) + '</td>' +
                '<td>' + escHtml(l.op_type) + '</td>' +
                '<td><span class="badge ' + statusBadge + '">' + escHtml(l.cmd_status_label) + '</span></td>' +
                '<td>' + escHtml(l.result) + '</td>' +
                '<td>' + formatFullTime(l.op_time) + '</td>' +
              '</tr>';
            }).join('') +
          '</tbody>' +
        '</table></div>' +
      '</div>';

      container.innerHTML = html;

      $('#log-search').oninput = function () {
        var q = this.value.toLowerCase();
        $$('#log-tbody tr').forEach(function (tr) {
          tr.style.display = tr.textContent.toLowerCase().includes(q) ? '' : 'none';
        });
      };
    } catch (e) {
      container.innerHTML = '<div class="error-state"><p>加载失败</p><button class="btn btn-outline btn-sm" onclick="renderPage()">重试</button></div>';
    }
  }

  // ---------- Payload 排查 ----------
  var payloadState = { page: 1, size: 20, event_type: null, parse_status: null, start: null, end: null, keyword: null };

  async function renderPayloadEvents(container) {
    container.innerHTML = '<div class="loading-state"><div class="loading-spinner"></div><p>加载 Payload 事件...</p></div>';

    var params = { page: payloadState.page, size: payloadState.size };
    if (payloadState.event_type != null) params.event_type = payloadState.event_type;
    if (payloadState.parse_status != null) params.parse_status = payloadState.parse_status;
    if (payloadState.start) params.start = payloadState.start;
    if (payloadState.end) params.end = payloadState.end;
    if (payloadState.keyword) params.keyword = payloadState.keyword;

    try {
      var res = await NHApi.getPayloadEvents(params);
      var data = res.data || {};
      var items = data.items || [];
      var total = data.total || 0;
      var page = data.page || 1;
      var size = data.size || 20;

      var totalPages = Math.ceil(total / size);

      var eventTypeOptions = [
        { val: '', label: '全部类型' },
        { val: '1', label: '注册' },
        { val: '2', label: '心跳' },
        { val: '3', label: '工作数据' },
        { val: '4', label: '控制结果' },
        { val: '5', label: 'JSON快照' }
      ];
      var parseStatusOptions = [
        { val: '', label: '全部状态' },
        { val: '1', label: '待解析' },
        { val: '2', label: '已解析' },
        { val: '3', label: '解析失败' },
        { val: '4', label: '部分解析' },
        { val: '5', label: '已忽略' }
      ];

      var selectedET = payloadState.event_type != null ? String(payloadState.event_type) : '';
      var selectedPS = payloadState.parse_status != null ? String(payloadState.parse_status) : '';

      var html = '<div class="table-section">' +
        '<div class="table-header">' +
          '<h3>Payload 事件</h3>' +
          '<span class="table-count">共 ' + total + ' 条</span>' +
        '</div>' +
        '<div class="filter-bar" style="display:flex;gap:8px;flex-wrap:wrap;align-items:center">' +
          '<select class="form-input" id="pe-filter-type" style="width:120px;height:32px;font-size:13px">' +
            eventTypeOptions.map(function (o) { return '<option value="' + o.val + '"' + (selectedET === o.val ? ' selected' : '') + '>' + o.label + '</option>'; }).join('') +
          '</select>' +
          '<select class="form-input" id="pe-filter-status" style="width:120px;height:32px;font-size:13px">' +
            parseStatusOptions.map(function (o) { return '<option value="' + o.val + '"' + (selectedPS === o.val ? ' selected' : '') + '>' + o.label + '</option>'; }).join('') +
          '</select>' +
          '<input type="datetime-local" class="form-input" id="pe-filter-start" style="width:180px;height:32px;font-size:13px" value="' + (payloadState.start || '') + '" placeholder="开始时间">' +
          '<input type="datetime-local" class="form-input" id="pe-filter-end" style="width:180px;height:32px;font-size:13px" value="' + (payloadState.end || '') + '" placeholder="结束时间">' +
          '<input type="text" class="table-search" id="pe-filter-keyword" style="width:180px;height:32px" value="' + escHtml(payloadState.keyword || '') + '" placeholder="搜索 gateway/message_id...">' +
          '<button class="btn btn-outline btn-sm" onclick="payloadResetFilters()">重置</button>' +
        '</div>' +
        '<div class="table-wrap"><table>' +
          '<thead><tr>' +
            '<th>ID</th><th>时间</th><th>事件类型</th><th>Gateway</th><th>来源</th><th>解析状态</th><th>Message ID</th><th>Events</th><th>来源IP</th><th>Payload</th><th>操作</th>' +
          '</tr></thead>' +
          '<tbody id="payload-tbody">' +
            (items.length === 0 ? '<tr><td colspan="11" class="table-empty">暂无数据</td></tr>' :
              items.map(function (item) {
                var statusCSS = (item.parse_status === 1 ? 'badge-warning' : item.parse_status === 2 ? 'badge-success' : item.parse_status === 3 ? 'badge-danger' : item.parse_status === 4 ? 'badge-info' : 'badge-muted');
                var eventCSS = (item.event_type === 1 ? 'badge-info' : item.event_type === 2 ? 'badge-muted' : item.event_type === 3 ? 'badge-success' : 'badge-warning');
                var isMock = !!item.is_mock;
                var sourceName = item.source_type_name || '--';
                return '<tr>' +
                  '<td style="font-family:monospace;font-size:12px">' + escHtml(String(item.raw_event_id)) + '</td>' +
                  '<td style="white-space:nowrap;font-size:12px">' + escHtml(item.received_at_ms) + '</td>' +
                  '<td><span class="badge ' + eventCSS + '">' + escHtml(item.event_type_name) + '</span></td>' +
                  '<td>' + escHtml(item.gateway_name || '--') + '</td>' +
                  '<td><span class="badge ' + (isMock ? 'badge-warning' : 'badge-muted') + '">' + (isMock ? 'MOCK' : escHtml(sourceName)) + '</span></td>' +
                  '<td><span class="badge ' + statusCSS + '">' + escHtml(item.parse_status_name) + '</span>' +
                    (item.parse_error ? ' <span style="font-size:10px;color:var(--color-text-muted)" title="' + escHtml(item.parse_error) + '">⚠</span>' : '') +
                  '</td>' +
                  '<td style="font-family:monospace;font-size:11px;max-width:160px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">' + (isMock ? '<span class="badge badge-warning" style="margin-right:4px">MOCK</span>' : '') + escHtml(item.message_id || '--') + '</td>' +
                  '<td><span style="font-size:12px">' + item.parsed_count + '/' + item.event_count + '</span></td>' +
                  '<td style="font-size:12px">' + escHtml(item.remote_ip || '--') + ':' + item.remote_port + '</td>' +
                  '<td style="font-size:11px;color:var(--color-text-muted)">' + (item.payload_len || 0) + 'B</td>' +
                  '<td><div style="display:flex;gap:4px">' +
                    '<button class="btn btn-outline btn-sm" onclick="payloadCopySnapshot(\'' + item.raw_event_id + '\')" title="拷贝 Payload">📋</button>' +
                    '<button class="btn btn-outline btn-sm" onclick="payloadViewDetail(\'' + item.raw_event_id + '\')" title="查看详情">🔍</button>' +
                  '</div></td>' +
                '</tr>';
              }).join('')) +
          '</tbody>' +
        '</table></div>' +
        '<div style="display:flex;align-items:center;justify-content:center;gap:8px;padding:12px;border-top:1px solid var(--color-border)">' +
          '<button class="btn btn-outline btn-sm" ' + (page <= 1 ? 'disabled' : '') + ' onclick="payloadGoPage(' + (page - 1) + ')">上一页</button>' +
          '<span style="font-size:13px;color:var(--color-text-muted)">第 ' + page + '/' + totalPages + ' 页</span>' +
          '<button class="btn btn-outline btn-sm" ' + (page >= totalPages ? 'disabled' : '') + ' onclick="payloadGoPage(' + (page + 1) + ')">下一页</button>' +
          '<span style="font-size:12px;color:var(--color-text-muted);margin-left:8px">每页 ' + size + ' 条</span>' +
        '</div>' +
      '</div>';

      container.innerHTML = html;

      // 绑定筛选事件
      $('#pe-filter-type').onchange = function () { var v = this.value; payloadState.event_type = v === '' ? null : parseInt(v); payloadState.page = 1; renderPage(); };
      $('#pe-filter-status').onchange = function () { var v = this.value; payloadState.parse_status = v === '' ? null : parseInt(v); payloadState.page = 1; renderPage(); };
      $('#pe-filter-start').onchange = function () { payloadState.start = this.value || null; payloadState.page = 1; renderPage(); };
      $('#pe-filter-end').onchange = function () { payloadState.end = this.value || null; payloadState.page = 1; renderPage(); };

      var searchTimer = null;
      $('#pe-filter-keyword').oninput = function () {
        clearTimeout(searchTimer);
        var val = this.value;
        searchTimer = setTimeout(function () {
          payloadState.keyword = val || null;
          payloadState.page = 1;
          renderPage();
        }, 400);
      };

    } catch (e) {
      container.innerHTML = '<div class="error-state"><p>加载失败</p><button class="btn btn-outline btn-sm" onclick="renderPage()">重试</button></div>';
    }
  }

  function payloadResetFilters() {
    payloadState = { page: 1, size: 20, event_type: null, parse_status: null, start: null, end: null, keyword: null };
    renderPage();
  }

  function payloadGoPage(p) {
    payloadState.page = p;
    renderPage();
  }

  async function payloadCopySnapshot(rawEventId) {
    try {
      var res = await NHApi.getPayloadEventDetail(rawEventId);
      if (!res.data) { showToast('获取详情失败', 'error'); return; }
      var snapshot = res.data.payload_snapshot || '';
      if (!snapshot) { showToast('该事件无 payload 数据', 'info'); return; }

      var copyText = snapshot;
      try {
        var parsed = JSON.parse(snapshot);
        copyText = JSON.stringify(parsed, null, 2);
      } catch (e) {}

      if (navigator.clipboard && navigator.clipboard.writeText) {
        await navigator.clipboard.writeText(copyText);
        showToast('已复制 Payload (' + copyText.length + ' 字符)', 'success');
      } else {
        var ta = document.createElement('textarea');
        ta.value = copyText;
        ta.style.position = 'fixed'; ta.style.left = '-9999px';
        document.body.appendChild(ta);
        ta.select();
        document.execCommand('copy');
        document.body.removeChild(ta);
        showToast('已复制 Payload (' + copyText.length + ' 字符)', 'success');
      }
    } catch (e) {
      showToast('拷贝失败: ' + (e.message || '未知错误'), 'error');
    }
  }

  async function payloadViewDetail(rawEventId) {
    var overlay = $('#modal-overlay');
    overlay.style.display = 'flex';
    overlay.innerHTML = '<div class="modal" style="max-width:720px">' +
      '<div class="modal-header">' +
        '<h3>Payload 详情</h3>' +
        '<button class="modal-close" onclick="closeModal()">×</button>' +
      '</div>' +
      '<div class="modal-body" style="max-height:70vh;overflow-y:auto">' +
        '<div class="loading-state"><div class="loading-spinner"></div><p>加载中...</p></div>' +
      '</div>' +
      '<div class="modal-actions">' +
        '<button class="btn btn-outline" onclick="closeModal()">关闭</button>' +
        '<button class="btn btn-primary btn-sm" id="btn-payload-copy-detail" style="display:none" onclick="payloadCopySnapshot(\'' + rawEventId + '\')">📋 拷贝 Payload</button>' +
      '</div>' +
    '</div>';

    try {
      var res = await NHApi.getPayloadEventDetail(rawEventId);
      if (!res.data) {
        overlay.querySelector('.modal-body').innerHTML = '<div class="error-state"><p>获取详情失败</p></div>';
        return;
      }
      var d = res.data;
      var isMock = !!d.is_mock;

      var bodyHtml = '<div style="display:grid;grid-template-columns:1fr 1fr;gap:10px;margin-bottom:16px">' +
        '<div class="info-item"><label>事件 ID</label><span style="font-family:monospace">' + escHtml(String(d.raw_event_id)) + '</span></div>' +
        '<div class="info-item"><label>接收时间</label><span>' + escHtml(d.received_at_ms) + '</span></div>' +
        '<div class="info-item"><label>事件类型</label><span class="badge">' + escHtml(d.event_type_name) + '</span></div>' +
        '<div class="info-item"><label>解析状态</label><span>' + escHtml(d.parse_status_name) + (d.parse_error ? ' <span style="color:var(--color-danger);font-size:11px">' + escHtml(d.parse_error) + '</span>' : '') + '</span></div>' +
        '<div class="info-item"><label>Gateway</label><span>' + escHtml(d.gateway_name || '--') + '</span></div>' +
        '<div class="info-item"><label>来源</label><span><span class="badge ' + (isMock ? 'badge-warning' : 'badge-muted') + '">' + (isMock ? 'MOCK' : escHtml(d.source_type_name || '--')) + '</span></span></div>' +
        '<div class="info-item"><label>Message ID</label><span style="font-family:monospace;font-size:11px">' + (isMock ? '<span class="badge badge-warning" style="margin-right:4px">MOCK</span>' : '') + escHtml(d.message_id || '--') + '</span></div>' +
        '<div class="info-item"><label>Events 数量</label><span>' + d.parsed_count + '/' + d.event_count + '</span></div>' +
        '<div class="info-item"><label>来源 IP</label><span>' + escHtml(d.remote_ip || '--') + ':' + d.remote_port + '</span></div>' +
      '</div>';

      var snapshot = d.payload_snapshot || '';
      var formattedPayload = snapshot;
      var isJSON = false;
      if (snapshot) {
        try {
          var parsed = JSON.parse(snapshot);
          formattedPayload = JSON.stringify(parsed, null, 2);
          isJSON = true;
        } catch (e) {}
      }

      bodyHtml += '<div style="margin-top:12px">' +
        '<label style="font-size:11px;color:var(--color-text-muted);text-transform:uppercase;letter-spacing:0.3px;margin-bottom:6px;display:block">Payload 快照' + (isJSON ? ' (JSON)' : '') + ' — ' + snapshot.length + ' 字符</label>' +
        '<pre style="background:var(--color-bg);border:1px solid var(--color-border);border-radius:var(--radius-sm);padding:12px;font-size:12px;font-family:monospace;max-height:400px;overflow:auto;white-space:pre-wrap;word-break:break-all">' + escHtml(formattedPayload || '(空)') + '</pre>' +
      '</div>';

      overlay.querySelector('.modal-body').innerHTML = bodyHtml;
      var copyBtn = overlay.querySelector('#btn-payload-copy-detail');
      if (copyBtn) copyBtn.style.display = '';
    } catch (e) {
      overlay.querySelector('.modal-body').innerHTML = '<div class="error-state"><p>加载失败: ' + escHtml(e.message || '未知错误') + '</p></div>';
    }
  }

  // ============================================================
  // 初始化
  // ============================================================

  // 更新时间
  function updateClock() {
    var now = new Date();
    var pad = function (n) { return n.toString().padStart(2, '0'); };
    $('#topbar-time').textContent = now.getFullYear() + '-' + pad(now.getMonth() + 1) + '-' + pad(now.getDate()) + ' ' + pad(now.getHours()) + ':' + pad(now.getMinutes()) + ':' + pad(now.getSeconds());
  }
  setInterval(updateClock, 1000);
  updateClock();

  // 版本号（控制台 + 页脚 + 右下角浮标）
  console.info('%c涅凰智农 IoT v' + (window.APP_VERSION_STRING || '?'), 'color:#2ecc71;font-weight:bold');
  // 侧边栏页脚
  var versionEl = document.getElementById('app-version');
  if (versionEl) versionEl.textContent = 'v' + (window.APP_VERSION || '?');
  // 登录页也能看到的右下角浮标
  var badge = document.getElementById('version-badge');
  if (badge && window.APP_VERSION) badge.textContent = 'v' + window.APP_VERSION;

  window.addEventListener('hashchange', renderPage);
  window.addEventListener('load', function () {
    var token = localStorage.getItem('nh_admin_token');
    var userJson = localStorage.getItem('nh_admin_user');
    if (token && userJson) {
      try {
        var user = JSON.parse(userJson);
        updateSidebarUser(user);
        $('#sidebar').style.display = '';
      } catch (e) {}
    }
    renderPage();
  });

  // 点击 modal 遮罩关闭
  document.addEventListener('click', function (e) {
    if (e.target.id === 'modal-overlay') closeModal();
  });

  // 暴露
  window.navigate = navigate;
  window.renderPage = renderPage;
  window.doLogout = doLogout;
  window.doRemoveBinding = doRemoveBinding;
  window.closeModal = closeModal;
  window.openEditFarmerModal = openEditFarmerModal;
  window.doUpdateFarmer = doUpdateFarmer;
  window.goFarmerDetail = goFarmerDetail;
  window.openBindDeviceModal = openBindDeviceModal;
  window.doBindDevice = doBindDevice;
  window.openBindingAddModal = openBindingAddModal;
  window.doBindFromAddModal = doBindFromAddModal;
  window.switchDashboardRange = switchDashboardRange;
  window.payloadResetFilters = payloadResetFilters;
  window.payloadGoPage = payloadGoPage;
  window.payloadCopySnapshot = payloadCopySnapshot;
  window.payloadViewDetail = payloadViewDetail;

})();
