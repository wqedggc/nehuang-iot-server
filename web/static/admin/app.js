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
    return { page: hash };
  }

  // ============================================================
  // 页面渲染
  // ============================================================
  function renderPage() {
    var route = parseHash();
    currentPage = route.page;
    var container = $('#main-content');

    if (route.page !== 'login' && !localStorage.getItem('admin_token')) {
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
      case 'logs': renderLogs(container); updatePageTitle('控制日志'); break;
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
            '<input class="form-input" id="login-username" type="text" placeholder="请输入用户名" value="admin">' +
          '</div>' +
          '<div class="form-group">' +
            '<label class="form-label">密码</label>' +
            '<input class="form-input" id="login-password" type="password" placeholder="请输入密码" value="admin123">' +
          '</div>' +
          '<button class="btn btn-primary btn-block" id="login-submit" style="margin-top:8px">登录</button>' +
          '<p style="font-size:11px;color:var(--color-text-muted);text-align:center;margin-top:12px">测试账号：admin / admin123</p>' +
        '</div>' +
      '</div>';

    // Safari 安全策略：innerHTML 动态插入的 password input 会丢失 value
    // 必须通过 JS 显式设置，不能依赖 HTML value 属性
    var usernameInput = $('#login-username');
    var passwordInput = $('#login-password');
    usernameInput.value = 'admin';
    passwordInput.value = 'admin123';

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
      var res = await MockAPI.login('admin', { username: username, password: password });
      if (res.data && res.data.token) {
        localStorage.setItem('admin_token', res.data.token);
        localStorage.setItem('admin_user', JSON.stringify(res.data));
        MockAPI.setUserId(res.data.user_id);
        updateSidebarUser(res.data);
        showToast('登录成功', 'success');
        $('#sidebar').style.display = '';
        navigate('dashboard');
      } else {
        showToast(res.message || '登录失败', 'error');
      }
    } catch (e) {
      // 显示具体错误原因，方便调试（CORS/超时/mock 密码错误等）
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
    localStorage.removeItem('admin_token');
    localStorage.removeItem('admin_user');
    showToast('已退出登录', 'info');
    navigate('login');
  }

  // ---------- Dashboard ----------

  var dashboardRange = '1d'; // 当前选中的时间范围

  function buildDashboardRangeBar() {
    var ranges = [
      { value: '1d', label: '近1天' },
      { value: '3d', label: '近3天' },
      { value: '7d', label: '近7天' },
      { value: '30d', label: '近30天' }
    ];
    var html = '<div class="dashboard-range-bar" style="display:flex;gap:8px;margin-bottom:16px">';
    for (var i = 0; i < ranges.length; i++) {
      var r = ranges[i];
      var activeClass = r.value === dashboardRange ? ' btn-primary' : ' btn-outline';
      html += '<button class="btn btn-sm' + activeClass + '" onclick="dashboardSetRange(\'' + r.value + '\')">' + r.label + '</button>';
    }
    html += '</div>';
    return html;
  }

  function dashboardSetRange(rangeStr) {
    dashboardRange = rangeStr;
    renderPage();
  }

  function formatPercent(val) {
    if (val == null || isNaN(val)) return '--';
    return val.toFixed(2) + '%';
  }

  function formatLatestTime(iso) {
    if (!iso) return '--';
    try {
      var d = new Date(iso);
      var pad = function (n) { return n.toString().padStart(2, '0'); };
      return d.getFullYear() + '-' + pad(d.getMonth() + 1) + '-' + pad(d.getDate()) + ' ' + pad(d.getHours()) + ':' + pad(d.getMinutes()) + ':' + pad(d.getSeconds());
    } catch (e) { return iso; }
  }

  async function renderDashboard(container) {
    container.innerHTML = '<div class="loading-state"><div class="loading-spinner"></div><p>加载仪表盘...</p></div>';

    try {
      var res = await MockAPI.getDashboardRange(dashboardRange);

      // 处理 HTTP 错误状态
      if (res.code && res.code !== 200) {
        // 401: 跳登录
        if (res.code === 401 || res.code === 4011 || res.code === 4012 || res.code === 4013) {
          localStorage.removeItem('admin_token');
          localStorage.removeItem('admin_user');
          navigate('login');
          return;
        }
        // 403: 无管理员权限
        if (res.code === 403 || res.code === 4031 || res.code === 4033) {
          container.innerHTML = '<div class="error-state"><p>无管理员权限，请使用管理员账号登录</p><button class="btn btn-outline btn-sm" onclick="doLogout()">重新登录</button></div>';
          return;
        }
        // 400: 参数错误
        if (res.code === 400) {
          container.innerHTML = '<div class="error-state"><p>请求参数错误：' + escHtml(res.message || '') + '</p><button class="btn btn-outline btn-sm" onclick="renderPage()">重试</button></div>';
          return;
        }
        // 其他错误
        container.innerHTML = '<div class="error-state"><p>' + escHtml(res.message || '服务器错误') + '</p><button class="btn btn-outline btn-sm" onclick="renderPage()">重试</button></div>';
        return;
      }

      // 不允许 fallback mock：data 为空视为错误
      if (!res.data) {
        container.innerHTML = '<div class="error-state"><p>获取仪表盘数据失败</p><button class="btn btn-outline btn-sm" onclick="renderPage()">重试</button></div>';
        return;
      }

      var d = res.data;
      var u = d.users || {};
      var dev = d.devices || {};
      var ing = d.ingest || {};
      var ctl = d.control || {};

      var html = buildDashboardRangeBar();

      // ── 第一行：用户统计 ──
      html += '<div class="stat-grid">' +
        '<div class="stat-card"><div class="stat-card-label">总用户数</div><div class="stat-card-value">' + (u.total_users || 0).toLocaleString() + '</div><div class="stat-card-sub">管理员 ' + (u.admin_count || 0) + ' / 农户 ' + (u.farmer_count || 0) + '</div></div>' +
        '<div class="stat-card"><div class="stat-card-label">活跃农户</div><div class="stat-card-value">' + (u.active_farmer_count || 0).toLocaleString() + '</div></div>' +
        '<div class="stat-card"><div class="stat-card-label">禁用农户</div><div class="stat-card-value">' + (u.disabled_farmer_count || 0).toLocaleString() + '</div></div>' +
      '</div>';

      // ── 第二行：设备统计 ──
      html += '<div class="stat-grid">' +
        '<div class="stat-card"><div class="stat-card-label">云边设备</div><div class="stat-card-value">' + (dev.gateway_online || 0) + '<span style="font-size:16px;font-weight:400;color:var(--color-text-muted)"> / ' + (dev.gateway_total || 0) + '</span></div><div class="stat-card-sub">在线 / 总数</div></div>' +
        '<div class="stat-card"><div class="stat-card-label">采集执行器</div><div class="stat-card-value">' + (dev.collector_node_total || 0).toLocaleString() + '</div></div>' +
        '<div class="stat-card"><div class="stat-card-label">功能点</div><div class="stat-card-value">' + (dev.device_point_total || 0).toLocaleString() + '</div><div class="stat-card-sub">已绑定 ' + (dev.bound_point_count || 0) + ' / 未绑定 ' + (dev.unbound_point_count || 0) + '</div></div>' +
      '</div>';

      // ── 第三行：设备状态 ──
      html += '<div class="stat-grid">' +
        '<div class="stat-card"><div class="stat-card-label">正常功能点</div><div class="stat-card-value" style="color:var(--color-success)">' + (dev.normal_point_count || 0).toLocaleString() + '</div></div>' +
        '<div class="stat-card"><div class="stat-card-label">异常功能点</div><div class="stat-card-value" style="color:var(--color-warning)">' + (dev.abnormal_point_count || 0).toLocaleString() + '</div></div>' +
        '<div class="stat-card"><div class="stat-card-label">离线功能点</div><div class="stat-card-value" style="color:var(--color-danger)">' + (dev.offline_point_count || 0).toLocaleString() + '</div></div>' +
        '<div class="stat-card"><div class="stat-card-label">最近上报时间</div><div class="stat-card-value" style="font-size:16px">' + formatLatestTime(dev.latest_report_time) + '</div></div>' +
      '</div>';

      // ── 第四行：数据上报统计 ──
      html += '<div class="stat-grid">' +
        '<div class="stat-card"><div class="stat-card-label">原始事件数</div><div class="stat-card-value">' + (ing.raw_event_count || 0).toLocaleString() + '</div></div>' +
        '<div class="stat-card"><div class="stat-card-label">遥测数据</div><div class="stat-card-value">' + (ing.telemetry_count || 0).toLocaleString() + '</div></div>' +
        '<div class="stat-card"><div class="stat-card-label">注册事件</div><div class="stat-card-value">' + (ing.register_count || 0).toLocaleString() + '</div></div>' +
        '<div class="stat-card"><div class="stat-card-label">工作上报</div><div class="stat-card-value">' + (ing.work_count || 0).toLocaleString() + '</div></div>' +
        '<div class="stat-card"><div class="stat-card-label">控制结果</div><div class="stat-card-value">' + (ing.control_result_count || 0).toLocaleString() + '</div></div>' +
      '</div>';

      // ── 第五行：心跳 + 控制成功率 ──
      html += '<div class="stat-grid">' +
        '<div class="stat-card"><div class="stat-card-label">心跳总数</div><div class="stat-card-value">' + (ing.heartbeat_total || 0).toLocaleString() + '</div><div class="stat-card-sub">成功 ' + (ing.heartbeat_success || 0) + ' / 失败 ' + (ing.heartbeat_failed || 0) + '</div></div>' +
        '<div class="stat-card"><div class="stat-card-label">心跳成功率</div><div class="stat-card-value">' + formatPercent(ing.heartbeat_success_rate) + '</div></div>' +
        '<div class="stat-card"><div class="stat-card-label">控制命令</div><div class="stat-card-value">' + (ctl.control_command_count || 0).toLocaleString() + '</div><div class="stat-card-sub">成功 ' + (ctl.control_success_count || 0) + ' / 失败 ' + (ctl.control_failed_count || 0) + ' / 超时 ' + (ctl.control_timeout_count || 0) + '</div></div>' +
        '<div class="stat-card"><div class="stat-card-label">控制成功率</div><div class="stat-card-value">' + formatPercent(ctl.control_success_rate) + '</div></div>' +
      '</div>';

      container.innerHTML = html;
    } catch (e) {
      // 网络错误不显示 mock 数据
      container.innerHTML = '<div class="error-state"><p>网络请求失败，请检查服务是否可用</p><button class="btn btn-outline btn-sm" onclick="renderPage()">重试</button></div>';
    }
  }

  // ---------- 用户管理 ----------
  async function renderUsers(container) {
    container.innerHTML = '<div class="loading-state"><div class="loading-spinner"></div><p>加载用户列表...</p></div>';
    try {
      var res = await MockAPI.getUsers();
      if (!res.data) { container.innerHTML = '<div class="error-state"><p>加载失败</p><button class="btn btn-outline btn-sm" onclick="renderPage()">重试</button></div>'; return; }
      var users = res.data;

      var html = '<div class="table-section">' +
        '<div class="table-header">' +
          '<h3>用户列表</h3>' +
          '<div class="table-header-right">' +
            '<input class="table-search" id="user-search" type="text" placeholder="搜索用户名...">' +
            '<span class="table-count">共 ' + users.length + ' 人</span>' +
          '</div>' +
        '</div>' +
        '<div class="table-wrap"><table>' +
          '<thead><tr><th>ID</th><th>用户名</th><th>显示名称</th><th>角色</th><th>绑定设备</th><th>状态</th></tr></thead>' +
          '<tbody id="user-tbody">' +
            users.map(function (u) {
              var roleBadge = u.role === 1 ? 'badge-danger' : u.role === 2 ? 'badge-info' : 'badge-success';
              var statusBadge = u.status === 1 ? 'badge-success' : u.status === 2 ? 'badge-warning' : 'badge-muted';
              return '<tr>' +
                '<td>' + u.user_id + '</td>' +
                '<td>' + escHtml(u.username) + '</td>' +
                '<td>' + escHtml(u.display_name) + '</td>' +
                '<td><span class="badge ' + roleBadge + '">' + escHtml(u.role_label) + '</span></td>' +
                '<td>' + u.binding_count + ' 台</td>' +
                '<td><span class="badge ' + statusBadge + '">' + escHtml(u.status_label) + '</span></td>' +
              '</tr>';
            }).join('') +
          '</tbody>' +
        '</table></div>' +
      '</div>';

      container.innerHTML = html;

      // 搜索
      $('#user-search').oninput = function () {
        var q = this.value.toLowerCase();
        $$('#user-tbody tr').forEach(function (tr) {
          tr.style.display = tr.textContent.toLowerCase().includes(q) ? '' : 'none';
        });
      };
    } catch (e) {
      container.innerHTML = '<div class="error-state"><p>加载失败</p><button class="btn btn-outline btn-sm" onclick="renderPage()">重试</button></div>';
    }
  }

  // ---------- 设备管理 ----------
  async function renderGateways(container) {
    container.innerHTML = '<div class="loading-state"><div class="loading-spinner"></div><p>加载设备数据...</p></div>';
    try {
      var gwRes = await MockAPI.getGateways();
      var clRes = await MockAPI.getCollectors();
      if (!gwRes.data || !clRes.data) { container.innerHTML = '<div class="error-state"><p>加载失败</p><button class="btn btn-outline btn-sm" onclick="renderPage()">重试</button></div>'; return; }
      var gateways = gwRes.data;
      var collectors = clRes.data;

      // 云边设备表
      var html = '<div class="table-section" style="margin-bottom:18px">' +
        '<div class="table-header"><h3>云边设备</h3><span class="table-count">共 ' + gateways.length + ' 台</span></div>' +
        '<div class="table-wrap"><table>' +
          '<thead><tr><th>ID</th><th>设备名称</th><th>IP</th><th>端口</th><th>SN</th><th>状态</th><th>最近心跳</th></tr></thead>' +
          '<tbody>' +
            gateways.map(function (g) {
              var statusHtml = g.online_status === 1
                ? '<span class="status-dot status-dot-online"></span>在线'
                : '<span class="status-dot status-dot-offline"></span>离线';
              return '<tr>' +
                '<td>' + g.gateway_id + '</td>' +
                '<td>' + escHtml(g.gateway_name) + '</td>' +
                '<td>' + escHtml(g.ip) + '</td>' +
                '<td>' + g.port + '</td>' +
                '<td>' + escHtml(g.gateway_sn || '--') + '</td>' +
                '<td>' + statusHtml + '</td>' +
                '<td>' + formatTime(g.last_heartbeat_at_ms) + '</td>' +
              '</tr>';
            }).join('') +
          '</tbody>' +
        '</table></div>' +
      '</div>';

      // 采集执行器表
      html += '<div class="table-section">' +
        '<div class="table-header">' +
          '<h3>采集执行器 / 功能点</h3>' +
          '<div class="table-header-right">' +
            '<input class="table-search" id="collector-search" type="text" placeholder="搜索棚号/设备...">' +
            '<span class="table-count">共 ' + collectors.length + ' 个</span>' +
          '</div>' +
        '</div>' +
        '<div class="table-wrap"><table>' +
          '<thead><tr><th>ID</th><th>云边设备</th><th>棚号</th><th>频点</th><th>设备ID</th><th>功能</th><th>实时值</th><th>单位</th><th>状态</th><th>更新时间</th></tr></thead>' +
          '<tbody id="collector-tbody">' +
            collectors.map(function (c) {
              var statusClass = 'status-dot-' + (c.status === 'normal' || c.status === 'on' ? 'normal' : c.status === 'offline' ? 'offline' : 'warning');
              return '<tr>' +
                '<td>' + c.de_re_id + '</td>' +
                '<td>' + escHtml(c.gateway_name) + '</td>' +
                '<td>' + escHtml(c.shed_no) + '</td>' +
                '<td>' + c.freq + '</td>' +
                '<td>' + c.devid + '</td>' +
                '<td>' + escHtml(c.func_name) + '</td>' +
                '<td>' + (c.real_value != null ? c.real_value : '--') + '</td>' +
                '<td>' + escHtml(c.unit) + '</td>' +
                '<td><span class="status-dot ' + statusClass + '"></span>' + escHtml(c.status_label) + '</td>' +
                '<td>' + formatTime(c.updated_at) + '</td>' +
              '</tr>';
            }).join('') +
          '</tbody>' +
        '</table></div>' +
      '</div>';

      container.innerHTML = html;

      $('#collector-search').oninput = function () {
        var q = this.value.toLowerCase();
        $$('#collector-tbody tr').forEach(function (tr) {
          tr.style.display = tr.textContent.toLowerCase().includes(q) ? '' : 'none';
        });
      };
    } catch (e) {
      container.innerHTML = '<div class="error-state"><p>加载失败</p><button class="btn btn-outline btn-sm" onclick="renderPage()">重试</button></div>';
    }
  }

  // ---------- 绑定管理 ----------
  async function renderBindings(container) {
    container.innerHTML = '<div class="loading-state"><div class="loading-spinner"></div><p>加载绑定关系...</p></div>';
    try {
      var res = await MockAPI.getBindings();
      if (!res.data) { container.innerHTML = '<div class="error-state"><p>加载失败</p><button class="btn btn-outline btn-sm" onclick="renderPage()">重试</button></div>'; return; }
      var bindings = res.data;

      var html = '<div class="table-section">' +
        '<div class="table-header">' +
          '<h3>绑定关系</h3>' +
          '<div class="table-header-right">' +
            '<button class="btn btn-primary btn-sm" onclick="openAddBindingModal()">+ 新增绑定</button>' +
            '<span class="table-count">共 ' + bindings.length + ' 条</span>' +
          '</div>' +
        '</div>' +
        '<div class="table-wrap"><table>' +
          '<thead><tr><th>ID</th><th>农户</th><th>云边设备ID</th><th>棚号</th><th>频点</th><th>设备ID</th><th>功能</th><th>操作</th></tr></thead>' +
          '<tbody>' +
            bindings.map(function (b) {
              var funcName = b.collector ? b.collector.func_name : ('功能' + b.func);
              return '<tr>' +
                '<td>' + b.bind_id + '</td>' +
                '<td>' + escHtml(b.bind_usr_name) + '</td>' +
                '<td>' + b.gateway_id + '</td>' +
                '<td>' + escHtml(b.shed_no) + '</td>' +
                '<td>' + b.freq + '</td>' +
                '<td>' + b.devid + '</td>' +
                '<td>' + escHtml(funcName) + '</td>' +
                '<td><button class="btn btn-danger btn-sm" onclick="doRemoveBinding(' + b.bind_id + ')">解绑</button></td>' +
              '</tr>';
            }).join('') +
          '</tbody>' +
        '</table></div>' +
      '</div>';

      container.innerHTML = html;
    } catch (e) {
      container.innerHTML = '<div class="error-state"><p>加载失败</p><button class="btn btn-outline btn-sm" onclick="renderPage()">重试</button></div>';
    }
  }

  function openAddBindingModal() {
    var overlay = $('#modal-overlay');
    overlay.style.display = 'flex';
    overlay.innerHTML =
      '<div class="modal">' +
        '<h3>新增绑定</h3>' +
        '<div class="form-group"><label class="form-label">农户名称</label><input class="form-input" id="bind-usr-name" placeholder="请输入农户名称" value="新农户"></div>' +
        '<div class="form-row">' +
          '<div class="form-group"><label class="form-label">云边设备 ID</label><input class="form-input" id="bind-gateway-id" type="number" placeholder="1" value="1"></div>' +
          '<div class="form-group"><label class="form-label">用户 ID</label><input class="form-input" id="bind-user-id" type="number" placeholder="3" value="3"></div>' +
        '</div>' +
        '<div class="form-row">' +
          '<div class="form-group"><label class="form-label">频点</label><input class="form-input" id="bind-freq" type="number" placeholder="1" value="1"></div>' +
          '<div class="form-group"><label class="form-label">设备 ID</label><input class="form-input" id="bind-devid" type="number" placeholder="1001" value="1001"></div>' +
        '</div>' +
        '<div class="form-row">' +
          '<div class="form-group"><label class="form-label">功能类型</label><select class="form-select" id="bind-func"><option value="1">温度</option><option value="2">墒情</option><option value="3">限位</option><option value="4">水泵</option><option value="5">风机</option><option value="6">继电器</option></select></div>' +
          '<div class="form-group"><label class="form-label">棚号</label><input class="form-input" id="bind-shed" placeholder="棚号" value="新棚-01"></div>' +
        '</div>' +
        '<div class="modal-actions">' +
          '<button class="btn btn-outline btn-sm" onclick="closeModal()">取消</button>' +
          '<button class="btn btn-primary btn-sm" onclick="doAddBinding()">确认绑定</button>' +
        '</div>' +
      '</div>';
  }

  function closeModal() {
    $('#modal-overlay').style.display = 'none';
  }

  async function doAddBinding() {
    var data = {
      bind_usr_name: $('#bind-usr-name').value.trim(),
      gateway_id: parseInt($('#bind-gateway-id').value) || 1,
      user_id: parseInt($('#bind-user-id').value) || 3,
      freq: parseInt($('#bind-freq').value) || 1,
      devid: parseInt($('#bind-devid').value) || 1001,
      func: parseInt($('#bind-func').value) || 1,
      shed_no: $('#bind-shed').value.trim(),
    };
    if (!data.bind_usr_name) { showToast('请输入农户名称', 'error'); return; }

    try {
      var res = await MockAPI.addBinding(data);
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
      var res = await MockAPI.removeBinding(id);
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
      var res = await MockAPI.getControlLogs();
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
    var token = localStorage.getItem('admin_token');
    var userJson = localStorage.getItem('admin_user');
    if (token && userJson) {
      try {
        var user = JSON.parse(userJson);
        MockAPI.setUserId(user.user_id);
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
  window.dashboardSetRange = dashboardSetRange;
  window.navigate = navigate;
  window.renderPage = renderPage;
  window.doLogout = doLogout;
  window.doRemoveBinding = doRemoveBinding;
  window.openAddBindingModal = openAddBindingModal;
  window.closeModal = closeModal;
  window.doAddBinding = doAddBinding;

})();
