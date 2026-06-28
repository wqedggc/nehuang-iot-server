/**
 * 涅凰智农 IoT 云平台 — API 层
 *
 * 用法：
 *   <script src="../mock/mock-api.js"></script>
 *   后续通过 window.MockAPI 调用。
 *
 * 运行模式（修改 MODE 即可切换）：
 *   'mock'    — 纯 mock，不请求网络
 *   'real'    — 纯真实后端，mock 数据作为 fallback（推荐）
 *   'offline' — 同 mock
 *
 * 切换真实后端地址：
 *   修改 API_BASE 为真实 Go 服务地址。
 *
 * Token 命名约定：
 *   nh_admin_token — 管理端 JWT（admin/super_admin）
 *   nh_mini_token  — C 端 JWT（farmer）
 */

(function (global) {
  'use strict';

  // ============================================================
  // 配置
  // ============================================================
  // 运行模式：
  //   'mock'  — 纯本地 mock，不请求网络（推荐开发时使用）
  //   'real'  — 请求真实后端，失败则自动 fallback mock（部署后使用）
  const MODE = 'real'; // ← 已对接真实后端
  const API_BASE = 'http://43.143.208.153:8080'; // 真实后端地址
  const MOCK_DELAY = 100; // mock 模拟网络延迟（ms）

  // C 端用户 ID（mock 模式使用）
  let currentUserId = 3;

  // ============================================================
  // Token 工具
  // ============================================================

  /**
   * 根据接口路径自动选择对应的 token：
   *   /api/v1/admin/* → nh_admin_token
   *   /api/v1/mini/*  → nh_mini_token
   *   其他接口        → 不带 token
   */
  function getTokenForPath(path) {
    // login 接口不需要带 token（可能会带一个已过期的 token 造成困扰）
    if (path === '/api/v1/admin/login' || path === '/api/v1/mini/mock-login') {
      return null;
    }
    if (path.indexOf('/api/v1/admin/') === 0 || path === '/api/v1/admin') {
      return localStorage.getItem('nh_admin_token') || null;
    }
    if (path.indexOf('/api/v1/mini/') === 0 || path === '/api/v1/mini') {
      return localStorage.getItem('nh_mini_token') || null;
    }
    // /api/v1/gateways 等通用接口 — 优先 admin token，其次 mini token
    return localStorage.getItem('nh_admin_token') || localStorage.getItem('nh_mini_token') || null;
  }

  // ============================================================
  // Mock 数据
  // ============================================================

  const MOCK_USERS = [
    { user_id: 1, user_type: 1, role: 1, username: 'admin', display_name: '超级管理员', status: 1 },
    { user_id: 2, user_type: 1, role: 2, username: 'operator', display_name: '运维管理员', status: 1 },
    { user_id: 3, user_type: 2, role: 3, display_name: '张三农场', status: 1 },
    { user_id: 4, user_type: 2, role: 3, display_name: '李四果园', status: 1 },
    { user_id: 5, user_type: 2, role: 3, display_name: '王五蔬菜基地', status: 1 },
    { user_id: 6, user_type: 2, role: 3, display_name: '赵六水产', status: 2 },
  ];

  const MOCK_GATEWAYS = [
    { gateway_id: 1, gateway_external_id: 'TEST_ADMIN_001', gateway_name: 'FP_TEST_ADMIN_001', ip: '123.190.0.47', port: 40000, gateway_sn: '', online_status: 1, last_heartbeat_at_ms: '2026-06-28T21:50:00.000Z' },
    { gateway_id: 2, gateway_external_id: 'TEST_ADMIN_002', gateway_name: 'FP_TEST_ADMIN_002', ip: '123.190.0.48', port: 40001, gateway_sn: '', online_status: 1, last_heartbeat_at_ms: '2026-06-28T21:51:00.000Z' },
    { gateway_id: 3, gateway_external_id: 'TEST_ADMIN_003', gateway_name: 'FP_TEST_ADMIN_003', ip: '123.190.0.49', port: 40002, gateway_sn: '', online_status: 0, last_heartbeat_at_ms: '2026-06-28T20:30:00.000Z' },
  ];

  function generateCollectors() {
    const funcTypes = [
      { func: 1, name: '温度', unit: '℃', best_min: 18, best_max: 30, run_mode: 1, real_value: 25.6, status: 'normal' },
      { func: 2, name: '墒情', unit: '%', best_min: 40, best_max: 80, run_mode: 1, real_value: 55.2, status: 'normal' },
      { func: 3, name: '限位', unit: '', best_min: 0, best_max: 1, run_mode: 1, real_value: 0, status: 'normal' },
      { func: 4, name: '水泵', unit: '', best_min: 0, best_max: 1, run_mode: 2, real_value: 1, status: 'on' },
      { func: 5, name: '风机', unit: '', best_min: 0, best_max: 1, run_mode: 2, real_value: 0, status: 'off' },
      { func: 6, name: '继电器', unit: '', best_min: 0, best_max: 1, run_mode: 2, real_value: 1, status: 'on' },
    ];
    const statusMap = { normal: '正常', low: '偏低', high: '偏高', offline: '离线', on: '开启', off: '关闭' };
    const collectors = [];
    let id = 0;
    for (let g = 0; g < MOCK_GATEWAYS.length; g++) {
      const gw = MOCK_GATEWAYS[g];
      const count = 3 + Math.floor(Math.random() * 4);
      for (let c = 0; c < count; c++) {
        id++;
        const ft = funcTypes[c % funcTypes.length];
        let realValue = ft.real_value;
        let status = ft.status;
        if (id === 2) { realValue = 35.2; status = 'high'; }
        if (id === 8) { realValue = 15.3; status = 'low'; }
        if (id === 15) { status = 'offline'; }
        if (gw.online_status === 0) { status = 'offline'; }
        collectors.push({
          de_re_id: id,
          gateway_id: gw.gateway_id,
          gateway_name: gw.gateway_name,
          shed_no: '棚' + (g + 1) + '-' + (c + 1).toString().padStart(2, '0'),
          collector_no: 'DEV-' + id.toString().padStart(4, '0'),
          freq: 1 + (c % 6),
          devid: 1000 + id,
          func: ft.func,
          func_name: ft.name,
          best_min: ft.best_min,
          best_max: ft.best_max,
          unit: ft.unit,
          real_temp: realValue,
          real_value: realValue,
          run_mode: ft.run_mode,
          run_mode_label: ft.run_mode === 1 ? '自动' : '手动',
          rel1: 0, rel2: 0,
          poll_interval: 300,
          status: status,
          status_label: statusMap[status] || '未知',
          updated_at: '2026-06-28T21:' + (50 - id).toString().padStart(2, '0') + ':00.000Z',
        });
      }
    }
    return collectors;
  }
  const MOCK_COLLECTORS = generateCollectors();

  const MOCK_BINDINGS = [
    { bind_id: 1, user_id: 3, gateway_id: 1, shed_no: '棚1-01', freq: 1, devid: 1001, func: 1, bind_usr_name: '张三农场', point_id: 1 },
    { bind_id: 2, user_id: 3, gateway_id: 1, shed_no: '棚1-02', freq: 2, devid: 1002, func: 2, bind_usr_name: '张三农场', point_id: 11 },
    { bind_id: 3, user_id: 3, gateway_id: 1, shed_no: '棚1-03', freq: 3, devid: 1003, func: 3, bind_usr_name: '张三农场', point_id: 21 },
    { bind_id: 4, user_id: 3, gateway_id: 2, shed_no: '棚2-01', freq: 1, devid: 1011, func: 4, bind_usr_name: '张三农场', point_id: 101 },
    { bind_id: 5, user_id: 4, gateway_id: 1, shed_no: '棚1-04', freq: 4, devid: 1004, func: 5, bind_usr_name: '李四果园', point_id: 31 },
    { bind_id: 6, user_id: 4, gateway_id: 1, shed_no: '棚1-05', freq: 5, devid: 1005, func: 6, bind_usr_name: '李四果园', point_id: 41 },
    { bind_id: 7, user_id: 5, gateway_id: 2, shed_no: '棚2-02', freq: 2, devid: 1012, func: 1, bind_usr_name: '王五蔬菜基地', point_id: 111 },
    { bind_id: 8, user_id: 5, gateway_id: 2, shed_no: '棚2-03', freq: 3, devid: 1013, func: 2, bind_usr_name: '王五蔬菜基地', point_id: 121 },
    { bind_id: 9, user_id: 5, gateway_id: 2, shed_no: '棚2-04', freq: 4, devid: 1014, func: 3, bind_usr_name: '王五蔬菜基地', point_id: 201 },
  ];

  // 扩展农户 mock 数据（包含 address, mobile, wechat_openid, remark, created_at）
  const MOCK_FARMERS = [
    { user_id: 3, user_type: 2, role: 3, username: 'farmer_zhang', display_name: '张三农场', mobile: '13800001111', address: '山东省潍坊市寿光市大棚区A-01', wechat_openid: 'wx_openid_zhang_001', remark: '寿光蔬菜大棚种植户', status: 1, status_label: '正常', binding_count: 4, created_at: '2026-03-15T10:30:00Z' },
    { user_id: 4, user_type: 2, role: 3, username: 'farmer_li', display_name: '李四果园', mobile: '13800002222', address: '山东省烟台市栖霞市果园路8号', wechat_openid: 'wx_openid_li_002', remark: '苹果种植大户', status: 1, status_label: '正常', binding_count: 2, created_at: '2026-04-20T14:00:00Z' },
    { user_id: 5, user_type: 2, role: 3, username: 'farmer_wang', display_name: '王五蔬菜基地', mobile: '13800003333', address: '山东省临沂市兰陵县蔬菜基地B-03', wechat_openid: 'wx_openid_wang_003', remark: '大棚蔬菜合作社', status: 1, status_label: '正常', binding_count: 3, created_at: '2026-05-10T08:15:00Z' },
    { user_id: 6, user_type: 2, role: 3, username: 'farmer_zhao', display_name: '赵六水产', mobile: '13800004444', address: '江苏省连云港市赣榆区水产养殖场', wechat_openid: 'wx_openid_zhao_004', remark: '水产养殖户', status: 2, status_label: '已禁用', binding_count: 0, created_at: '2026-06-01T16:45:00Z' },
  ];

  // 设备树 mock 数据
  function generateDeviceTree() {
    const funcTypes = [
      { func: 1, name: '温度', unit: '℃' },
      { func: 2, name: '墒情', unit: '%' },
      { func: 3, name: '限位', unit: '' },
      { func: 4, name: '水泵', unit: '' },
      { func: 5, name: '风机', unit: '' },
      { func: 6, name: '继电器', unit: '' },
    ];
    const tree = [];
    MOCK_GATEWAYS.forEach((gw, gi) => {
      const collectors = [];
      const count = 3 + Math.floor(Math.random() * 4);
      for (let ci = 0; ci < count; ci++) {
        const ft = funcTypes[ci % funcTypes.length];
        const points = [{
          point_id: gi * 100 + ci * 10 + 1,
          point_name: ft.name + '传感器',
          display_name: ft.name + '传感器',
          function_type: ft.func,
          channel_no: 1,
          freq: ci + 1,
          devid: 1000 + gi * 10 + ci + 1,
          func: ft.func,
          func_name: ft.name,
          unit: ft.unit,
          shed_no: '棚' + (gi + 1) + '-' + (ci + 1).toString().padStart(2, '0'),
        }];
        collectors.push({
          collector_id: gi * 10 + ci + 1,
          collector_no: 'DEV-' + (gi * 10 + ci + 1).toString().padStart(4, '0'),
          node_id: gi * 10 + ci + 1,
          frequency: ci + 1,
          device_external_id: 'DEV-' + (gi * 10 + ci + 1).toString().padStart(4, '0'),
          shed_no: '棚' + (gi + 1) + '-' + (ci + 1).toString().padStart(2, '0'),
          points: points,
        });
      }
      tree.push({
        gateway_id: gw.gateway_id,
        gateway_name: gw.gateway_name,
        gateway_external_id: gw.gateway_external_id,
        online_status: gw.online_status,
        collectors: collectors,
      });
    });
    return tree;
  }
  const MOCK_DEVICE_TREE = generateDeviceTree();

  function generateControlLogs() {
    const ops = ['开启', '停止', '关闭'];
    const statuses = ['succeeded', 'failed', 'timeout'];
    const statusLabels = { succeeded: '成功', failed: '失败', timeout: '超时' };
    const logs = [];
    for (let i = 0; i < 25; i++) {
      const collector = MOCK_COLLECTORS[i % MOCK_COLLECTORS.length];
      logs.push({
        op_id: i + 1,
        gateway_id: collector.gateway_id,
        gateway_name: collector.gateway_name,
        bind_usr_name: ['张三农场', '李四果园', '王五蔬菜基地'][i % 3],
        freq: collector.freq,
        devid: collector.devid,
        func: collector.func,
        func_name: collector.func_name,
        op_type: ops[i % 3],
        run_mode: collector.run_mode,
        cmd_status: statuses[i % 3],
        cmd_status_label: statusLabels[statuses[i % 3]],
        result: statuses[i % 3] === 'succeeded' ? '执行成功' : statuses[i % 3] === 'failed' ? '设备无响应' : '超时未确认',
        op_time: '2026-06-28T' + (20 + Math.floor(i / 6)).toString().padStart(2, '0') + ':' + ((i * 7) % 60).toString().padStart(2, '0') + ':00.000Z',
      });
    }
    return logs;
  }
  const MOCK_CONTROL_LOGS = generateControlLogs();

  function generateHistory(range) {
    const points = [];
    const now = new Date('2026-06-28T21:54:00.000Z');
    let count, stepMs;
    switch (range) {
      case '1h': count = 60; stepMs = 60 * 1000; break;
      case '24h': count = 144; stepMs = 10 * 60 * 1000; break;
      case '7d': count = 168; stepMs = 60 * 60 * 1000; break;
      default: count = 60; stepMs = 60 * 1000;
    }
    let baseTemp = 25 + Math.random() * 2;
    let baseHumidity = 55 + Math.random() * 5;
    for (let i = count - 1; i >= 0; i--) {
      const t = new Date(now.getTime() - i * stepMs);
      baseTemp += (Math.random() - 0.5) * 0.8;
      baseTemp = Math.max(10, Math.min(40, baseTemp));
      baseHumidity += (Math.random() - 0.5) * 2;
      baseHumidity = Math.max(20, Math.min(90, baseHumidity));
      points.push({
        time: t.toISOString(),
        real_temp: Math.round(baseTemp * 10) / 10,
        real_humidity: Math.round(baseHumidity * 10) / 10,
      });
    }
    return points;
  }

  function getDashboard() {
    const onlineGateways = MOCK_GATEWAYS.filter(g => g.online_status === 1).length;
    return {
      online_gateways: onlineGateways,
      total_gateways: MOCK_GATEWAYS.length,
      total_collectors: MOCK_COLLECTORS.length,
      farmer_count: MOCK_USERS.filter(u => u.user_type === 2 && u.status === 1).length,
      today_reports: 12847,
      last_report_time: '2026-06-28T21:53:45.000Z',
    };
  }

  let nextBindId = 10;

  // ============================================================
  // Mock 路由处理器
  // ============================================================
  const mockHandlers = {
    'POST /api/v1/mini/mock-login': function (body) {
      let user = MOCK_USERS.find(u => u.user_type === 2 && u.status === 1) || MOCK_USERS[2];
      return { data: { token: 'mock_mini_token_' + user.user_id, user_id: user.user_id, is_new_user: false } };
    },
    'GET /api/v1/mini/me': function () {
      const uid = getCurrentUserId();
      const user = MOCK_USERS.find(u => u.user_id === uid) || MOCK_USERS[2];
      const bindingCount = MOCK_BINDINGS.filter(b => b.user_id === uid).length;
      // 尝试从 MOCK_FARMERS 中获取更完整的资料
      const farmer = MOCK_FARMERS.find(f => f.user_id === uid);
      const mobile = farmer ? farmer.mobile : '';
      const address = farmer ? farmer.address : '';
      const remark = farmer ? farmer.remark : '';
      return { data: { user_id: user.user_id, display_name: user.display_name, mobile: mobile, address: address, remark: remark, role: user.role, binding_count: bindingCount } };
    },
    'PATCH /api/v1/mini/me': function (params, body) {
      const uid = getCurrentUserId();
      const farmerIdx = MOCK_FARMERS.findIndex(f => f.user_id === uid);
      if (farmerIdx < 0) return { code: 404, message: 'user not found' };

      const allowed = ['display_name', 'mobile', 'address', 'remark'];
      allowed.forEach(k => {
        if (body[k] !== undefined && body[k] !== null && body[k] !== '') {
          MOCK_FARMERS[farmerIdx][k] = body[k];
        }
      });

      // 同步更新 MOCK_USERS 中的 display_name
      const userIdx = MOCK_USERS.findIndex(u => u.user_id === uid);
      if (userIdx >= 0 && body.display_name) {
        MOCK_USERS[userIdx].display_name = body.display_name;
      }

      const f = MOCK_FARMERS[farmerIdx];
      return { data: { user_id: f.user_id, display_name: f.display_name, mobile: f.mobile, address: f.address, remark: f.remark, role: f.role } };
    },
    'GET /api/v1/mini/devices': function () {
      const uid = getCurrentUserId();
      const bindings = MOCK_BINDINGS.filter(b => b.user_id === uid);
      const devices = bindings.map(b => {
        const c = MOCK_COLLECTORS.find(x => x.gateway_id === b.gateway_id && x.freq === b.freq && x.devid === b.devid);
        if (!c) return null;
        return {
          de_re_id: c.de_re_id, gateway_name: c.gateway_name, shed_no: c.shed_no,
          collector_no: c.collector_no, freq: c.freq, devid: c.devid,
          func: c.func, func_name: c.func_name, real_value: c.real_value, unit: c.unit,
          status: c.status, status_label: c.status_label,
          run_mode: c.run_mode, run_mode_label: c.run_mode_label, updated_at: c.updated_at,
        };
      }).filter(Boolean);
      return { data: devices };
    },
    'GET /api/v1/mini/devices/:id/realtime': function (params) {
      const c = MOCK_COLLECTORS.find(x => x.de_re_id === parseInt(params.id));
      return c ? { data: c } : { code: 404, message: 'device not found' };
    },
    'GET /api/v1/mini/devices/:id/history': function (params, query) {
      const c = MOCK_COLLECTORS.find(x => x.de_re_id === parseInt(params.id));
      if (!c) return { code: 404, message: 'device not found' };
      return { data: { de_re_id: c.de_re_id, func_name: c.func_name, unit: c.unit, best_min: c.best_min, best_max: c.best_max, points: generateHistory(query.range || '24h') } };
    },
    'POST /api/v1/mini/devices/:id/control': function (params, body) {
      const c = MOCK_COLLECTORS.find(x => x.de_re_id === parseInt(params.id));
      if (!c) return { code: 404, message: 'device not found' };
      const action = body.action || 'stop';
      const success = Math.random() > 0.1;
      if (success) {
        if (action === 'start') { c.real_value = 1; c.status = 'on'; c.status_label = '开启'; }
        else { c.real_value = 0; c.status = 'off'; c.status_label = '关闭'; }
      }
      return { data: { command_id: Date.now(), action, action_label: { start: '开启', stop: '停止', close: '关闭' }[action] || action, status: success ? 'succeeded' : 'failed', message: success ? '操作成功' : '设备无响应，请重试' } };
    },
    'POST /api/v1/admin/login': function (body) {
      console.debug('[MockAPI] admin/login body=', body);
      if (!body || !body.username || !body.password) return { code: 400, message: '用户名和密码不能为空' };
      if (body.password !== 'admin123') return { code: 4013, message: '用户名或密码错误' };
      const user = MOCK_USERS.find(u => u.username === body.username && (u.role === 1 || u.role === 2));
      if (!user) return { code: 4013, message: '用户名或密码错误' };
      return { data: { token: 'mock_admin_token_' + user.user_id, user_id: user.user_id, role: user.role, display_name: user.display_name } };
    },
    'GET /api/v1/admin/me': function () {
      const uid = getCurrentUserId();
      const user = MOCK_USERS.find(u => u.user_id === uid) || MOCK_USERS[0];
      return { data: { user_id: user.user_id, username: user.username || '', role: user.role, display_name: user.display_name } };
    },
    'GET /api/v1/admin/dashboard': function () {
      return { data: getDashboard() };
    },
    'GET /api/v1/admin/users': function () {
      const users = MOCK_USERS.map(u => {
        const bc = MOCK_BINDINGS.filter(b => b.user_id === u.user_id).length;
        return { user_id: u.user_id, username: u.username || '', display_name: u.display_name, role: u.role, role_label: { 1: '超级管理员', 2: '管理员', 3: '农户' }[u.role] || '', user_type: u.user_type, status: u.status, status_label: { 1: '正常', 2: '已禁用', 3: '已删除' }[u.status] || '', binding_count: bc };
      });
      return { data: users };
    },
    'GET /api/v1/gateways': function () {
      return { data: { items: MOCK_GATEWAYS.map(g => ({
        id: g.gateway_id, dev_id: g.gateway_external_id, dev_name: g.gateway_name,
        ip: g.ip, port: g.port, dev_sn: g.gateway_sn,
        online: g.online_status === 1,
        create_time: '2026-06-28T12:00:00Z',
        last_heartbeat: g.last_heartbeat_at_ms,
      })) } };
    },
    'GET /api/v1/admin/gateways': function () {
      return { data: MOCK_GATEWAYS.map(g => ({ ...g, online_label: g.online_status === 1 ? '在线' : '离线' })) };
    },
    'GET /api/v1/admin/collectors': function () {
      return { data: MOCK_COLLECTORS };
    },
    'GET /api/v1/admin/bindings': function () {
      return { data: MOCK_BINDINGS.map(b => {
        const c = MOCK_COLLECTORS.find(x => x.gateway_id === b.gateway_id && x.freq === b.freq && x.devid === b.devid);
        return { ...b, collector: c || null };
      }) };
    },
    'POST /api/v1/admin/bindings': function (params, body) {
      var userId = String(body.user_id);
      var pointId = String(body.point_id);

      // 冲突检测
      var existing = MOCK_BINDINGS.find(function (b) {
        return String(b.point_id) === pointId;
      });
      if (existing) {
        return {
          code: 409,
          message: 'device point already bound to another user',
          data: {
            existing_user_id: String(existing.user_id),
            existing_bind_id: String(existing.bind_id),
            existing_user_name: existing.bind_user_name || existing.bind_usr_name || ''
          }
        };
      }

      // 从 MOCK_DEVICE_TREE 查找 point 信息补全字段
      var gwId = 0, freq = 0, devId = '', shedNo = '', funcType = 0;
      MOCK_DEVICE_TREE.forEach(function (gw) {
        (gw.collectors || []).forEach(function (cl) {
          (cl.points || []).forEach(function (pt) {
            if (String(pt.point_id) === pointId) {
              gwId = gw.gateway_id;
              freq = cl.frequency;
              devId = cl.device_external_id;
              shedNo = pt.shed_no || cl.shed_no || '';
              funcType = pt.function_type;
            }
          });
        });
      });

      var nb = {
        bind_id: nextBindId++,
        user_id: parseInt(userId),
        point_id: parseInt(pointId),
        gateway_id: gwId,
        shed_no: shedNo,
        freq: freq,
        devid: devId,
        func: funcType,
        bind_usr_name: ''
      };
      MOCK_BINDINGS.push(nb);
      return { code: 0, data: { bind_id: String(nb.bind_id), user_id: userId, point_id: pointId, created_at: new Date().toISOString() } };
    },
    'DELETE /api/v1/admin/bindings/:id': function (params) {
      const idx = MOCK_BINDINGS.findIndex(b => b.bind_id === parseInt(params.id));
      if (idx < 0) return { code: 404, message: 'binding not found' };
      MOCK_BINDINGS.splice(idx, 1);
      return { data: { deleted: true } };
    },
    'GET /api/v1/admin/control-logs': function () {
      return { data: MOCK_CONTROL_LOGS };
    },
    // ---- 农户接口 ----
    'GET /api/v1/admin/farmers': function (params, query) {
      let farmers = [...MOCK_FARMERS];
      // 关键词搜索
      if (query && query.keyword) {
        const kw = query.keyword.toLowerCase();
        farmers = farmers.filter(f =>
          f.display_name.toLowerCase().includes(kw) ||
          f.mobile.toLowerCase().includes(kw) ||
          f.wechat_openid.toLowerCase().includes(kw)
        );
      }
      return { data: farmers };
    },
    'GET /api/v1/admin/farmers/:id': function (params) {
      const farmer = MOCK_FARMERS.find(f => f.user_id === parseInt(params.id));
      return farmer ? { data: farmer } : { code: 404, message: 'farmer not found' };
    },
    'PATCH /api/v1/admin/farmers/:id': function (params, body) {
      const idx = MOCK_FARMERS.findIndex(f => f.user_id === parseInt(params.id));
      if (idx < 0) return { code: 404, message: 'farmer not found' };
      const allowed = ['display_name', 'mobile', 'address', 'remark', 'status'];
      allowed.forEach(k => { if (body[k] !== undefined) MOCK_FARMERS[idx][k] = body[k]; });
      if (body.status !== undefined) {
        MOCK_FARMERS[idx].status_label = { 1: '正常', 2: '已禁用', 3: '已删除' }[body.status] || '';
      }
      return { data: MOCK_FARMERS[idx] };
    },
    'GET /api/v1/admin/farmers/:id/bindings': function (params) {
      const uid = parseInt(params.id);
      const bindings = MOCK_BINDINGS.filter(b => b.user_id === uid);
      return { data: bindings.map(b => {
        const c = MOCK_COLLECTORS.find(x => x.gateway_id === b.gateway_id && x.freq === b.freq && x.devid === b.devid);
        return { ...b, collector: c || null };
      }) };
    },
    // ---- 设备树 ----
    'GET /api/v1/admin/device-points': function (params, query) {
      var keyword = (query.keyword || '').toLowerCase();
      var bindStatus = query.bind_status || '';
      var results = [];
      var funcNames = { 1: '温度', 2: '墒情', 3: '限位', 4: '水泵', 5: '风机', 6: '继电器' };

      MOCK_DEVICE_TREE.forEach(function (gw) {
        (gw.collectors || []).forEach(function (cl) {
          (cl.points || []).forEach(function (pt) {
            // 判断是否已绑定
            var boundBinding = MOCK_BINDINGS.find(function (b) {
              return String(b.point_id) === String(pt.point_id);
            });

            var pointName = pt.display_name || pt.point_name || (funcNames[pt.function_type] + '传感器');
            var shedNo = pt.shed_no || cl.shed_no || '';
            var searchStr = (pointName + ' ' + gw.gateway_name + ' ' + cl.frequency + ' ' + cl.device_external_id + ' ' + shedNo + ' ' + funcNames[pt.function_type]).toLowerCase();

            if (keyword && searchStr.indexOf(keyword) === -1) return;

            var isBound = !!boundBinding;

            // bind_status 过滤
            if (bindStatus === 'unbound' && isBound) return;
            if (bindStatus === 'bound' && !isBound) return;

            results.push({
              point_id: String(pt.point_id),
              gateway_id: String(gw.gateway_id),
              gateway_name: gw.gateway_name,
              node_id: String(cl.node_id || ''),
              frequency: cl.frequency,
              device_external_id: cl.device_external_id,
              function_type: pt.function_type,
              function_name: funcNames[pt.function_type] || '未知',
              channel_no: pt.channel_no || 1,
              point_name: pointName,
              unit: pt.unit || '',
              shed_no: shedNo,
              is_bound: isBound,
              bind_user_id: boundBinding ? String(boundBinding.user_id) : null,
              bind_user_name: boundBinding ? (boundBinding.bind_user_name || boundBinding.bind_usr_name || '') : ''
            });
          });
        });
      });

      return { code: 0, data: results };
    },
    'GET /api/v1/admin/device-tree': function () {
      return { data: MOCK_DEVICE_TREE };
    },
  };

  // ============================================================
  // 工具
  // ============================================================

  function getCurrentUserId() { return currentUserId; }
  function setCurrentUserId(uid) { currentUserId = uid; }

  function matchMockRoute(method, cleanPath) {
    for (const key of Object.keys(mockHandlers)) {
      const [m, routePath] = key.split(' ');
      if (m !== method) continue;
      const routeParts = routePath.split('/');
      const pathParts = cleanPath.split('/');
      if (routeParts.length !== pathParts.length) continue;
      const params = {};
      let match = true;
      for (let i = 0; i < routeParts.length; i++) {
        if (routeParts[i].startsWith(':')) { params[routeParts[i].slice(1)] = pathParts[i]; }
        else if (routeParts[i] !== pathParts[i]) { match = false; break; }
      }
      if (match) return { key, params };
    }
    return null;
  }

  function parseQuery(path) {
    const qsIdx = path.indexOf('?');
    if (qsIdx < 0) return { cleanPath: path, query: {} };
    const query = {};
    path.substring(qsIdx + 1).split('&').forEach(p => {
      const [k, v] = p.split('=');
      if (k) query[k] = decodeURIComponent(v || '');
    });
    return { cleanPath: path.substring(0, qsIdx), query };
  }

  /**
   * 判断路径是否为需要 admin token 的保护接口（排除 login）。
   */
  function isProtectedAdminPath(path) {
    if (path.indexOf('/api/v1/admin/') !== 0 && path !== '/api/v1/admin') return false;
    // login 是公开接口，不走 401/403 拦截
    if (path === '/api/v1/admin/login') return false;
    return true;
  }

  // ============================================================
  // 核心请求函数
  // MODE='real'：先请求真实后端，失败则 fallback mock
  // MODE='mock'：只用 mock
  // ============================================================

  async function request(method, path, body) {
    const { cleanPath, query } = parseQuery(path);

    // mock 模式：完全不走网络
    if (MODE === 'mock') {
      return mockRequest(method, cleanPath, query, body);
    }

    // real 模式：先尝试真实后端
    try {
      const realResult = await realRequest(method, cleanPath, query, body);
      return realResult;
    } catch (e) {
      // admin 接口的 401/403 和 mini 接口的 401/403 已在 realRequest 中直接抛出，
      // 不会走到这里。走到这里的只有网络/超时/404 等错误。
      console.warn('[API] 真实后端请求失败，fallback 到 mock:', method, cleanPath, e.message);
      return mockRequest(method, cleanPath, query, body);
    }
  }

  // ---------- 真实后端请求 ----------
  const REAL_REQUEST_TIMEOUT = 5000; // 5 秒超时（CORS/网络不可达时快速 fallback）

  async function realRequest(method, cleanPath, query, body) {
    const headers = { 'Content-Type': 'application/json' };
    const token = getTokenForPath(cleanPath);
    if (token) headers['Authorization'] = 'Bearer ' + token;

    let url = API_BASE + cleanPath;
    if (method === 'GET' && Object.keys(query).length > 0) {
      url += '?' + Object.entries(query).map(([k, v]) => k + '=' + encodeURIComponent(v)).join('&');
    }

    // 用 AbortController 实现超时（fetch 本身不支持 timeout）
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), REAL_REQUEST_TIMEOUT);

    const opts = { method, headers, mode: 'cors', signal: controller.signal };
    if (body && method !== 'GET') opts.body = JSON.stringify(body);

    let res;
    try {
      res = await fetch(url, opts);
    } catch (fetchErr) {
      clearTimeout(timeoutId);
      if (fetchErr.name === 'AbortError') {
        console.warn('[API] 请求超时 (' + (REAL_REQUEST_TIMEOUT / 1000) + 's):', method, cleanPath, '→ fallback mock');
        throw new Error('请求超时，已切换本地模式');
      }
      console.warn('[API] 网络请求失败（可能 CORS 或后端不可达）:', method, cleanPath, fetchErr);
      throw new Error('网络不可用，已切换本地模式');
    }
    clearTimeout(timeoutId);

    // ─── Admin 保护接口 401/403：不允许 fallback mock，直接报错 ───
    if (isProtectedAdminPath(cleanPath)) {
      if (res.status === 401) {
        // 清除可能残留的错误 token
        localStorage.removeItem('nh_admin_token');
        throw new Error('认证失败，请使用管理员账号重新登录');
      }
      if (res.status === 403) {
        // 403 = 权限不足（如用 farmer token 访问 admin 接口）
        throw new Error('权限不足，请使用管理员账号登录');
      }
    }

    // ─── Mini 接口 401：清除 mini token ───
    if (res.status === 401) {
      localStorage.removeItem('nh_mini_token');
      throw new Error('认证失败，请重新登录');
    }

    // ─── Mini 接口 403：权限不足（如 admin token 访问 mini 接口），不 fallback mock ───
    if (res.status === 403 && (cleanPath.indexOf('/api/v1/mini/') === 0 || cleanPath === '/api/v1/mini')) {
      const errBody = await res.json().catch(() => ({}));
      throw new Error(errBody.message || '权限不足（403）');
    }

    // 404：后端未实现该接口，fallback mock
    if (res.status === 404) {
      throw new Error('接口未实现 (404)');
    }

    // 其他 HTTP 错误
    if (!res.ok) {
      const errBody = await res.json().catch(() => ({}));
      throw new Error(errBody.message || '请求失败 (' + res.status + ')');
    }

    const json = await res.json();
    return json;
  }

  // ---------- Mock 请求 ----------
  async function mockRequest(method, cleanPath, query, body) {
    await new Promise(r => setTimeout(r, MOCK_DELAY));

    const route = matchMockRoute(method, cleanPath);
    if (!route) {
      return { code: 404, message: 'mock route not found: ' + method + ' ' + cleanPath };
    }

    const handler = mockHandlers[route.key];
    if (!handler) {
      return { code: 404, message: 'mock handler not found' };
    }

    try {
      return handler(route.params, body || query);
    } catch (e) {
      return { code: 500, message: e.message };
    }
  }

  // ============================================================
  // 公开 API
  // ============================================================
  global.MockAPI = {
    get: function (path) { return request('GET', path); },
    post: function (path, body) { return request('POST', path, body); },
    patch: function (path, body) { return request('PATCH', path, body); },
    delete: function (path) { return request('DELETE', path); },

    setUserId: setCurrentUserId,
    getUserId: getCurrentUserId,

    login: async function (type, credentials) {
      if (type === 'mini') {
        return request('POST', '/api/v1/mini/mock-login', credentials || { mock_openid: 'mock_user_001', mock_nickname: '测试农户' });
      }
      return request('POST', '/api/v1/admin/login', credentials);
    },

    getMe: async function (type) {
      return request('GET', '/api/v1/' + type + '/me');
    },

    updateMyProfile: async function (data) {
      return request('PATCH', '/api/v1/mini/me', data);
    },

    getDevices: async function () {
      return request('GET', '/api/v1/mini/devices');
    },

    getDeviceRealtime: async function (id) {
      return request('GET', '/api/v1/mini/devices/' + id + '/realtime');
    },

    getDeviceHistory: async function (id, range) {
      return request('GET', '/api/v1/mini/devices/' + id + '/history?range=' + (range || '24h'));
    },

    controlDevice: async function (id, action) {
      return request('POST', '/api/v1/mini/devices/' + id + '/control', { action });
    },

    getDashboard: async function () {
      return request('GET', '/api/v1/admin/dashboard');
    },

    getUsers: async function () {
      return request('GET', '/api/v1/admin/users');
    },

    getGateways: async function () {
      // 注意：后端接口路径是 /api/v1/gateways（无 admin 前缀）
      return request('GET', '/api/v1/gateways');
    },

    getCollectors: async function () {
      return request('GET', '/api/v1/admin/collectors');
    },

    getBindings: async function () {
      return request('GET', '/api/v1/admin/bindings');
    },

    addBinding: async function (data) {
      return request('POST', '/api/v1/admin/bindings', data);
    },

    removeBinding: async function (id) {
      return request('DELETE', '/api/v1/admin/bindings/' + id);
    },

    getControlLogs: async function () {
      return request('GET', '/api/v1/admin/control-logs');
    },

    // ---- 农户管理 ----
    getFarmers: async function (keyword) {
      var path = '/api/v1/admin/farmers';
      if (keyword) path += '?keyword=' + encodeURIComponent(keyword);
      return request('GET', path);
    },

    getFarmerDetail: async function (userId) {
      return request('GET', '/api/v1/admin/farmers/' + userId);
    },

    updateFarmer: async function (userId, data) {
      return request('PATCH', '/api/v1/admin/farmers/' + userId, data);
    },

    getFarmerBindings: async function (userId) {
      return request('GET', '/api/v1/admin/farmers/' + userId + '/bindings');
    },

    // ---- 设备树 ----
    getDeviceTree: async function () {
      return request('GET', '/api/v1/admin/device-tree');
    },

    searchDevicePoints: async function (keyword, bindStatus) {
      var path = '/api/v1/admin/device-points';
      var params = [];
      if (keyword) params.push('keyword=' + encodeURIComponent(keyword));
      if (bindStatus) params.push('bind_status=' + encodeURIComponent(bindStatus));
      if (params.length) path += '?' + params.join('&');
      return request('GET', path);
    },
  };

})(window);
