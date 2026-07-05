/**
 * 涅凰智农 IoT 云平台 — H5 真实 API Client
 *
 * 生产 H5 只允许调用真实后端，不做本地数据兜底。
 */
(function (global) {
  'use strict';

  var DEFAULT_REMOTE_BASE = 'http://43.143.208.153:8080';
  var API_BASE = global.NEHUANG_API_BASE || (
    global.location && global.location.protocol !== 'file:' ? global.location.origin : DEFAULT_REMOTE_BASE
  );

  var ADMIN_TOKEN_KEY = 'nh_admin_token';
  var ADMIN_USER_KEY = 'nh_admin_user';
  var MINI_TOKEN_KEY = 'nh_mini_token';
  var MINI_USER_KEY = 'nh_mini_user';
  var REQUEST_TIMEOUT = 10000;

  function ApiError(message, status, code, data) {
    this.name = 'ApiError';
    this.message = message || '请求失败';
    this.status = status || 0;
    this.code = code || 0;
    this.data = data || null;
  }
  ApiError.prototype = Object.create(Error.prototype);
  ApiError.prototype.constructor = ApiError;

  function isAdminPath(path) {
    return path === '/api/v1/admin' || path.indexOf('/api/v1/admin/') === 0;
  }

  function isMiniPath(path) {
    return path === '/api/v1/mini' || path.indexOf('/api/v1/mini/') === 0;
  }

  function isPublicPath(path) {
    return path === '/api/v1/admin/login' || path === '/api/v1/mini/login';
  }

  function getTokenForPath(path) {
    if (isPublicPath(path)) return null;
    if (isAdminPath(path) || path === '/api/v1/gateways') {
      return localStorage.getItem(ADMIN_TOKEN_KEY) || null;
    }
    if (isMiniPath(path)) {
      return localStorage.getItem(MINI_TOKEN_KEY) || null;
    }
    return null;
  }

  function clearTokenForPath(path) {
    if (isAdminPath(path) || path === '/api/v1/gateways') {
      localStorage.removeItem(ADMIN_TOKEN_KEY);
      localStorage.removeItem(ADMIN_USER_KEY);
    }
    if (isMiniPath(path)) {
      localStorage.removeItem(MINI_TOKEN_KEY);
      localStorage.removeItem(MINI_USER_KEY);
    }
  }

  function buildURL(path) {
    if (/^https?:\/\//.test(path)) return path;
    return API_BASE.replace(/\/$/, '') + path;
  }

  function request(method, path, body) {
    var headers = { 'Content-Type': 'application/json' };
    var token = getTokenForPath(path.split('?')[0]);
    if (token) headers.Authorization = 'Bearer ' + token;

    var controller = new AbortController();
    var timeoutID = setTimeout(function () { controller.abort(); }, REQUEST_TIMEOUT);
    var opts = { method: method, headers: headers, mode: 'cors', signal: controller.signal };
    if (body != null && method !== 'GET') opts.body = JSON.stringify(body);

    return fetch(buildURL(path), opts).then(function (res) {
      clearTimeout(timeoutID);
      return res.text().then(function (text) {
        var payload = {};
        if (text) {
          try { payload = JSON.parse(text); }
          catch (e) { payload = { message: text }; }
        }

        if (res.status === 401) {
          clearTokenForPath(path);
          throw new ApiError(payload.message || '认证失败，请重新登录', res.status, payload.code, payload);
        }
        if (res.status === 403) {
          throw new ApiError(payload.message || '权限不足', res.status, payload.code, payload);
        }
        if (!res.ok) {
          throw new ApiError(payload.message || ('请求失败 (' + res.status + ')'), res.status, payload.code, payload);
        }
        return payload;
      });
    }).catch(function (err) {
      clearTimeout(timeoutID);
      if (err && err.name === 'AbortError') {
        throw new ApiError('请求超时，请检查网络或服务状态', 0, 0, null);
      }
      if (err instanceof ApiError) throw err;
      throw new ApiError(err && err.message ? err.message : '网络异常，请稍后重试', 0, 0, null);
    });
  }

  global.NHApi = {
    keys: {
      adminToken: ADMIN_TOKEN_KEY,
      adminUser: ADMIN_USER_KEY,
      miniToken: MINI_TOKEN_KEY,
      miniUser: MINI_USER_KEY
    },
    ApiError: ApiError,
    get: function (path) { return request('GET', path); },
    post: function (path, body) { return request('POST', path, body); },
    patch: function (path, body) { return request('PATCH', path, body); },
    delete: function (path) { return request('DELETE', path); },
    loginAdmin: function (credentials) {
      return request('POST', '/api/v1/admin/login', credentials);
    },
    loginMini: function (code, profile) {
      var body = Object.assign({ code: code }, profile || {});
      return request('POST', '/api/v1/mini/login', body);
    },
    getMe: function (type) {
      return request('GET', '/api/v1/' + type + '/me');
    },
    updateMyProfile: function (data) {
      return request('PATCH', '/api/v1/mini/me', data);
    },
    getDevices: function () {
      return request('GET', '/api/v1/mini/devices');
    },
    getDeviceRealtime: function (pointID) {
      return request('GET', '/api/v1/mini/devices/' + encodeURIComponent(pointID) + '/realtime');
    },
    getDeviceHistory: function (pointID, range) {
      return request('GET', '/api/v1/mini/devices/' + encodeURIComponent(pointID) + '/history?range=' + encodeURIComponent(range || '1d'));
    },
    controlDevice: function (pointID, action) {
      return request('POST', '/api/v1/mini/devices/' + encodeURIComponent(pointID) + '/control', { action: action });
    },
    getDashboard: function (range) {
      return request('GET', '/api/v1/admin/dashboard' + (range ? '?range=' + encodeURIComponent(range) : ''));
    },
    getUsers: function () {
      return request('GET', '/api/v1/admin/users');
    },
    getGateways: function () {
      return request('GET', '/api/v1/gateways');
    },
    getCollectors: function () {
      return request('GET', '/api/v1/admin/collectors');
    },
    getBindings: function () {
      return request('GET', '/api/v1/admin/bindings');
    },
    addBinding: function (data) {
      return request('POST', '/api/v1/admin/bindings', data);
    },
    removeBinding: function (id) {
      return request('DELETE', '/api/v1/admin/bindings/' + encodeURIComponent(id));
    },
    getControlLogs: function () {
      return request('GET', '/api/v1/admin/control-logs');
    },
    getOperationLogs: function () {
      return request('GET', '/api/v1/admin/operation-logs');
    },
    getFarmers: function (keyword) {
      return request('GET', '/api/v1/admin/farmers' + (keyword ? '?keyword=' + encodeURIComponent(keyword) : ''));
    },
    getFarmerDetail: function (userID) {
      return request('GET', '/api/v1/admin/farmers/' + encodeURIComponent(userID));
    },
    updateFarmer: function (userID, data) {
      return request('PATCH', '/api/v1/admin/farmers/' + encodeURIComponent(userID), data);
    },
    getFarmerBindings: function (userID) {
      return request('GET', '/api/v1/admin/farmers/' + encodeURIComponent(userID) + '/bindings');
    },
    getDeviceTree: function () {
      return request('GET', '/api/v1/admin/device-tree');
    },
    searchDevicePoints: function (keyword, bindStatus) {
      var params = [];
      if (keyword) params.push('keyword=' + encodeURIComponent(keyword));
      if (bindStatus) params.push('bind_status=' + encodeURIComponent(bindStatus));
      return request('GET', '/api/v1/admin/device-points' + (params.length ? '?' + params.join('&') : ''));
    },
    updateDevicePoint: function (pointID, data) {
      return request('PATCH', '/api/v1/admin/device-points/' + encodeURIComponent(pointID), data);
    },
    updateGateway: function (gatewayID, data) {
      return request('PATCH', '/api/v1/admin/gateways/' + encodeURIComponent(gatewayID), data);
    },
    createGatewayCollector: function (gatewayID, data) {
      return request('POST', '/api/v1/admin/gateways/' + encodeURIComponent(gatewayID) + '/collectors', data);
    },
    seedGatewayPoints: function (gatewayID, data) {
      return request('POST', '/api/v1/admin/gateways/' + encodeURIComponent(gatewayID) + '/collectors', data);
    },
    getPayloadEvents: function (params) {
      var qs = [];
      params = params || {};
      Object.keys(params).forEach(function (key) {
        if (params[key] != null && params[key] !== '') {
          qs.push(key + '=' + encodeURIComponent(params[key]));
        }
      });
      return request('GET', '/api/v1/admin/payload-events' + (qs.length ? '?' + qs.join('&') : ''));
    },
    getPayloadEventDetail: function (id) {
      return request('GET', '/api/v1/admin/payload-events/' + encodeURIComponent(id));
    }
  };
})(window);
