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
  var bindingDialogState = null;

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

  // ---------- 统一术语与兼容映射 ----------
  function getFunctionTypeName(v) {
    var type = parseInt(v);
    if (type === 1) return '温度';
    if (type === 2) return '墒情';
    if (type === 3) return '限位';
    if (type === 4) return '继电器';
    if (type === 5) return '水泵';
    if (type === 6) return '风机';
    return '功能点';
  }

  function toText(v) {
    if (v === null || v === undefined) return '';
    return String(v);
  }

  function toNum(v) {
    var n = parseInt(v, 10);
    return isNaN(n) ? null : n;
  }

  function toFloat(v) {
    if (v === null || v === undefined || v === '') return null;
    var n = parseFloat(toText(v));
    return isNaN(n) ? null : n;
  }

  function normalizeHexText(hex) {
    var raw = toText(hex).trim();
    if (!raw) return '';
    return raw.replace(/^0x/i, '').replace(/[^0-9a-fA-F]/g, '').toUpperCase();
  }

  function formatHexDisplay(hex) {
    var clean = normalizeHexText(hex);
    if (!clean) return '';
    var parts = [];
    for (var i = 0; i < clean.length; i += 2) {
      parts.push(clean.slice(i, i + 2));
    }
    return parts.join(' ');
  }

  function hexToAscii(hex) {
    var clean = normalizeHexText(hex);
    if (!clean) return '';
    var out = [];
    for (var i = 0; i + 1 < clean.length; i += 2) {
      var byte = parseInt(clean.slice(i, i + 2), 16);
      if (isNaN(byte)) continue;
      if (byte >= 32 && byte <= 126) out.push(String.fromCharCode(byte));
      else if (byte === 10) out.push('\\n');
      else if (byte === 13) out.push('\\r');
      else if (byte === 9) out.push('\\t');
      else out.push('.');
    }
    return out.join('');
  }

  function normalizeProtocolSummary(summary) {
    if (!summary) return null;
    if (typeof summary === 'string') {
      try {
        summary = JSON.parse(summary);
      } catch (e) {
        return { rawText: summary };
      }
    }
    if (typeof summary !== 'object') return null;

    var fields = [];
    if (Array.isArray(summary.fields)) {
      fields = summary.fields.map(function (field) {
        return {
          name: toText(field.name || ''),
          label: toText(field.label || field.name || ''),
          value: toText(field.value || '')
        };
      });
    }

    return {
      kind: toText(summary.kind || ''),
      label: toText(summary.label || ''),
      description: toText(summary.description || ''),
      ascii: toText(summary.ascii || ''),
      crcValid: summary.crc_valid === true ? true : summary.crc_valid === false ? false : null,
      fields: fields,
      rawText: toText(summary.rawText || '')
    };
  }

  function getGatewayId(gateway) {
    if (!gateway) return '';
    return toText(gateway.gateway_id || gateway.gatewayId || gateway.gatewayIdStr || gateway.gateway_external_id || gateway.gatewayId || gateway.id || gateway.ID || gateway.gateway_id_str || '').trim();
  }

  function getGatewayName(gateway) {
    if (!gateway) return '';
    return toText(gateway.gateway_name || gateway.dev_name || gateway.name || '').trim();
  }

  function normalizeGateway(gateway) {
    return {
      gatewayId: getGatewayId(gateway),
      gatewayName: getGatewayName(gateway),
      ip: toText(gateway.ip || gateway.host || ''),
      port: gateway.port != null ? gateway.port : '',
      status: toNum(gateway.status) || 0,
      online: gateway.online === true || gateway.online === 1 || gateway.online === '1',
      collectors: gateway.collectors || []
    };
  }

  function normalizeBindingType(binding) {
    if (!binding) return 'point';
    var raw = (binding.binding_type || binding.bindingType || binding.bind_type || '').toLowerCase();
    if (raw === 'gateway' || raw === 'point') return raw;
    if (binding.point_id || binding.node_id || binding.function_type || binding.frequency || binding.device_external_id || binding.func || binding.bound_to_point) return 'point';
    return 'gateway';
  }

  function bindingTypeLabel(bindingType) {
    return bindingType === 'gateway' ? '设备网关' : '功能点';
  }

  function resolvePayloadEventCategory(item) {
    var rawLabel = toText(item.event_type_label || item.event_type_name || item.event_name || item.eventLabel || item.type_name || item.type).trim();
    var lower = rawLabel.toLowerCase();
    if (lower === 'register' || lower === 'registered' || lower === 'reg' || lower === '注册') return 'register';
    if (lower === 'heartbeat' || lower === 'heart' || lower === '心跳') return 'heartbeat';
    if (lower === 'work' || lower === 'telemetry' || lower === 'work_data' || lower === 'workdata' || lower === '遥测' || lower === '工作数据') return 'work';
    if (lower === 'alarm' || lower === 'warn' || lower === 'warning' || lower === '告警') return 'alarm';
    if (lower === 'unknown' || lower === '未知包' || lower === '未识别') return 'unknown';
    if (lower === '控制结果' || lower === 'json快照') return 'unknown';

    var backendType = toText(item.event_type || item.type).toLowerCase();
    if (backendType === 'register' || backendType === 'heartbeat' || backendType === 'work' || backendType === 'alarm' || backendType === 'unknown') return backendType;

    var t = parseInt(item.event_type);
    if (t === 1) return 'register';
    if (t === 2) return 'heartbeat';
    if (t === 3) return 'work';
    if (t === 4 || t === 5 || t === 7) return 'unknown';
    if (t === 6) return 'alarm';
    return 'unknown';
  }

  function resolvePayloadEventName(item) {
    return payloadCategoryLabel(resolvePayloadEventCategory(item));
  }

  function getPayloadEventDisplayLabel(item) {
    return toText(item.event_type_name || item.event_type_label || item.eventLabel || item.event_label || item.type_name || item.type || '').trim() ||
      payloadCategoryLabel(resolvePayloadEventCategory(item));
  }

  function payloadCategoryLabel(category) {
    if (category === 'register') return '注册';
    if (category === 'heartbeat') return '心跳';
    if (category === 'work') return '工作数据';
    if (category === 'alarm') return '告警';
    if (category === 'unknown') return '未知包';
    return '未知包';
  }

  function payloadCategoryClass(category) {
    if (category === 'register') return 'badge-info';
    if (category === 'heartbeat') return 'badge-muted';
    if (category === 'work') return 'badge-success';
    if (category === 'alarm') return 'badge-danger';
    return 'badge-warning';
  }

  function payloadTypeToCategoryFilter(value) {
    if (value === 'register') return 1;
    if (value === 'heartbeat') return 2;
    if (value === 'work') return 3;
    if (value === 'alarm') return 6;
    if (value === 'unknown') return 7;
    return null;
  }

  function getPayloadParseStatusClass(status) {
    if (status === 1) return 'badge-warning';
    if (status === 2) return 'badge-success';
    if (status === 3) return 'badge-danger';
    if (status === 4) return 'badge-info';
    if (status === 5) return 'badge-muted';
    return 'badge-muted';
  }

  function getPayloadParseStatusLabel(status, label) {
    if (label && label !== '--') return label;
    if (status === 1) return '待解析';
    if (status === 2) return '已解析';
    if (status === 3) return '解析失败';
    if (status === 4) return '部分解析';
    if (status === 5) return '已忽略';
    return '--';
  }

  function resolvePayloadFilterValue(category) {
    if (!category) return '';
    return category;
  }

  function normalizePayloadEventFromResponse(item) {
    var category = resolvePayloadEventCategory(item);
    return {
      raw: item,
      rawEventId: toText(item.raw_event_id || item.id || ''),
      eventCategory: category,
      eventName: getPayloadEventDisplayLabel(item),
      eventTypeBadgeClass: payloadCategoryClass(category),
      eventTypeLabel: payloadCategoryLabel(category),
      gatewayName: toText(item.gateway_name || item.gateway || '--'),
      sourceName: toText(item.source_type_name || item.source || '--'),
      isMock: !!item.is_mock,
      parseStatusName: toText(item.parse_status_name || item.status_name || '--'),
      parseStatus: toNum(item.parse_status),
      parseError: toText(item.parse_error || item.error || ''),
      parseStatusLabel: toText(item.parse_status_name || item.parseStatus || ''),
      messageId: toText(item.message_id || ''),
      remoteIp: toText(item.remote_ip || ''),
      remotePort: toText(item.remote_port || ''),
      payloadHex: toText(item.payload_hex || item.payloadHex || ''),
      payloadLen: toNum(item.payload_len),
      payloadByteLen: toNum(item.payload_byte_len || item.payloadByteLen || item.payload_len),
      protocolSummary: normalizeProtocolSummary(item.protocol_summary || item.protocolSummary || null),
      eventCount: toNum(item.event_count),
      parsedCount: toNum(item.parsed_count),
      receivedAt: item.received_at_ms || item.received_at || item.created_at || ''
    };
  }

  function normalizeCommandStatusLabel(statusValue, labelValue) {
    var label = toText(labelValue || '').trim();
    if (label && label !== '--') return label;
    var raw = toText(statusValue || '').trim();
    var n = parseInt(raw, 10);
    if (n === 1) return 'pending';
    if (n === 2) return 'sent';
    if (n === 3) return 'ack';
    if (n === 4) return 'succeeded';
    if (n === 5) return 'failed';
    if (n === 6) return 'timeout';
    if (raw === 'succeeded' || raw === 'failed' || raw === 'timeout' || raw === 'pending' || raw === 'sent' || raw === 'ack') return raw;
    return raw || '--';
  }

  function normalizeControlLogRecord(item) {
    if (!item) return {};
    var statusText = normalizeCommandStatusLabel(item.cmd_status || item.status || item.command_status || item.commandStatus, item.cmd_status_label || item.status_label || item.statusLabel);
    var targetPointId = toText(item.point_id || item.pointId || item.target_point_id || item.targetPointId || '');
    var targetPointName = toText(item.point_name || item.pointName || item.func_name || item.function_name || '');
    var gatewaySummary = [toText(item.gateway_name || item.gatewayName || ''), toText(item.freq || item.frequency || ''), toText(item.devid || item.device_external_id || '')]
      .filter(function (part) { return part !== ''; })
      .join(' / ');
    return {
      raw: item,
      commandId: toText(item.command_id || item.commandId || ''),
      opId: toText(item.op_id || item.opId || ''),
      gatewayId: toText(item.gateway_id || item.gatewayId || ''),
      gatewayName: toText(item.gateway_name || item.gatewayName || ''),
      nodeId: toText(item.node_id || item.nodeId || ''),
      pointId: targetPointId,
      pointName: targetPointName,
      targetSummary: targetPointName || targetPointId || gatewaySummary || '--',
      bindUserName: toText(item.bind_usr_name || item.bindUserName || ''),
      requestedBy: toText(item.requested_by || item.requestedBy || ''),
      sourceLabel: toText(item.command_source_label || item.commandSourceLabel || item.source_label || ''),
      frequency: toText(item.freq || item.frequency || ''),
      devid: toText(item.devid || item.device_external_id || ''),
      funcName: toText(item.func_name || item.function_name || ''),
      opType: toText(item.op_type || ''),
      runMode: toText(item.run_mode || ''),
      cmdStatus: toText(item.cmd_status || item.status || ''),
      cmdStatusLabel: statusText || '--',
      commandHex: normalizeHexText(item.command_hex || item.commandHex || ''),
      result: toText(item.result || ''),
      gatewayResponse: toText(item.gateway_response || item.gatewayResponse || ''),
      errorMessage: toText(item.error_message || item.errorMessage || ''),
      opTime: item.op_time || item.opTime || '',
      sentAt: item.sent_at_ms || item.sentAtMS || item.sent_at || '',
      finishedAt: item.finished_at_ms || item.finishedAtMS || item.finished_at || ''
    };
  }

  function payloadFilterSupportsServer(category) {
    return category === 'register' || category === 'heartbeat' || category === 'work' || category === 'alarm' || category === 'unknown';
  }

  function toBindingPointCandidateId(point) {
    return toText(point.point_id || point.pointId || point.id || point.ID || '').trim();
  }

  function normalizeGatewayBindingFallbackPoint(point) {
    return {
      pointId: toBindingPointCandidateId(point),
      isBound: !!(point.is_bound || point.isBound || point.bind_user_id || point.bind_user_name)
    };
  }

  function getUnboundPointIdFromGatewaysTree(tree, gatewayId) {
    if (!tree) return '';
    var targetGatewayId = toText(gatewayId);
    for (var i = 0; i < tree.length; i++) {
      if (getGatewayId(tree[i]) !== targetGatewayId) continue;

      var collectors = tree[i].collectors || tree[i].collector_list || [];
      for (var c = 0; c < collectors.length; c++) {
        var points = collectors[c].points || [];
        for (var p = 0; p < points.length; p++) {
          var normalized = normalizeGatewayBindingFallbackPoint(points[p]);
          if (!normalized.isBound && normalized.pointId) return normalized.pointId;
        }
      }

      if (Array.isArray(tree[i].points)) {
        for (var p2 = 0; p2 < tree[i].points.length; p2++) {
          var fallback = normalizeGatewayBindingFallbackPoint(tree[i].points[p2]);
          if (!fallback.isBound && fallback.pointId) return fallback.pointId;
        }
      }
    }

    return '';
  }

  function isApiStatusError(err, statusCode) {
    return !!(err && (err.status === statusCode || err.code === statusCode));
  }

  function normalizeGatewayForDisplay(gateway) {
    if (!gateway) return {};
    return {
      gatewayId: toText(gateway.gateway_id || gateway.gatewayId || gateway.id || gateway.ID || gateway.gateway_id_str || '').trim(),
      gatewayName: toText(gateway.gateway_name || gateway.dev_name || gateway.name || '').trim(),
      ip: toText(gateway.ip || gateway.host || ''),
      port: toText(gateway.port || ''),
      online: gateway.online === true || gateway.online === 1 || gateway.online === '1',
      note: toText(gateway.remark || gateway.note || ''),
      collectors: gateway.collectors || [],
      collectorCount: toNum(gateway.collector_count || gateway.collectorCount),
      pointCount: toNum(gateway.point_count || gateway.pointCount)
    };
  }

  function normalizeCollectorForDisplay(collector) {
    if (!collector) return {};
    return {
      collectorId: toText(collector.collector_id || collector.collectorId || collector.id || collector.node_id || collector.nodeId || '').trim(),
      collectorName: toText(collector.collector_name || collector.name || collector.devid || collector.device_name || '').trim(),
      shedNo: toText(collector.shed_no || collector.shedNo || '').trim(),
      frequency: toNum(collector.frequency),
      deviceExternalId: toText(collector.device_external_id || collector.devid || '').trim(),
      statusLabel: toText(collector.status_label || collector.statusLabel || ''),
      points: (collector.points || []).map(normalizePointForDisplay)
    };
  }

  function normalizeBindingSubmitState(state) {
    if (!state || !state.userId) return null;
    if (state.bindingType === 'gateway') {
      if (!state.selectedGatewayId) return null;
      var payload = { binding_type: 'gateway', user_id: state.userId, gateway_id: state.selectedGatewayId };
      return payload;
    }
    if (!state.selectedPointId) return null;
    return { binding_type: 'point', user_id: state.userId, point_id: state.selectedPointId };
  }

  function normalizePointPayloadFromBindingState(state, pointId) {
    if (!state || !state.userId || !pointId) return null;
    return { binding_type: 'point', user_id: state.userId, point_id: toText(pointId) };
  }

  function isBindingResponseSuccess(res) {
    return !!(res && (res.data || res.code === 200 || res.code === 201 || res.success === true));
  }

  function isGatewayBindingSubmitError(err) {
    if (!err) return false;
    if (isApiStatusError(err, 409) || isApiStatusError(err, 404) || isApiStatusError(err, 401) || isApiStatusError(err, 403)) return false;
    var message = toText(err.message || (err.data && err.data.message));
    var lower = message.toLowerCase();
    return lower.indexOf('point_id') !== -1 || lower.indexOf('point id') !== -1 || lower.indexOf('绑定') !== -1 || lower.indexOf('binding_type') !== -1;
  }

  function buildGatewayBindFallbackCandidates(pointCandidates) {
    var list = [];
    if (Array.isArray(pointCandidates)) {
      for (var i = 0; i < pointCandidates.length; i++) {
        if (!pointCandidates[i]) continue;
        var raw = pointCandidates[i];
        var pid = toBindingPointCandidateId(raw);
        if (!pid) continue;
        list.push({
          pointId: pid,
          gatewayId: toText(raw.gateway_id || raw.gatewayId || raw.gateway_id_str || ''),
          isBound: !!(raw.is_bound || raw.isBound || raw.bind_user_id || raw.bind_user_name)
        });
      }
    }
    return list;
  }

  async function findUnboundPointIdFromGatewayForBinding(gatewayId) {
    var points = [];
    try {
      var treeRes = await NHApi.getDeviceTree();
      var gateways = [];
      if (Array.isArray(treeRes && treeRes.data)) gateways = treeRes.data;
      if (Array.isArray(treeRes && treeRes.data && treeRes.data.items)) gateways = treeRes.data.items;
      var found = getUnboundPointIdFromGatewaysTree(gateways, gatewayId);
      if (found) return found;
    } catch (err) {}

    try {
      var pointRes = await NHApi.searchDevicePoints('', 'unbound');
      points = buildGatewayBindFallbackCandidates(pointRes && pointRes.data);
      for (var i = 0; i < points.length; i++) {
        if (points[i].gatewayId && points[i].gatewayId === toText(gatewayId) && !points[i].isBound) {
          return points[i].pointId;
        }
      }
    } catch (err) {}

    return '';
  }

  function normalizeGatewayCandidateForPoint(point) {
    return {
      pointId: point.pointId,
      pointName: point.pointName,
      gatewayId: point.gatewayId,
      gatewayName: point.gatewayName,
      shedNo: point.shedNo,
      frequency: point.frequency,
      deviceExternalId: point.deviceExternalId,
      functionType: point.functionType,
      functionName: getFunctionTypeName(point.functionType),
      unit: point.unit,
      isBound: point.isBound,
      raw: point.raw
    };
  }

  function normalizeCollectorForDisplayCollector(gatewayId, collector) {
    if (!collector) return {};
    var normalized = normalizeCollectorForDisplay(collector);
    if (!normalized.gatewayId) normalized.gatewayId = gatewayId;
    return normalized;
  }

  function normalizeGatewayForTree(gateway) {
    var normalized = normalizeGatewayForDisplay(gateway);
    var collectors = (gateway.collectors || gateway.collector_list || []).map(function (c) {
      return normalizeCollectorForDisplayCollector(normalized.gatewayId, c);
    });
    normalized.collectors = collectors;
    normalized.collectorCount = collectors.length;
    normalized.pointCount = collectors.reduce(function (acc, c) {
      return acc + (Array.isArray(c.points) ? c.points.length : 0);
    }, 0);
    return normalized;
  }

  function normalizeCandidateGateway(item) {
    var normalized = normalizeGatewayForDisplay(item);
    var points = [];
    if (Array.isArray(item.points)) {
      points = item.points.map(normalizePointForDisplay);
    } else if (Array.isArray(item.collectors)) {
      item.collectors.forEach(function (c) {
        var collector = normalizeCollectorForDisplay(c);
        (collector.points || []).forEach(function (p) { points.push(p); });
      });
    }
    return {
      candidateType: 'gateway',
      gatewayId: normalized.gatewayId,
      gatewayName: normalized.gatewayName,
      ip: normalized.ip,
      port: normalized.port,
      online: normalized.online,
      collectorCount: normalized.collectorCount,
      pointCount: normalized.pointCount,
      note: normalized.note,
      points: points,
      raw: item
    };
  }

  function normalizeBindingCandidatePoint(point) {
    var normalized = normalizePointForDisplay(point);
    return {
      candidateType: 'point',
      pointId: normalized.pointId,
      pointName: normalized.pointName,
      gatewayId: normalized.gatewayId,
      gatewayName: normalized.gatewayName,
      shedNo: normalized.shedNo,
      frequency: normalized.frequency,
      deviceExternalId: normalized.deviceExternalId,
      functionType: normalized.functionType,
      unit: normalized.unit,
      remark: normalized.remark,
      raw: point
    };
  }

  function getGatewayCandidateListFromResponse(res) {
    if (!res) return [];
    if (Array.isArray(res.data)) return res.data.map(normalizeCandidateGateway);
    if (res.data && Array.isArray(res.data.items)) return res.data.items.map(normalizeCandidateGateway);
    if (Array.isArray(res.items)) return res.items.map(normalizeCandidateGateway);
    if (Array.isArray(res)) return res.map(normalizeCandidateGateway);
    return [];
  }

  function getPointCandidateListFromResponse(res) {
    if (!res) return [];
    if (Array.isArray(res.data)) return res.data.map(normalizeBindingCandidatePoint);
    if (res.data && Array.isArray(res.data.items)) return res.data.items.map(normalizeBindingCandidatePoint);
    if (Array.isArray(res.items)) return res.items.map(normalizeBindingCandidatePoint);
    if (Array.isArray(res)) return res.map(normalizeBindingCandidatePoint);
    return [];
  }

  function normalizePointForDisplay(point) {
    if (!point) return {};
    return {
      pointId: toText(point.point_id || point.pointId || ''),
      pointName: toText(point.point_name || point.function_name || point.name || point.func_name || '').trim(),
      gatewayId: toText(point.gateway_id || point.gatewayId || ''),
      gatewayName: toText(point.gateway_name || point.gatewayName || '').trim(),
      shedNo: toText(point.shed_no || point.shedNo || point.shed || '').trim(),
      nodeId: toText(point.node_id || point.nodeId || ''),
      frequency: toNum(point.frequency),
      bestMin: toFloat(point.best_min || point.bestMin),
      bestMax: toFloat(point.best_max || point.bestMax),
      runMode: toNum(point.run_mode || point.runMode),
      controllableStatus: toNum(point.controllable_status || point.controllableStatus),
      pollInterval: toNum(point.poll_interval || point.pollInterval),
      pollCmd: toText(point.poll_cmd || point.pollCmd || ''),
      pointStatus: toNum(point.point_status || point.pointStatus),
      deviceExternalId: toText(point.device_external_id || point.devid || '').trim(),
      functionType: toNum(point.function_type || point.functionType || point.func || ''),
      unit: toText(point.unit || '').trim(),
      remark: toText(point.remark || '').trim(),
      isBound: !!(point.is_bound || point.isBound || point.bind_user_id || point.bind_user_name)
    };
  }

  function normalizeBindingRecord(binding) {
    var bindingType = normalizeBindingType(binding);
    return {
      bindingType: bindingType,
      bindingTypeLabel: bindingTypeLabel(bindingType),
      bindId: toText(binding.bind_id || binding.bindId || ''),
      userId: toText(binding.user_id || binding.userId || ''),
      userName: toText(binding.bind_user_name || binding.bind_usr_name || binding.user_name || '--'),
      bindStatus: toNum(binding.bind_status || binding.bindStatus || 1),
      createdAt: binding.created_at || binding.createdAt || binding.created_at_ms || binding.createdAtMs,
      gatewayId: getGatewayId(binding),
      pointId: toText(binding.point_id || binding.pointId || ''),
      nodeId: toText(binding.node_id || binding.nodeId || ''),
      gatewayName: getGatewayName(binding),
      frequency: toNum(binding.frequency || binding.freq || binding.frequencyHz),
      deviceExternalId: toText(binding.device_external_id || binding.devid || ''),
      functionType: toNum(binding.function_type || binding.functionType || binding.func || ''),
      statusLabel: toText(binding.status_label || binding.statusLabel || ''),
      shedNo: toText(binding.shed_no || binding.shedNo || ''),
      pointName: toText(binding.point_name || binding.func_name || ''),
      unit: toText(binding.unit || ''),
      bindUserName: toText(binding.bind_user_name || binding.bind_usr_name || binding.bindUserName || ''),
      raw: binding
    };
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
      case 'payload-events': renderPayloadEvents(container); updatePageTitle('原始报文排查'); break;
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
  var currentDashboardRange = '1d';
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
        '1d': '1 天',
        '3d': '3 天',
        '7d': '7 天',
        '30d': '30 天',
      };
      var ranges = ['1d', '3d', '7d', '30d'];
      var rangeBtns = ranges.map(function (r) {
        var active = r === (d.range || '1d') ? ' btn-primary' : ' btn-outline';
        return '<button class="btn btn-sm' + active + '" onclick="switchDashboardRange(\'' + r + '\')">' + (rangeLabels[r] || r) + '</button>';
      }).join('');

      var html = '<div class="page-header" style="display:flex;align-items:center;justify-content:space-between">' +
        '<h2>Dashboard</h2>' +
        '<div style="display:flex;gap:6px">' + rangeBtns + '</div>' +
      '</div>' +
      '<div class="stat-grid">' +
        '<div class="stat-card"><div class="stat-card-label">在线设备网关</div><div class="stat-card-value">' + (devices.gateway_online || 0) + '<span style="font-size:16px;font-weight:400;color:var(--color-text-muted)"> / ' + (devices.gateway_total || 0) + '</span></div></div>' +
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
        '<div class="stat-card"><div class="stat-card-label">注册 / 遥测</div><div class="stat-card-value">' + (ingest.register_count || 0) + ' / ' + (ingest.telemetry_count || 0) + '</div><div class="stat-card-sub">原始报文 ' + (ingest.raw_event_count || 0) + '</div></div>' +
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
          '<thead><tr><th>ID</th><th>姓名</th><th>手机号</th><th>地址</th><th>微信OpenID</th><th>绑定关系</th><th>状态</th><th>创建时间</th><th>操作</th></tr></thead>' +
          '<tbody id="user-tbody">' +
            farmers.map(function (u) {
              var statusBadge = u.status === 1 ? 'badge-success' : u.status === 2 ? 'badge-warning' : 'badge-muted';
              return '<tr>' +
                '<td>' + u.user_id + '</td>' +
                '<td>' + escHtml(u.display_name) + '</td>' +
                '<td>' + escHtml(u.mobile || '--') + '</td>' +
                '<td>' + escHtml(u.address || '--') + '</td>' +
                '<td><span style="font-size:11px;word-break:break-all">' + escHtml(u.wechat_openid || '--') + '</span></td>' +
                '<td>' + (u.binding_count || 0) + ' 条</td>' +
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
        '<td>' + (u.binding_count || 0) + ' 条</td>' +
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
      var bindings = (bindingsRes.data || []).map(normalizeBindingRecord);

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
          '<button class="btn btn-primary btn-sm" id="btn-bind-device" onclick="openBindDeviceModal(\'' + userId + '\')">+ 绑定网关</button>' +
        '</div>' +
        '<div class="table-wrap"><table><thead><tr>' +
          '<th>绑定类型</th><th>绑定对象</th><th>关联功能点</th><th>网关ID</th><th>绑定时间</th><th>操作</th>' +
        '</tr></thead><tbody>';

      bindings.forEach(function (b) {
        var target = '--';
        var pointInfo = '--';
        if (b.bindingType === 'gateway') {
          target = escHtml(b.gatewayName || '--');
        } else {
          target = escHtml(b.gatewayName || '--');
          pointInfo = escHtml((b.pointName || '--') + (b.pointId ? ' (' + b.pointId + ')' : '')) + (b.frequency ? ' · 频点' + b.frequency : '');
        }

        html += '<tr>' +
          '<td><span class="badge ' + (b.bindingType === 'gateway' ? 'badge-info' : 'badge-muted') + '">' + escHtml(b.bindingTypeLabel) + '</span></td>' +
          '<td>' + target + '</td>' +
          '<td>' + pointInfo + '</td>' +
          '<td>' + escHtml((b.gatewayId || '--')) + '</td>' +
          '<td>' + formatTime(b.createdAt || b.created_at) + '</td>' +
          '<td><button class="btn btn-danger btn-sm js-remove-binding" data-bind-id="' + escHtml(b.bindId || b.bind_id || '') + '">解绑</button></td>' +
        '</tr>';
      });

      html += '</tbody></table></div></div>';

      if (bindings.length === 0) {
        html += '<div class="empty-state"><p>暂无绑定关系</p><p style="font-size:12px;color:var(--color-text-muted)">点击"绑定网关"为该农户添加绑定</p></div>';
      }

      container.innerHTML = html;
      bindRemoveButtons(container);
    } catch (e) {
      container.innerHTML = '<div class="error-state"><p>加载失败</p><button class="btn btn-outline btn-sm" onclick="renderPage()">重试</button></div>';
    }
  }

  // ---------- 绑定设备弹窗（农户详情页） ----------
  async function openBindDeviceModal(userId) {
    var overlay = $('#modal-overlay');
    overlay.style.display = 'flex';
    bindingDialogState = {
      mode: 'farmer',
      userId: toText(userId),
      bindingType: 'gateway',
      selectedGatewayId: '',
      selectedPointId: '',
      fallbackPointMode: false
    };

    overlay.innerHTML = '<div class="modal" style="max-width:700px">' +
      '<div class="modal-header">' +
        '<h3>绑定设备网关</h3>' +
        '<button class="modal-close" onclick="closeModal()">×</button>' +
      '</div>' +
      '<div class="modal-body">' +
        '<div class="form-group">' +
          '<label class="form-label">绑定类型</label>' +
          '<select class="form-select" id="bind-device-type">' +
            '<option value="gateway" selected>设备网关</option>' +
            '<option value="point">功能点（兼容旧版）</option>' +
          '</select>' +
        '</div>' +
        '<div class="form-group">' +
          '<label class="form-label">搜索候选</label>' +
          '<input type="text" class="form-input" id="bind-device-search" placeholder="输入网关名称/网关ID/DevID/功能点关键词...">' +
        '</div>' +
        '<div id="bind-device-list" class="bind-candidate-wrap" style="max-height:360px;overflow-y:auto;margin-top:12px">' +
          '<div class="loading-state"><div class="loading-spinner"></div><p>加载候选...</p></div>' +
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

    var searchTimer = null;
    var listEl = $('#bind-device-list');
    var searchEl = $('#bind-device-search');
    var typeEl = $('#bind-device-type');

    typeEl.onchange = function () {
      bindingDialogState.bindingType = this.value;
      bindingDialogState.selectedGatewayId = '';
      bindingDialogState.selectedPointId = '';
      $('#bind-device-selected').style.display = 'none';
      $('#bind-selected-name').textContent = '';
      $('#btn-confirm-bind').disabled = true;
      loadBindingCandidates(searchEl.value);
    };

    searchEl.oninput = function () {
      clearTimeout(searchTimer);
      searchTimer = setTimeout(function () {
        loadBindingCandidates(searchEl.value);
      }, 300);
    };

    await loadBindingCandidates('');

    async function loadGatewayCandidates(keyword) {
      var qs = [];
      if (keyword) qs.push('keyword=' + encodeURIComponent(keyword));
      return NHApi.get('/api/v1/admin/gateway-bind-candidates' + (qs.length ? ('?' + qs.join('&')) : ''));
    }

    async function loadBindingCandidates(keyword) {
      listEl.innerHTML = '<div class="loading-state"><div class="loading-spinner"></div><p>搜索候选...</p></div>';
      var type = bindingDialogState.bindingType;
      var candidates = [];

      if (type === 'gateway') {
        try {
          var gwRes = await loadGatewayCandidates(keyword);
          candidates = getGatewayCandidateListFromResponse(gwRes || {});
          if (candidates.length === 0) {
            listEl.innerHTML = '<div class="empty-state"><p>没有可绑定的设备网关</p><p style="font-size:12px;color:var(--color-text-muted)">请尝试切换到“功能点（兼容旧版）”继续搜索</p></div>';
            return;
          }
        } catch (e) {
          if (isApiStatusError(e, 404) || isApiStatusError(e, 405) || isApiStatusError(e, 501)) {
            type = 'point';
            bindingDialogState.bindingType = 'point';
            typeEl.value = 'point';
            if (typeEl) typeEl.className = 'form-select';
            bindingDialogState.fallbackPointMode = true;
          } else {
            listEl.innerHTML = '<div class="error-state"><p>加载候选失败，请稍后重试</p></div>';
            return;
          }
        }
      }

      if (type === 'point') {
        try {
          var pointRes = await NHApi.searchDevicePoints(keyword, 'unbound');
          candidates = getPointCandidateListFromResponse(pointRes || {});
          if (candidates.length === 0) {
            listEl.innerHTML = '<div class="empty-state"><p>没有可绑定的功能点</p></div>';
            return;
          }
        } catch (e) {
          listEl.innerHTML = '<div class="error-state"><p>加载候选失败</p></div>';
          return;
        }
      }

      var html = '';
      if (type === 'gateway') {
        candidates.forEach(function (g) {
          var isSelected = bindingDialogState.selectedGatewayId === g.gatewayId;
          html += '<div class="bind-candidate-item' + (isSelected ? ' selected' : '') + '" data-gateway-id="' + escHtml(g.gatewayId) + '" data-type="gateway">' +
            '<div style="display:flex;justify-content:space-between;align-items:center">' +
              '<div style="font-weight:600">' + escHtml(g.gatewayName || '未命名网关') + '</div>' +
              '<span class="badge ' + (g.online ? 'badge-success' : 'badge-muted') + '">' + (g.online ? '在线' : '离线') + '</span>' +
            '</div>' +
            '<div style="font-size:12px;color:var(--color-text-muted);margin-top:4px">网关ID：' + escHtml(g.gatewayId || '--') + ' · IP：' + escHtml(g.ip || '--') + (g.port ? (' : ' + escHtml(g.port)) : '') + '</div>' +
            '<div style="font-size:12px;color:var(--color-text-muted);margin-top:4px">采集执行器：' + (g.collectorCount != null ? g.collectorCount : '--') + ' · 功能点：' + (g.pointCount != null ? g.pointCount : '--') + '</div>' +
          '</div>';
        });
      } else {
        candidates.forEach(function (p) {
          var isSelected = bindingDialogState.selectedPointId === p.pointId;
          html += '<div class="bind-candidate-item' + (isSelected ? ' selected' : '') + '" data-point-id="' + escHtml(p.pointId) + '" data-type="point" data-gateway-name="' + escHtml(p.gatewayName || '') + '">' +
            '<div style="font-weight:600">' + escHtml(p.pointName || '--') + '</div>' +
            '<div style="font-size:12px;color:var(--color-text-muted);margin-top:4px">' +
              '网关：' + escHtml(p.gatewayName || '--') + (p.frequency != null ? ' · 频点：' + p.frequency : '') +
            '</div>' +
            '<div style="font-size:12px;color:var(--color-text-muted);margin-top:2px">' +
              '棚号：' + escHtml(p.shedNo || '--') + ' · DevID：' + escHtml(p.deviceExternalId || '--') +
            '</div>' +
          '</div>';
        });
      }

      listEl.innerHTML = html;

      listEl.querySelectorAll('.bind-candidate-item').forEach(function (el) {
        el.onclick = function () {
          if (this.dataset.type === 'gateway') {
            bindingDialogState.bindingType = 'gateway';
            bindingDialogState.selectedGatewayId = this.dataset.gatewayId;
            bindingDialogState.selectedPointId = '';
            var gatewayText = (this.querySelector('div') && this.querySelector('div').textContent) || '';
            $('#bind-selected-name').textContent = gatewayText + '（设备网关）';
          } else {
            bindingDialogState.bindingType = 'point';
            bindingDialogState.selectedPointId = this.dataset.pointId;
            bindingDialogState.selectedGatewayId = '';
            var selectedText = this.dataset.gatewayName;
            var firstLine = (this.querySelector('div') && this.querySelector('div').textContent) || '';
            $('#bind-selected-name').textContent = (firstLine || '') + (selectedText ? ('（' + selectedText + '）') : '');
          }

          listEl.querySelectorAll('.bind-candidate-item').forEach(function (item) {
            item.classList.remove('selected');
          });
          this.classList.add('selected');
          $('#bind-device-selected').style.display = 'block';
          $('#btn-confirm-bind').disabled = false;
        };
      });
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
  var gatewayDetailState = { gatewayId: '', keyword: '' };
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

      var flatGateways = [];
      if (Array.isArray(gwRes.data)) flatGateways = gwRes.data;
      else if (gwRes.data && Array.isArray(gwRes.data.items)) flatGateways = gwRes.data.items;
      else if (Array.isArray(gwRes.data && gwRes.data.data)) flatGateways = gwRes.data.data;

      var tree = [];
      if (Array.isArray(treeRes.data)) tree = treeRes.data;
      else if (treeRes.data && Array.isArray(treeRes.data.items)) tree = treeRes.data.items;

      var byTree = {};
      tree.forEach(function (gw) {
        byTree[getGatewayId(gw)] = normalizeGatewayForTree(gw);
      });

      var gateways = [];
      flatGateways.forEach(function (gw) {
        var base = normalizeGatewayForDisplay(gw);
        var node = byTree[base.gatewayId] || {};
        var merged = {
          gatewayId: base.gatewayId || getGatewayId(node),
          gatewayName: base.gatewayName || getGatewayName(node) || toText(gw.gateway_external_id || gw.external_id || '--'),
          ip: base.ip || toText(gw.ip),
          port: base.port || toText(gw.port),
          online: !!(base.online || node.online || gw.online || gw.online_status === 1),
          note: base.note || toText(gw.remark || gw.note || ''),
          collectors: node.collectors || [],
          collectorCount: toNum(node.collectorCount != null ? node.collectorCount : gw.collector_count),
          pointCount: toNum(node.pointCount != null ? node.pointCount : gw.point_count),
          raw: gw
        };
        if (!merged.gatewayId && base.ip) merged.gatewayId = base.gatewayId;
        gateways.push(merged);
      });
      if (gateways.length === 0 && tree.length) {
        gateways = tree.map(function (gw) {
          return normalizeGatewayForTree(gw);
        });
      }

      if (gateways.length === 0) {
        container.innerHTML = '<div class="empty-state"><p>暂无设备网关数据</p></div>';
        return;
      }

      var defaultGateway = gateways[0];
      gatewayDetailState.gatewayId = defaultGateway.gatewayId || '';

      // 预建立“功能点编辑”缓存，供详情弹窗使用
      window.__gatewayPagePointCache = {};
      window.__adminGatewayPageData = gateways;

      var html = '<div class="table-section">' +
        '<div class="table-header">' +
          '<h3>设备网关（' + gateways.length + '）</h3>' +
          '<div class="table-header-right">' +
            '<input class="table-search" id="gateway-search" style="width:240px" type="text" placeholder="搜索设备网关/网关ID/IP...">' +
          '</div>' +
        '</div>' +
        '<div class="table-wrap"><table><thead><tr>' +
          '<th>网关ID</th><th>网关名称</th><th>IP</th><th>端口</th><th>在线</th><th>采集执行器</th><th>功能点</th><th>操作</th>' +
        '</tr></thead><tbody id="gateway-list-tbody"></tbody></table></div>' +
      '</div>' +
      '<div class="table-section" style="margin-top:16px">' +
        '<div class="table-header">' +
          '<h3 id="gateway-detail-title">设备详情</h3>' +
          '<div class="table-header-right">' +
            '<button class="btn btn-primary btn-sm" onclick="openGatewaySeedModal(window.__adminSelectedGatewayId || \'\')">+ 联调点位</button>' +
            '<input class="table-search" id="gateway-point-search" style="width:240px" type="text" placeholder="搜索采集执行器/功能点...">' +
          '</div>' +
        '</div>' +
        '<div id="gateway-detail-body" style="padding:0 18px 18px"></div>' +
      '</div>';

      container.innerHTML = html;

      var listTbody = $('#gateway-list-tbody');
      var searchGateway = $('#gateway-search');
      var searchPoint = $('#gateway-point-search');

      function renderGatewayList(filter) {
        if (!listTbody) return;
        var keyword = (filter || '').trim().toLowerCase();
        var rows = gateways.filter(function (gw) {
          if (!keyword) return true;
          var rowText = [gw.gatewayId, gw.gatewayName, gw.ip, gw.port, gw.note].join(' ').toLowerCase();
          return rowText.indexOf(keyword) !== -1;
        });

        if (rows.length === 0) {
          listTbody.innerHTML = '<tr><td colspan="8" class="table-empty">未找到匹配的设备网关</td></tr>';
          return;
        }

        listTbody.innerHTML = rows.map(function (gw) {
          var isSelected = gw.gatewayId === gatewayDetailState.gatewayId;
          return '<tr class="' + (isSelected ? 'selected-gateway-row' : '') + '">' +
            '<td style="font-family:monospace;font-size:12px">' + escHtml(gw.gatewayId || '--') + '</td>' +
            '<td>' + escHtml(gw.gatewayName || '--') + '</td>' +
            '<td>' + escHtml(gw.ip || '--') + '</td>' +
            '<td>' + escHtml(toText(gw.port) || '--') + '</td>' +
            '<td><span class="badge ' + (gw.online ? 'badge-success' : 'badge-muted') + '">' + (gw.online ? '在线' : '离线') + '</span></td>' +
            '<td>' + (gw.collectorCount != null ? gw.collectorCount : '--') + '</td>' +
            '<td>' + (gw.pointCount != null ? gw.pointCount : '--') + '</td>' +
            '<td>' +
              '<button class="btn btn-outline btn-sm" data-gateway-id="' + escHtml(gw.gatewayId) + '" onclick="window.__selectGatewayFromAdmin(\'' + escHtml(gw.gatewayId) + '\')">查看</button>' +
            '</td>' +
          '</tr>';
        }).join('');
      }

      function renderGatewayDetail(gatewayId) {
        var detailWrap = $('#gateway-detail-body');
        var detailTitle = $('#gateway-detail-title');
        if (!detailWrap || !detailTitle) return;

        var gateway = null;
        for (var i = 0; i < gateways.length; i++) {
          if ((gateways[i].gatewayId || '') === gatewayId) { gateway = gateways[i]; break; }
        }
        if (!gateway) return;

        window.__adminSelectedGatewayId = gateway.gatewayId || '';
        detailTitle.textContent = (gateway.gatewayName || '未命名网关') + '（功能结构）';

        window.__gatewayPagePointCache = {};
        var keyword = (gatewayDetailState.keyword || '').trim().toLowerCase();
        var collectors = gateway.collectors || [];
        if (collectors.length === 0) {
          detailWrap.innerHTML = '<div class="empty-state"><p>该网关暂无采集执行器与功能点</p><p style="font-size:12px;color:var(--color-text-muted);margin-top:6px">当前只完成设备网关注册；真实工作包解析或联调点位写入后，才会在这里和农户端展示具体设备。</p><button class="btn btn-primary btn-sm" style="margin-top:12px" onclick="openGatewaySeedModal(\'' + escHtml(gateway.gatewayId) + '\')">+ 添加联调点位</button></div>';
          return;
        }

        var pointsTotal = 0;
        var htmlDetail = '';
        collectors.forEach(function (collector, ci) {
          var col = normalizeCollectorForDisplay(collector);
          col.points = Array.isArray(col.points) ? col.points : [];

          var rowPoints = col.points.filter(function (point) {
            if (!keyword) return true;
            var p = normalizePointForDisplay(point);
            var rowText = [
              p.pointName,
              p.pointId,
              p.shedNo,
              p.deviceExternalId,
              getFunctionTypeName(p.functionType),
              p.unit
            ].join(' ').toLowerCase();
            return rowText.indexOf(keyword) !== -1;
          }).map(function (point) {
            var pointItem = normalizePointForDisplay(point);
            pointItem.frequency = toNum(pointItem.frequency != null ? pointItem.frequency : col.frequency);
            pointItem.collectorId = col.collectorId;
            pointItem.collectorName = col.collectorName;
            pointItem.collectorFrequency = toNum(col.frequency);
            window.__gatewayPagePointCache[pointItem.pointId] = pointItem;
            pointsTotal += 1;
            return '<tr>' +
              '<td style="font-family:monospace;font-size:12px;">' + escHtml(pointItem.pointId || '--') + '</td>' +
              '<td>' + escHtml(pointItem.pointName || '--') + '</td>' +
              '<td>' + escHtml(pointItem.shedNo || '--') + '</td>' +
              '<td>' + escHtml(pointItem.deviceExternalId || '--') + '</td>' +
              '<td>' + toText(pointItem.frequency || '--') + '</td>' +
              '<td>' + escHtml(getFunctionTypeName(pointItem.functionType || pointItem.function_type || pointItem.functionType)) + '</td>' +
              '<td>' + escHtml(pointItem.unit || '--') + '</td>' +
              '<td>' + (pointItem.isBound ? '<span class="badge badge-warning">已绑定</span>' : '<span class="badge badge-muted">未绑定</span>') + '</td>' +
              '<td><div class="table-actions">' +
                '<button class="btn btn-outline btn-sm" onclick="openPointEditModal(\'' + escHtml(pointItem.pointId) + '\')">编辑</button>' +
                '<button class="btn btn-outline btn-sm" onclick="openPointControlModal(\'' + escHtml(pointItem.pointId) + '\')">联调</button>' +
              '</div></td>' +
            '</tr>';
          }).join('');

          htmlDetail += '<div class="gateway-collector-card">' +
            '<div class="gateway-collector-head">' +
              '<span style="font-weight:600">' + escHtml(col.collectorName || '采集执行器') + '</span>' +
              '<span style="font-size:12px;color:var(--color-text-muted)">采集执行器 ' + (ci + 1) + ' · 频点 ' + escHtml(toText(col.frequency || '--')) + ' · DevID ' + escHtml(col.deviceExternalId || '--') + '</span>' +
            '</div>' +
            '<div class="table-wrap">' +
              '<table><thead><tr><th>功能点ID</th><th>功能点名称</th><th>棚号</th><th>DevID</th><th>采样频点</th><th>功能类型</th><th>单位</th><th>绑定状态</th><th>操作</th></tr></thead>' +
              '<tbody>' + (rowPoints || '<tr><td colspan="9" class="table-empty">该采集执行器暂无功能点</td></tr>') + '</tbody></table>' +
            '</div>' +
          '</div>';
        });

        detailWrap.innerHTML =
          '<div class="info-card" style="margin:18px 0 12px">' +
            '<div class="info-grid">' +
              '<div class="info-item"><label>网关ID</label><span style="font-family:monospace;font-size:12px">' + escHtml(gateway.gatewayId || '--') + '</span></div>' +
              '<div class="info-item"><label>网关名称</label><span>' + escHtml(gateway.gatewayName || '--') + '</span></div>' +
              '<div class="info-item"><label>在线</label><span><span class="badge ' + (gateway.online ? 'badge-success' : 'badge-muted') + '">' + (gateway.online ? '在线' : '离线') + '</span></span></div>' +
              '<div class="info-item"><label>采集执行器</label><span>' + collectors.length + ' 个</span></div>' +
              '<div class="info-item"><label>功能点</label><span>' + pointsTotal + ' 个</span></div>' +
            '</div>' +
          '</div>' +
          htmlDetail;
      }

      window.__selectGatewayFromAdmin = function (gatewayId) {
        gatewayDetailState.gatewayId = gatewayId;
        gatewayDetailState.keyword = searchPoint.value || '';
        renderGatewayList(searchGateway.value);
        renderGatewayDetail(gatewayId);
      };

      searchGateway.oninput = function () { renderGatewayList(this.value); };
      searchPoint.oninput = function () {
        gatewayDetailState.keyword = this.value || '';
        renderGatewayDetail(gatewayDetailState.gatewayId);
      };

      renderGatewayList('');
      renderGatewayDetail(gatewayDetailState.gatewayId);
    } catch (e) {
      container.innerHTML = '<div class="error-state"><p>加载失败</p><button class="btn btn-outline btn-sm" onclick="renderPage()">重试</button></div>';
    }
  }

  function getGatewayItemFromPageData(gatewayId) {
    var key = toText(gatewayId);
    var gateways = window.__adminGatewayPageData || [];
    for (var i = 0; i < gateways.length; i++) {
      if (toText(gateways[i].gatewayId) === key) return gateways[i];
    }
    return null;
  }

  function openGatewaySeedModal(gatewayId) {
    gatewayId = toText(gatewayId).trim();
    var gateway = getGatewayItemFromPageData(gatewayId);
    if (!gateway) {
      showToast('请先选择一个设备网关', 'error');
      return;
    }
    var overlay = $('#modal-overlay');
    overlay.style.display = 'flex';
    overlay.innerHTML = '<div class="modal" style="max-width:640px">' +
      '<div class="modal-header">' +
        '<h3>添加联调点位</h3>' +
        '<button class="modal-close" onclick="closeModal()">×</button>' +
      '</div>' +
      '<div class="modal-body">' +
        '<div class="info-card control-point-card">' +
          '<div class="info-grid">' +
            '<div class="info-item"><label>设备网关</label><span>' + escHtml(gateway.gatewayName || '--') + '</span></div>' +
            '<div class="info-item"><label>网关ID</label><span style="font-family:monospace;font-size:12px">' + escHtml(gateway.gatewayId || '--') + '</span></div>' +
          '</div>' +
        '</div>' +
        '<div class="form-row">' +
          '<div class="form-group"><label class="form-label">采样频点</label><input class="form-input" id="seed-frequency" type="number" min="1" step="1" value="1"></div>' +
          '<div class="form-group"><label class="form-label">DevID</label><input class="form-input" id="seed-device-external-id" type="text" value="1001" placeholder="采集执行器设备ID"></div>' +
        '</div>' +
        '<div class="form-row">' +
          '<div class="form-group"><label class="form-label">棚号</label><input class="form-input" id="seed-shed-no" type="text" value="联调棚"></div>' +
          '<div class="form-group"><label class="form-label">采集执行器名称</label><input class="form-input" id="seed-node-name" type="text" value="联调采集执行器 1001"></div>' +
        '</div>' +
        '<div class="form-group"><label class="form-label">备注</label><input class="form-input" id="seed-remark" type="text" value="后台联调生成"></div>' +
        '<div class="hint-text">将创建或更新 4 个默认功能点：联调温度、联调墒情、联调水泵、联调风机。真实硬件工作包解析确认后，可由上报链路自动创建或更新。</div>' +
      '</div>' +
      '<div class="modal-actions">' +
        '<button class="btn btn-outline" onclick="closeModal()">取消</button>' +
        '<button class="btn btn-primary" onclick="submitGatewaySeedPoints(\'' + escHtml(gateway.gatewayId) + '\')">创建联调点位</button>' +
      '</div>' +
    '</div>';
    var devidInput = $('#seed-device-external-id');
    var nodeNameInput = $('#seed-node-name');
    if (devidInput && nodeNameInput) {
      devidInput.oninput = function () {
        nodeNameInput.value = '联调采集执行器 ' + (this.value || '').trim();
      };
    }
  }

  async function submitGatewaySeedPoints(gatewayId) {
    var frequency = toNum($('#seed-frequency').value);
    var deviceExternalId = toText($('#seed-device-external-id').value || '').trim();
    var nodeName = toText($('#seed-node-name').value || '').trim();
    var shedNo = toText($('#seed-shed-no').value || '').trim();
    var remark = toText($('#seed-remark').value || '').trim();
    if (!frequency || frequency <= 0) {
      showToast('请输入有效采样频点', 'error');
      return;
    }
    if (!deviceExternalId) {
      showToast('请输入 DevID', 'error');
      return;
    }

    var btn = $('#modal-overlay .modal-actions .btn-primary');
    if (btn) btn.disabled = true;
    try {
      var res = await NHApi.seedGatewayPoints(gatewayId, {
        frequency: frequency,
        device_external_id: deviceExternalId,
        node_name: nodeName,
        shed_no: shedNo,
        remark: remark
      });
      if (res && res.data) {
        showToast('联调点位已创建：新增 ' + (res.data.created_points || 0) + '，更新 ' + (res.data.updated_points || 0), 'success');
        closeModal();
        renderPage();
        return;
      }
      showToast((res && res.message) || '创建联调点位失败', 'error');
    } catch (e) {
      showToast((e && e.message) || '创建联调点位失败', 'error');
    } finally {
      if (btn) btn.disabled = false;
    }
  }

  function getPointItemFromGatewayData(pointId) {
    var cache = window.__gatewayPagePointCache || {};
    var key = toText(pointId);
    if (key && cache[key]) return cache[key];

    var gateways = window.__adminGatewayPageData || [];
    for (var i = 0; i < gateways.length; i++) {
      var gateway = gateways[i];
      var collectors = gateway.collectors || [];
      for (var c = 0; c < collectors.length; c++) {
        var points = collectors[c].points || [];
        for (var p = 0; p < points.length; p++) {
          var normalized = normalizePointForDisplay(points[p]);
          var pid = toText(normalized.pointId);
          if (pid && pid === key) {
            normalized.collectorId = toText(collectors[c].collectorId);
            normalized.collectorName = collectors[c].collectorName || '';
            normalized.collectorFrequency = toNum(collectors[c].frequency);
            normalized.frequency = toNum(normalized.frequency != null ? normalized.frequency : normalized.collectorFrequency);
            return normalized;
          }
        }
      }
    }
    return null;
  }

  function openPointEditModal(pointId) {
    var point = getPointItemFromGatewayData(pointId);
    if (!point) {
      showToast('未找到可编辑的功能点', 'error');
      return;
    }
    var overlay = $('#modal-overlay');
    overlay.style.display = 'flex';

    overlay.innerHTML = '<div class="modal" style="max-width:680px">' +
      '<div class="modal-header">' +
        '<h3>编辑功能点</h3>' +
        '<button class="modal-close" onclick="closeModal()">×</button>' +
      '</div>' +
      '<div class="modal-body">' +
        '<div class="form-group"><label class="form-label">功能点ID</label><input class="form-input" id="point-edit-id" type="text" readonly value="' + escHtml(point.pointId || '--') + '"></div>' +
        '<div class="form-group"><label class="form-label">功能点名称</label><input class="form-input" id="point-edit-name" type="text" value="' + escHtml(point.pointName || '') + '" placeholder="请输入功能点名称"></div>' +
        '<div class="form-row">' +
          '<div class="form-group"><label class="form-label">采样频点</label><input class="form-input" id="point-edit-frequency" type="number" min="1" step="1" value="' + escHtml(toText(point.frequency || '')) + '" placeholder="按采集执行器频点默认值可留空"></div>' +
          '<div class="form-group"><label class="form-label">采集执行器名称</label><input class="form-input" id="point-edit-collector" type="text" value="' + escHtml(point.collectorName || '') + '" readonly></div>' +
        '</div>' +
        '<div class="form-row">' +
          '<div class="form-group"><label class="form-label">最低值</label><input class="form-input" id="point-edit-best-min" type="number" step="0.01" value="' + escHtml(point.bestMin != null ? point.bestMin : '') + '" placeholder="留空表示不限制"></div>' +
          '<div class="form-group"><label class="form-label">最高值</label><input class="form-input" id="point-edit-best-max" type="number" step="0.01" value="' + escHtml(point.bestMax != null ? point.bestMax : '') + '" placeholder="留空表示不限制"></div>' +
        '</div>' +
        '<div class="form-row">' +
          '<div class="form-group"><label class="form-label">运行模式</label><select class="form-select" id="point-edit-run-mode"><option value="1"' + ((point.runMode === 1 || point.runMode === '1') ? ' selected' : '') + '>自动</option><option value="2"' + ((point.runMode === 2 || point.runMode === '2') ? ' selected' : '') + '>手动</option></select></div>' +
          '<div class="form-group"><label class="form-label">是否可控</label><select class="form-select" id="point-edit-controllable"><option value="0"' + ((point.controllableStatus === 0 || point.controllableStatus === '0' || point.controllableStatus === '') ? ' selected' : '') + '>否</option><option value="1"' + ((point.controllableStatus === 1 || point.controllableStatus === '1') ? ' selected' : '') + '>是</option></select></div>' +
        '</div>' +
        '<div class="form-row">' +
          '<div class="form-group"><label class="form-label">轮询周期（秒）</label><input class="form-input" id="point-edit-poll-interval" type="number" min="5" max="86400" step="1" value="' + escHtml(point.pollInterval != null ? point.pollInterval : '') + '" placeholder="5-86400"></div>' +
          '<div class="form-group"><label class="form-label">功能点状态</label><select class="form-select" id="point-edit-point-status"><option value="">未设置</option><option value="0"' + ((point.pointStatus === 0 || point.pointStatus === '0') ? ' selected' : '') + '>0</option><option value="1"' + ((point.pointStatus === 1 || point.pointStatus === '1') ? ' selected' : '') + '>1</option><option value="2"' + ((point.pointStatus === 2 || point.pointStatus === '2') ? ' selected' : '') + '>2</option></select></div>' +
        '</div>' +
        '<div class="form-row">' +
          '<div class="form-group"><label class="form-label">棚号</label><input class="form-input" id="point-edit-shed-no" type="text" value="' + escHtml(point.shedNo || '') + '"></div>' +
          '<div class="form-group"><label class="form-label">单位</label><input class="form-input" id="point-edit-unit" type="text" value="' + escHtml(point.unit || '') + '"></div>' +
        '</div>' +
        '<div class="form-group"><label class="form-label">控制命令</label><input class="form-input" id="point-edit-poll-cmd" type="text" value="' + escHtml(point.pollCmd || '') + '"></div>' +
        '<div class="form-group"><label class="form-label">备注</label><input class="form-input" id="point-edit-remark" type="text" value="' + escHtml(point.remark || '') + '"></div>' +
        '<div class="control-entry">' +
          '<div class="control-entry-head">' +
            '<div>' +
              '<div class="control-entry-title">发送下行 Hex 指令</div>' +
              '<div class="control-entry-sub">管理员联调入口，command_hex 优先；真实语义以硬件协议为准。</div>' +
            '</div>' +
            '<button class="btn btn-outline btn-sm" onclick="openPointControlModal(\'' + escHtml(point.pointId) + '\')">打开联调</button>' +
          '</div>' +
        '</div>' +
      '</div>' +
      '<div class="modal-actions">' +
        '<button class="btn btn-outline" onclick="closeModal()">取消</button>' +
        '<button class="btn btn-primary" onclick="savePointConfig(\'' + escHtml(point.pointId) + '\')">保存</button>' +
      '</div>' +
    '</div>';
  }

  async function savePointConfig(pointId) {
    var payload = {};
    var pointName = toText($('#point-edit-name').value || '').trim();
    var shedNo = toText($('#point-edit-shed-no').value || '').trim();
    var unit = toText($('#point-edit-unit').value || '').trim();
    var pollCmd = toText($('#point-edit-poll-cmd').value || '').trim();
    var bestMin = $('#point-edit-best-min').value;
    var bestMax = $('#point-edit-best-max').value;
    var runMode = $('#point-edit-run-mode').value;
    var controllableStatus = $('#point-edit-controllable').value;
    var pollInterval = $('#point-edit-poll-interval').value;
    var pointStatus = $('#point-edit-point-status').value;
    var remark = toText($('#point-edit-remark').value || '').trim();

    if (pointName) payload.point_name = pointName;
    if (shedNo) payload.shed_no = shedNo;
    if (unit) payload.unit = unit;
    if (pollCmd) payload.poll_cmd = pollCmd;
    if (remark) payload.remark = remark;

    var bestMinNum = toFloat(bestMin);
    var bestMaxNum = toFloat(bestMax);
    if (bestMinNum !== null) payload.best_min = bestMinNum;
    if (bestMaxNum !== null) payload.best_max = bestMaxNum;
    if (bestMinNum !== null && bestMaxNum !== null && bestMinNum > bestMaxNum) {
      showToast('最低值不能大于最高值', 'error');
      return;
    }

    if (runMode !== '' && runMode !== null) payload.run_mode = toNum(runMode);
    if (controllableStatus !== '' && controllableStatus !== null) payload.controllable_status = toNum(controllableStatus);
    var pollIntervalNum = toNum(pollInterval);
    if (pollIntervalNum !== null) payload.poll_interval = pollIntervalNum;
    if (pointStatus !== '' && pointStatus !== null) payload.point_status = toNum(pointStatus);

    if (Object.keys(payload).length === 0) {
      showToast('暂无可提交的修改项', 'info');
      return;
    }

    try {
      var res = await NHApi.updateDevicePoint(pointId, payload);
      if (isBindingResponseSuccess(res)) {
        showToast('保存成功', 'success');
        closeModal();
        renderPage();
        return;
      }
      showToast((res && (res.message || res.code)) || '保存失败', 'error');
    } catch (e) {
      showToast((e && e.message) || '保存失败', 'error');
    }
  }

  function openPointControlModal(pointId) {
    var point = getPointItemFromGatewayData(pointId);
    if (!point) {
      showToast('未找到可联调的功能点', 'error');
      return;
    }
    var overlay = $('#modal-overlay');
    overlay.style.display = 'flex';
    overlay.innerHTML = '<div class="modal" style="max-width:640px">' +
      '<div class="modal-header">' +
        '<h3>发送下行 Hex 指令</h3>' +
        '<button class="modal-close" onclick="closeModal()">×</button>' +
      '</div>' +
      '<div class="modal-body">' +
        '<div class="info-card control-point-card">' +
          '<div class="info-grid">' +
            '<div class="info-item"><label>功能点ID</label><span style="font-family:monospace;font-size:12px">' + escHtml(point.pointId || '--') + '</span></div>' +
            '<div class="info-item"><label>功能点名称</label><span>' + escHtml(point.pointName || '--') + '</span></div>' +
            '<div class="info-item"><label>设备网关</label><span>' + escHtml(point.gatewayName || '--') + '</span></div>' +
            '<div class="info-item"><label>采集执行器</label><span>' + escHtml(point.collectorName || '--') + '</span></div>' +
          '</div>' +
        '</div>' +
        '<div class="form-group"><label class="form-label">command_hex</label><input class="form-input" id="point-control-command-hex" type="text" placeholder="请输入十六进制指令，例如 F0 或 01020304"></div>' +
        '<div class="form-group"><label class="form-label">用途备注</label><textarea class="form-input form-textarea" id="point-control-note" placeholder="联调用途备注，可留空"></textarea></div>' +
      '</div>' +
      '<div class="modal-actions">' +
        '<button class="btn btn-outline" onclick="closeModal()">取消</button>' +
        '<button class="btn btn-primary" onclick="sendPointControlCommand(\'' + escHtml(point.pointId) + '\')">发送下行</button>' +
      '</div>' +
    '</div>';
  }

  async function sendPointControlCommand(pointId) {
    var commandHex = normalizeHexText($('#point-control-command-hex').value || '');
    var note = toText($('#point-control-note').value || '').trim();

    if (!commandHex) {
      showToast('请输入 command_hex', 'error');
      return;
    }
    if (!/^[0-9A-F]+$/.test(commandHex) || commandHex.length % 2 !== 0) {
      showToast('command_hex 必须是偶数长度的十六进制字符串', 'error');
      return;
    }

    var btn = $('#modal-overlay .modal-actions .btn-primary');
    if (btn) btn.disabled = true;
    try {
      var payload = { command_hex: commandHex };
      if (note) payload.remark = note;
      var res = await NHApi.post('/api/v1/admin/device-points/' + encodeURIComponent(pointId) + '/control', payload);
      if (res && res.data) {
        var parts = ['已创建下行命令'];
        if (res.data.status) parts.push('状态 ' + res.data.status);
        if (res.data.command_hex) parts.push('Hex ' + res.data.command_hex);
        showToast(parts.join(' · '), 'success');
        closeModal();
        return;
      }
      showToast((res && res.message) || '创建下行命令失败', 'error');
    } catch (e) {
      showToast((e && e.message) || '创建下行命令失败', 'error');
    } finally {
      if (btn) btn.disabled = false;
    }
  }

  // ---------- 绑定管理 ----------

  async function renderBindings(container) {
    container.innerHTML = '<div class="loading-state"><div class="loading-spinner"></div><p>加载绑定关系...</p></div>';
    try {
      var res = await NHApi.getBindings();
      if (!res.data) { container.innerHTML = '<div class="error-state"><p>加载失败</p><button class="btn btn-outline btn-sm" onclick="renderPage()">重试</button></div>'; return; }
      var bindings = (res.data || []).map(normalizeBindingRecord);

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
          '<input type="text" class="table-search" id="bind-filter-farmer" placeholder="按农户筛选..." style="width:180px">' +
          '<input type="text" class="table-search" id="bind-filter-device" placeholder="按绑定对象筛选（网关/功能点）..." style="width:240px">' +
          '<select class="form-input" id="bind-filter-type" style="width:140px">' +
            '<option value="">全部绑定类型</option>' +
            '<option value="gateway">设备网关</option>' +
            '<option value="point">功能点</option>' +
          '</select>' +
          '<select class="form-input" id="bind-filter-status" style="width:140px">' +
            '<option value="">全部状态</option>' +
            '<option value="active">有效</option>' +
            '<option value="unbound">已解绑</option>' +
          '</select>' +
        '</div>' +
        '<div class="table-wrap"><table><thead><tr>' +
          '<th>绑定ID</th><th>农户</th><th>绑定类型</th><th>绑定对象</th><th>采集执行器/功能点</th><th>状态</th><th>绑定时间</th><th>操作</th>' +
        '</tr></thead><tbody id="bindings-tbody">' +
        '</tbody></table></div></div>';

      container.innerHTML = html;

      // 渲染表格 + 筛选逻辑
      function renderBindingsTable(filterFarmer, filterDevice, filterType, filterStatus) {
        var tbody = $('#bindings-tbody');
        if (!tbody) return;
        var farmerKeyword = toText(filterFarmer || '').toLowerCase();
        var deviceKeyword = toText(filterDevice || '').toLowerCase();
        var bindingType = filterType || '';

        var filtered = bindings.filter(function (b) {
          if (farmerKeyword && (b.userName || b.bind_user_name || b.bind_usr_name || '').toLowerCase().indexOf(farmerKeyword) === -1) return false;
          if (bindingType && b.bindingType !== bindingType) return false;
          if (deviceKeyword) {
            var deviceStr = [
              b.gatewayName || b.gateway_name || '--',
              b.pointName || b.func_name || b.function_name || '',
              b.deviceExternalId || b.device_external_id || '',
              b.shedNo || b.shed_no || '',
              b.pointId || b.point_id || '',
              b.frequency || b.freq || ''
            ].join(' ').toLowerCase();
            if (deviceStr.indexOf(deviceKeyword) === -1) return false;
          }
          var status = b.bindStatus != null ? b.bindStatus : b.bind_status;
          if (filterStatus === 'active' && status !== 1) return false;
          if (filterStatus === 'unbound' && status !== 0 && status !== 2) return false;
          return true;
        });

        tbody.innerHTML = filtered.map(function (b) {
          var status = b.bindStatus != null ? b.bindStatus : b.bind_status;
          var statusLabel = status === 1 ? '<span class="badge badge-success">有效</span>' : '<span class="badge">已解绑</span>';
          var bindingObject = '--';
          var deviceLine = '--';
          var bindingTypeLabel = b.bindingType === 'gateway' ? '设备网关' : '功能点';
          if (b.bindingType === 'gateway') {
            bindingObject = escHtml((b.gatewayName || '--') + (b.gatewayId ? ('（' + b.gatewayId + '）') : ''));
          } else {
            bindingObject = escHtml(b.pointName || b.point_id || '--');
            if (b.pointId || b.point_id) {
              var pointId = toText(b.pointId || b.point_id);
              deviceLine = '点ID：' + pointId;
            } else if (b.deviceExternalId || b.device_external_id) {
              deviceLine = 'DevID：' + escHtml(b.deviceExternalId || b.device_external_id);
            }
          }

          var pointInfo = [];
          if (b.deviceExternalId || b.device_external_id) pointInfo.push('DevID：' + (b.deviceExternalId || b.device_external_id));
          if (b.shedNo || b.shed_no) pointInfo.push('棚号：' + (b.shedNo || b.shed_no));
          if (b.frequency || b.freq) pointInfo.push('频点：' + (b.frequency || b.freq));
          if (pointInfo.length > 0) {
            if (deviceLine === '--') {
              deviceLine = pointInfo.join(' · ');
            } else {
              deviceLine += ' · ' + pointInfo.join(' · ');
            }
          }

          if (b.bindUserName) deviceLine = (deviceLine === '--' ? '' : (deviceLine + ' · ')) + '绑定用户：' + escHtml(b.bindUserName);

          if (deviceLine === '--' && b.gatewayName) {
            deviceLine = escHtml('网关：' + b.gatewayName) ;
          }
          return '<tr>' +
            '<td>' + b.bindId + '</td>' +
            '<td>' + escHtml(b.userName || b.bind_user_name || b.bind_usr_name || '--') + '</td>' +
            '<td><span class="badge ' + (b.bindingType === 'gateway' ? 'badge-info' : 'badge-muted') + '">' + bindingTypeLabel + '</span></td>' +
            '<td>' + bindingObject + '</td>' +
            '<td>' + (deviceLine === '--' ? '--' : deviceLine) + '</td>' +
            '<td>' + statusLabel + '</td>' +
            '<td>' + formatTime(b.createdAt || b.created_at) + '</td>' +
            '<td><button class="btn btn-danger btn-sm js-remove-binding" data-bind-id="' + escHtml(b.bindId || b.bind_id || '') + '">解绑</button></td>' +
          '</tr>';
        }).join('');
        bindRemoveButtons(tbody);
      }

      renderBindingsTable('', '', '', '');

      // 筛选事件
      $('#bind-filter-farmer').oninput = function () {
        renderBindingsTable(this.value, $('#bind-filter-device').value, $('#bind-filter-type').value, $('#bind-filter-status').value);
      };
      $('#bind-filter-device').oninput = function () {
        renderBindingsTable($('#bind-filter-farmer').value, this.value, $('#bind-filter-type').value, $('#bind-filter-status').value);
      };
      $('#bind-filter-type').onchange = function () {
        renderBindingsTable($('#bind-filter-farmer').value, $('#bind-filter-device').value, this.value, $('#bind-filter-status').value);
      };
      $('#bind-filter-status').onchange = function () {
        renderBindingsTable($('#bind-filter-farmer').value, $('#bind-filter-device').value, $('#bind-filter-type').value, this.value);
      };
    } catch (e) {
      container.innerHTML = '<div class="error-state"><p>加载失败</p><button class="btn btn-outline btn-sm" onclick="renderPage()">重试</button></div>';
    }
  }

  function closeModal() {
    $('#modal-overlay').style.display = 'none';
  }

  async function doBindDevice(userId) {
    if (bindingDialogState && !bindingDialogState.userId) {
      bindingDialogState.userId = toText(userId);
    }
    if (!bindingDialogState || !bindingDialogState.userId) {
      showToast('请先选择绑定人', 'error');
      return;
    }

    var payload = normalizeBindingSubmitState(bindingDialogState);
    if (!payload) {
      showToast('请先选择要绑定的对象', 'error');
      return;
    }

    try {
      var res = await NHApi.addBinding(payload);
      if (res && (res.data || res.code === 200 || res.code === 201 || res.success === true)) {
        showToast('绑定成功', 'success');
        bindingDialogState = null;
        closeModal();
        renderPage();
      } else {
        var failName = ((res && res.message) || (res && res.code)) || '绑定失败';
        showToast(String(failName), 'error');
      }
    } catch (e) {
      if (isApiStatusError(e, 409)) {
        var name = ((e && e.data && e.data.existing_user_name) || '其他农户');
        showToast('该对象已绑定给「' + name + '」', 'error');
        return;
      }
      showToast((e && e.message) || '操作异常', 'error');
    }
  }

  // ---------- 绑定管理页 - 新增绑定弹窗 ----------
  async function openBindingAddModal() {
    var overlay = $('#modal-overlay');
    overlay.style.display = 'flex';
    bindingDialogState = {
      mode: 'bindings',
      userId: '',
      bindingType: 'gateway',
      selectedGatewayId: '',
      selectedPointId: '',
      fallbackPointMode: false
    };

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
          '<label class="form-label">绑定类型</label>' +
          '<select class="form-select" id="bind-add-type">' +
            '<option value="gateway" selected>设备网关</option>' +
            '<option value="point">功能点（兼容旧版）</option>' +
          '</select>' +
        '</div>' +
        '<div class="form-group">' +
          '<label class="form-label">搜索候选</label>' +
          '<input type="text" class="form-input" id="bind-add-device-search" placeholder="输入网关名称/网关ID/DevID/功能点关键词...">' +
        '</div>' +
        '<div id="bind-add-device-list" class="bind-candidate-wrap" style="max-height:300px;overflow-y:auto;margin-top:12px">' +
          '<p style="color:var(--color-text-muted);font-size:13px;text-align:center;padding:20px">输入关键词搜索候选</p>' +
        '</div>' +
        '<div id="bind-add-selected" style="display:none;margin-top:12px;padding:10px;background:var(--color-bg);border-radius:6px">' +
          '<span style="font-size:13px;color:var(--color-text-muted)">已选择：</span>' +
          '<span id="bind-add-selected-name" style="font-weight:600"></span>' +
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

    var selectEl = $('#bind-add-farmer');
    var typeEl = $('#bind-add-type');
    var searchEl = $('#bind-add-device-search');
    var listEl = $('#bind-add-device-list');
    var selectedWrap = $('#bind-add-selected');
    var selectedName = $('#bind-add-selected-name');

    function syncUser() {
      bindingDialogState.userId = toText(selectEl.value);
      var btn = $('#btn-confirm-binding-add');
      if (btn) btn.disabled = !(bindingDialogState.userId && (bindingDialogState.selectedGatewayId || bindingDialogState.selectedPointId));
    }
    function syncButtonState() {
      var btn = $('#btn-confirm-binding-add');
      if (btn) btn.disabled = !(bindingDialogState.userId && (bindingDialogState.selectedGatewayId || bindingDialogState.selectedPointId));
    }

    selectEl.onchange = function () {
      syncUser();
    };

    typeEl.onchange = function () {
      bindingDialogState.bindingType = this.value;
      bindingDialogState.selectedGatewayId = '';
      bindingDialogState.selectedPointId = '';
      selectedWrap.style.display = 'none';
      selectedName.textContent = '';
      syncButtonState();
      searchBindingsCandidates(searchEl.value);
    };

    searchEl.oninput = function () {
      clearTimeout(searchTimer);
      searchTimer = setTimeout(function () {
        searchBindingsCandidates(searchEl.value);
      }, 300);
    };

    var searchTimer = null;
    await searchBindingsCandidates('');

    async function searchBindingsCandidates(keyword) {
      listEl.innerHTML = '<div class="loading-state"><div class="loading-spinner"></div><p>搜索候选...</p></div>';
      var type = bindingDialogState.bindingType;
      var candidates = [];

      if (type === 'gateway') {
        try {
          var qs = [];
          if (keyword) qs.push('keyword=' + encodeURIComponent(keyword));
          var gwRes = await NHApi.get('/api/v1/admin/gateway-bind-candidates' + (qs.length ? ('?' + qs.join('&')) : ''));
          candidates = getGatewayCandidateListFromResponse(gwRes || {});
          if (candidates.length === 0) {
            listEl.innerHTML = '<div class="empty-state"><p>没有可绑定的设备网关</p><p style="font-size:12px;color:var(--color-text-muted)">请尝试切换到“功能点（兼容旧版）”继续搜索</p></div>';
            return;
          }
        } catch (e) {
          if (isApiStatusError(e, 404) || isApiStatusError(e, 405) || isApiStatusError(e, 501)) {
            type = 'point';
            bindingDialogState.bindingType = 'point';
            typeEl.value = 'point';
            bindingDialogState.fallbackPointMode = true;
          } else {
            listEl.innerHTML = '<div class="error-state"><p>加载候选失败，请稍后重试</p></div>';
            return;
          }
        }
      }

      if (type === 'point') {
        try {
          var pointRes = await NHApi.searchDevicePoints(keyword, 'unbound');
          candidates = getPointCandidateListFromResponse(pointRes || {});
          if (candidates.length === 0) {
            listEl.innerHTML = '<div class="empty-state"><p>没有可绑定的功能点</p></div>';
            return;
          }
        } catch (e) {
          listEl.innerHTML = '<div class="error-state"><p>加载候选失败</p></div>';
          return;
        }
      }

      var html = '';
      if (type === 'gateway') {
        candidates.forEach(function (g) {
          var isSelected = bindingDialogState.selectedGatewayId === g.gatewayId;
          html += '<div class="bind-candidate-item' + (isSelected ? ' selected' : '') + '" data-gateway-id="' + escHtml(g.gatewayId) + '" data-type="gateway">' +
            '<div style="display:flex;justify-content:space-between;align-items:center">' +
              '<div style="font-weight:600">' + escHtml(g.gatewayName || '未命名网关') + '</div>' +
              '<span class="badge ' + (g.online ? 'badge-success' : 'badge-muted') + '">' + (g.online ? '在线' : '离线') + '</span>' +
            '</div>' +
            '<div style="font-size:12px;color:var(--color-text-muted);margin-top:4px">网关ID：' + escHtml(g.gatewayId || '--') + ' · IP：' + escHtml(g.ip || '--') + (g.port ? (' : ' + escHtml(g.port)) : '') + '</div>' +
            '<div style="font-size:12px;color:var(--color-text-muted);margin-top:4px">采集执行器：' + (g.collectorCount != null ? g.collectorCount : '--') + ' · 功能点：' + (g.pointCount != null ? g.pointCount : '--') + '</div>' +
          '</div>';
        });
      } else {
        candidates.forEach(function (p) {
          var isSelected = bindingDialogState.selectedPointId === p.pointId;
          html += '<div class="bind-candidate-item' + (isSelected ? ' selected' : '') + '" data-point-id="' + escHtml(p.pointId) + '" data-type="point" data-gateway-name="' + escHtml(p.gatewayName || '') + '">' +
            '<div style="font-weight:600">' + escHtml(p.pointName || '--') + '</div>' +
            '<div style="font-size:12px;color:var(--color-text-muted);margin-top:4px">' +
              '网关：' + escHtml(p.gatewayName || '--') + (p.frequency != null ? ' · 频点：' + p.frequency : '') +
            '</div>' +
            '<div style="font-size:12px;color:var(--color-text-muted);margin-top:2px">' +
              '棚号：' + escHtml(p.shedNo || '--') + ' · DevID：' + escHtml(p.deviceExternalId || '--') +
            '</div>' +
          '</div>';
        });
      }

      listEl.innerHTML = html;
      listEl.querySelectorAll('.bind-candidate-item').forEach(function (el) {
        el.onclick = function () {
          if (this.dataset.type === 'gateway') {
            bindingDialogState.bindingType = 'gateway';
            bindingDialogState.selectedGatewayId = this.dataset.gatewayId;
            bindingDialogState.selectedPointId = '';
            var gatewayLine = (this.querySelector('div') && this.querySelector('div').textContent) || '';
            selectedName.textContent = gatewayLine + '（设备网关）';
          } else {
            bindingDialogState.bindingType = 'point';
            bindingDialogState.selectedPointId = this.dataset.pointId;
            bindingDialogState.selectedGatewayId = '';
            var pText = (this.querySelector('div') && this.querySelector('div').textContent) || '';
            var gName = this.dataset.gatewayName ? ('（' + this.dataset.gatewayName + '）') : '';
            selectedName.textContent = pText + gName;
          }
          listEl.querySelectorAll('.bind-candidate-item').forEach(function (item) {
            item.classList.remove('selected');
          });
          this.classList.add('selected');
          selectedWrap.style.display = 'block';
          syncButtonState();
        };
      });
    }
  }

  async function doBindFromAddModal() {
    if (!bindingDialogState || !bindingDialogState.userId) {
      bindingDialogState = bindingDialogState || {};
      bindingDialogState.userId = toText($('#bind-add-farmer').value);
    }
    if (!bindingDialogState.userId) {
      showToast('请选择农户', 'error');
      return;
    }

    var payload = normalizeBindingSubmitState(bindingDialogState);
    if (!payload) {
      showToast('请选择一个对象', 'error');
      return;
    }

    try {
      var res = await NHApi.addBinding(payload);
      if (res && (res.data || res.code === 200 || res.code === 201 || res.success === true)) {
        showToast('绑定成功', 'success');
        bindingDialogState = null;
        closeModal();
        renderPage();
      } else if (isApiStatusError(res, 409) || res.code === 409) {
        showToast('该对象已绑定给「' + ((res.data && res.data.existing_user_name) || '其他农户') + '」', 'error');
        return;
      } else {
        showToast((res && res.message) || '绑定失败', 'error');
      }
    } catch (e) {
      if (isApiStatusError(e, 409)) {
        showToast('该对象已绑定给「' + ((e.data && e.data.existing_user_name) || '其他农户') + '」', 'error');
        return;
      }
      showToast((e && e.message) || '操作异常', 'error');
    }
  }

  function bindRemoveButtons(root) {
    if (!root) return;
    root.querySelectorAll('.js-remove-binding').forEach(function (btn) {
      btn.onclick = function () {
        doRemoveBinding(this.dataset.bindId || '');
      };
    });
  }

  async function doRemoveBinding(id) {
    id = toText(id).trim();
    if (!id) {
      showToast('绑定ID缺失', 'error');
      return;
    }
    if (!confirm('确认解除该绑定关系？')) return;
    try {
      var res = await NHApi.removeBinding(id);
      if ((res.data && res.data.deleted) || res.code === 200 || res.success === true) {
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
      var logs = (res.data || []).map(normalizeControlLogRecord);

      var html = '<div class="table-section">' +
        '<div class="table-header">' +
          '<h3>控制日志</h3>' +
          '<div class="table-header-right">' +
            '<input class="table-search" id="log-search" type="text" placeholder="搜索操作人/设备/Hex...">' +
            '<span class="table-count">共 ' + logs.length + ' 条</span>' +
          '</div>' +
        '</div>' +
        '<div class="table-wrap"><table>' +
          '<thead><tr><th>ID</th><th>来源</th><th>目标功能点 / 设备网关</th><th>操作人</th><th>功能</th><th>操作类型</th><th>command_hex</th><th>命令状态</th><th>结果</th><th>时间</th></tr></thead>' +
          '<tbody id="log-tbody">' +
            logs.map(function (l) {
              var statusClass = l.cmdStatusLabel === '成功' || l.cmdStatusLabel === '已完成' || l.cmdStatusLabel === 'succeeded' ? 'badge-success' :
                l.cmdStatusLabel === '失败' || l.cmdStatusLabel === 'failed' ? 'badge-danger' :
                l.cmdStatusLabel === '超时' || l.cmdStatusLabel === 'timeout' ? 'badge-warning' :
                l.cmdStatusLabel === 'pending' ? 'badge-warning' :
                l.cmdStatusLabel === 'sent' || l.cmdStatusLabel === 'ack' ? 'badge-info' : 'badge-muted';
              var targetHtml = '<div>' + escHtml(l.targetSummary || '--') + '</div>';
              if (l.pointId || l.gatewayName) {
                var fallbackLine = [];
                if (l.pointId && l.pointId !== l.targetSummary) fallbackLine.push('点 ' + l.pointId);
                if (l.nodeId) fallbackLine.push('采集执行器 ' + l.nodeId);
                if (l.gatewayName) fallbackLine.push(l.gatewayName);
                if (l.frequency) fallbackLine.push('频点 ' + l.frequency);
                if (l.devid) fallbackLine.push('DevID ' + l.devid);
                if (fallbackLine.length) {
                  targetHtml += '<div style="font-size:11px;color:var(--color-text-muted);margin-top:2px">' + escHtml(fallbackLine.join(' · ')) + '</div>';
                }
              }
              var resultParts = [];
              if (l.gatewayResponse) resultParts.push('响应：' + l.gatewayResponse);
              if (l.errorMessage) resultParts.push('错误：' + l.errorMessage);
              if (!resultParts.length && l.result) resultParts.push(l.result);
              var timeHtml = '<div style="white-space:nowrap">' + formatFullTime(l.opTime) + '</div>';
              var timeParts = [];
              if (l.sentAt) timeParts.push('下发 ' + formatFullTime(l.sentAt));
              if (l.finishedAt) timeParts.push('完成 ' + formatFullTime(l.finishedAt));
              if (timeParts.length) {
                timeHtml += '<div style="font-size:11px;color:var(--color-text-muted);margin-top:2px">' + escHtml(timeParts.join(' · ')) + '</div>';
              }
              return '<tr>' +
                '<td style="font-family:monospace;font-size:12px">' + escHtml(l.opId || '--') + '</td>' +
                '<td><span class="badge badge-muted">' + escHtml(l.sourceLabel || '--') + '</span></td>' +
                '<td class="log-target-cell">' + targetHtml + '</td>' +
                '<td>' + escHtml(l.bindUserName || '--') + '</td>' +
                '<td>' + escHtml(l.funcName || '--') + '</td>' +
                '<td>' + escHtml(l.opType || '--') + '</td>' +
                '<td><code class="code-pill">' + escHtml(formatHexDisplay(l.commandHex) || '--') + '</code></td>' +
                '<td><span class="badge ' + statusClass + '">' + escHtml(l.cmdStatusLabel || '--') + '</span></td>' +
                '<td class="log-result-cell">' + escHtml(resultParts.join('；') || '--') + '</td>' +
                '<td>' + timeHtml + '</td>' +
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

  // ---------- 原始报文排查 ----------
  var payloadState = { page: 1, size: 20, event_type: null, parse_status: null, start: null, end: null, keyword: null };

  async function renderPayloadEvents(container) {
    container.innerHTML = '<div class="loading-state"><div class="loading-spinner"></div><p>加载原始报文...</p></div>';

    var params = { page: payloadState.page, size: payloadState.size };
    var useServerEventTypeFilter = payloadState.event_type ? payloadFilterSupportsServer(payloadState.event_type) : false;
    var eventTypeForServer = useServerEventTypeFilter ? payloadTypeToCategoryFilter(payloadState.event_type) : null;
    if (eventTypeForServer != null) {
      params.event_type = eventTypeForServer;
    }
    if (payloadState.parse_status != null) params.parse_status = payloadState.parse_status;
    if (payloadState.start) params.start = payloadState.start;
    if (payloadState.end) params.end = payloadState.end;
    if (payloadState.keyword) params.keyword = payloadState.keyword;

    try {
      var res = await NHApi.getPayloadEvents(params);
      var data = res.data || {};
      var items = (data.items || []).map(normalizePayloadEventFromResponse);
      if (payloadState.event_type) {
        items = items.filter(function (item) { return item.eventCategory === payloadState.event_type; });
      }
      if (payloadState.parse_status != null) {
        items = items.filter(function (item) { return item.parseStatus === payloadState.parse_status; });
      }
      var total = data.total || 0;
      var page = data.page || 1;
      var size = data.size || 20;
      if (payloadState.event_type && !useServerEventTypeFilter) {
        total = items.length;
      }

      var totalPages = Math.ceil(total / size);

      var eventTypeOptions = [
        { val: '', label: '全部类型' },
        { val: 'register', label: '注册' },
        { val: 'heartbeat', label: '心跳' },
        { val: 'work', label: '工作数据' },
        { val: 'alarm', label: '告警' },
        { val: 'unknown', label: '未知包' }
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
          '<h3>原始报文</h3>' +
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
          '<input type="text" class="table-search" id="pe-filter-keyword" style="width:180px;height:32px" value="' + escHtml(payloadState.keyword || '') + '" placeholder="搜索网关/消息ID/Hex">' +
          '<button class="btn btn-outline btn-sm" onclick="payloadResetFilters()">重置</button>' +
        '</div>' +
        '<div class="table-wrap"><table>' +
          '<thead><tr>' +
            '<th>ID</th><th>接收时间</th><th>事件类型</th><th>设备网关</th><th>来源</th><th>解析状态</th><th>消息ID</th><th>事件数</th><th>来源IP</th><th>原始报文</th><th>操作</th>' +
          '</tr></thead>' +
          '<tbody id="payload-tbody">' +
            (items.length === 0 ? '<tr><td colspan="11" class="table-empty">暂无数据</td></tr>' :
              items.map(function (item) {
                var statusCSS = getPayloadParseStatusClass(item.parseStatus);
                var statusText = getPayloadParseStatusLabel(item.parseStatus, item.parseStatusName);
                var isMock = !!item.isMock;
                var sourceName = item.sourceName || '--';
                var ip = item.remoteIp || '--';
                var port = item.remotePort || '';
                var remoteAddress = ip;
                if (ip && port) remoteAddress = ip + ':' + port;
                else if (port) remoteAddress = port;
                return '<tr>' +
                  '<td style="font-family:monospace;font-size:12px">' + escHtml(String(item.rawEventId)) + '</td>' +
                  '<td style="white-space:nowrap;font-size:12px">' + escHtml(item.receivedAt || '--') + '</td>' +
                  '<td><span class="badge ' + item.eventTypeBadgeClass + '">' + escHtml(item.eventTypeLabel) + '</span></td>' +
                  '<td>' + escHtml(item.gatewayName || '--') + '</td>' +
                  '<td><span class="badge ' + (isMock ? 'badge-warning' : 'badge-muted') + '">' + (isMock ? 'MOCK' : escHtml(sourceName)) + '</span></td>' +
                  '<td><span class="badge ' + statusCSS + '">' + escHtml(statusText) + '</span>' +
                    (item.parseError ? ' <span style="font-size:10px;color:var(--color-text-muted)" title="' + escHtml(item.parseError) + '">⚠</span>' : '') +
                  '</td>' +
                  '<td style="font-family:monospace;font-size:11px;max-width:160px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">' + (isMock ? '<span class="badge badge-warning" style="margin-right:4px">MOCK</span>' : '') + escHtml(item.messageId || '--') + '</td>' +
                  '<td><span style="font-size:12px">' + (item.parsedCount != null ? item.parsedCount : 0) + '/' + (item.eventCount != null ? item.eventCount : 0) + '</span></td>' +
                  '<td style="font-size:12px">' + escHtml(remoteAddress) + '</td>' +
                  '<td style="font-size:11px;color:var(--color-text-muted)">' + (item.payloadLen != null ? item.payloadLen : 0) + 'B</td>' +
                  '<td><div style="display:flex;gap:4px">' +
                    '<button class="btn btn-outline btn-sm" onclick="payloadCopySnapshot(\'' + item.rawEventId + '\')" title="拷贝原始报文">📋</button>' +
                    '<button class="btn btn-outline btn-sm" onclick="payloadViewDetail(\'' + item.rawEventId + '\')" title="查看详情">🔍</button>' +
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
      $('#pe-filter-type').onchange = function () { var v = this.value; payloadState.event_type = v === '' ? null : v; payloadState.page = 1; renderPage(); };
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
        showToast('已复制原始报文 (' + copyText.length + ' 字符)', 'success');
      } else {
        var ta = document.createElement('textarea');
        ta.value = copyText;
        ta.style.position = 'fixed'; ta.style.left = '-9999px';
        document.body.appendChild(ta);
        ta.select();
        document.execCommand('copy');
        document.body.removeChild(ta);
        showToast('已复制原始报文 (' + copyText.length + ' 字符)', 'success');
      }
    } catch (e) {
      showToast('拷贝失败：' + (e.message || '未知错误'), 'error');
    }
  }

  async function payloadViewDetail(rawEventId) {
    var overlay = $('#modal-overlay');
    overlay.style.display = 'flex';
    overlay.innerHTML = '<div class="modal" style="max-width:720px">' +
      '<div class="modal-header">' +
        '<h3>原始报文详情</h3>' +
        '<button class="modal-close" onclick="closeModal()">×</button>' +
      '</div>' +
      '<div class="modal-body" style="max-height:70vh;overflow-y:auto">' +
        '<div class="loading-state"><div class="loading-spinner"></div><p>加载中...</p></div>' +
      '</div>' +
      '<div class="modal-actions">' +
        '<button class="btn btn-outline" onclick="closeModal()">关闭</button>' +
        '<button class="btn btn-primary btn-sm" id="btn-payload-copy-detail" style="display:none" onclick="payloadCopySnapshot(\'' + rawEventId + '\')">📋 拷贝原始报文</button>' +
      '</div>' +
    '</div>';

    try {
      var res = await NHApi.getPayloadEventDetail(rawEventId);
      if (!res.data) {
        overlay.querySelector('.modal-body').innerHTML = '<div class="error-state"><p>获取详情失败</p></div>';
        return;
      }
      var d = normalizePayloadEventFromResponse(res.data);
      var isMock = !!d.isMock;
      var statusText = getPayloadParseStatusLabel(d.parseStatus, d.parseStatusName);
      var parseLabel = escHtml(statusText) + (d.parseError ? ' <span style="color:var(--color-danger);font-size:11px">' + escHtml(d.parseError) + '</span>' : '');
      var remoteAddress = d.remoteIp || '--';
      if (d.remoteIp && d.remotePort) remoteAddress = d.remoteIp + ':' + d.remotePort;
      else if (d.remotePort) remoteAddress = d.remotePort;
      var payloadHex = formatHexDisplay(d.payloadHex || res.data.payload_hex || '');
      var payloadAscii = d.protocolSummary && d.protocolSummary.ascii ? d.protocolSummary.ascii : hexToAscii(d.payloadHex || res.data.payload_hex || '');
      var protocolSummary = d.protocolSummary || normalizeProtocolSummary(res.data.protocol_summary || res.data.protocolSummary || null);
      var protocolLabelText = (protocolSummary && (protocolSummary.label || protocolSummary.rawText)) || '--';
      var protocolKindText = (protocolSummary && (protocolSummary.kind || protocolSummary.rawText)) || '--';
      var protocolDescText = (protocolSummary && (protocolSummary.description || protocolSummary.rawText)) || '--';
      var crcText = '--';
      if (protocolSummary) {
        if (protocolSummary.crcValid === true) crcText = '通过';
        else if (protocolSummary.crcValid === false) crcText = '不通过';
      }

      var protocolFieldsHtml = '';
      if (protocolSummary && protocolSummary.fields && protocolSummary.fields.length) {
        protocolFieldsHtml = '<div class="detail-mini-table">' +
          protocolSummary.fields.map(function (field) {
            return '<div class="detail-mini-row">' +
              '<div class="detail-mini-key">' + escHtml(field.label || field.name || '--') + '</div>' +
              '<div class="detail-mini-val">' + escHtml(field.value || '--') + '</div>' +
            '</div>';
          }).join('') +
        '</div>';
      }

      var bodyHtml = '<div class="detail-grid">' +
        '<div class="info-item"><label>原始报文 ID</label><span style="font-family:monospace">' + escHtml(String(d.rawEventId || '--')) + '</span></div>' +
        '<div class="info-item"><label>接收时间</label><span>' + escHtml(d.receivedAt || '--') + '</span></div>' +
        '<div class="info-item"><label>事件类型</label><span class="badge ' + d.eventTypeBadgeClass + '">' + escHtml(d.eventTypeLabel) + '</span></div>' +
        '<div class="info-item"><label>解析状态</label><span>' + parseLabel + '</span></div>' +
        '<div class="info-item"><label>设备网关</label><span>' + escHtml(d.gatewayName || '--') + '</span></div>' +
        '<div class="info-item"><label>来源</label><span><span class="badge ' + (isMock ? 'badge-warning' : 'badge-muted') + '">' + (isMock ? 'MOCK' : escHtml(d.sourceName || '--')) + '</span></span></div>' +
        '<div class="info-item"><label>消息 ID</label><span style="font-family:monospace;font-size:11px">' + (isMock ? '<span class="badge badge-warning" style="margin-right:4px">MOCK</span>' : '') + escHtml(d.messageId || '--') + '</span></div>' +
        '<div class="info-item"><label>事件数量</label><span>' + (d.parsedCount != null ? d.parsedCount : 0) + '/' + (d.eventCount != null ? d.eventCount : 0) + '</span></div>' +
        '<div class="info-item"><label>来源 IP</label><span>' + escHtml(remoteAddress) + '</span></div>' +
        '<div class="info-item"><label>Payload 长度</label><span>' + escHtml(toText(d.payloadByteLen != null ? d.payloadByteLen : d.payloadLen != null ? d.payloadLen : '--')) + '</span></div>' +
      '</div>';

      bodyHtml += '<div class="detail-section">' +
        '<div class="detail-section-head">' +
          '<div>' +
            '<div class="detail-section-title">原始 Hex</div>' +
            '<div class="detail-section-sub">已兼容 `payload_hex` 为空的情况。</div>' +
          '</div>' +
        '</div>' +
        '<pre class="code-panel">' + escHtml(payloadHex || '(空)') + '</pre>' +
      '</div>';

      bodyHtml += '<div class="detail-section">' +
        '<div class="detail-section-head">' +
          '<div>' +
            '<div class="detail-section-title">ASCII</div>' +
            '<div class="detail-section-sub">由协议摘要优先展示，缺失时按 Hex 反推。</div>' +
          '</div>' +
        '</div>' +
        '<pre class="code-panel">' + escHtml(payloadAscii || '(空)') + '</pre>' +
      '</div>';

      bodyHtml += '<div class="detail-grid detail-grid-4">' +
        '<div class="info-item"><label>CRC</label><span><span class="badge ' + (crcText === '通过' ? 'badge-success' : crcText === '不通过' ? 'badge-danger' : 'badge-muted') + '">' + escHtml(crcText) + '</span></span></div>' +
        '<div class="info-item"><label>协议摘要</label><span>' + escHtml(protocolLabelText) + '</span></div>' +
        '<div class="info-item"><label>协议类型</label><span>' + escHtml(protocolKindText) + '</span></div>' +
        '<div class="info-item"><label>协议说明</label><span>' + escHtml(protocolDescText) + '</span></div>' +
      '</div>';

      if (protocolFieldsHtml) {
        bodyHtml += '<div class="detail-section">' +
          '<div class="detail-section-head">' +
            '<div>' +
              '<div class="detail-section-title">协议字段</div>' +
              '<div class="detail-section-sub">后端已解析出的稳定字段。</div>' +
            '</div>' +
          '</div>' +
          protocolFieldsHtml +
        '</div>';
      }

      var snapshot = res.data.payload_snapshot || '';
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
        '<label style="font-size:11px;color:var(--color-text-muted);text-transform:uppercase;letter-spacing:0.3px;margin-bottom:6px;display:block">原始报文快照' + (isJSON ? ' (JSON)' : '') + ' — ' + snapshot.length + ' 字符</label>' +
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
  window.openPointEditModal = openPointEditModal;
  window.openPointControlModal = openPointControlModal;
  window.openGatewaySeedModal = openGatewaySeedModal;
  window.submitGatewaySeedPoints = submitGatewaySeedPoints;
  window.savePointConfig = savePointConfig;
  window.sendPointControlCommand = sendPointControlCommand;

})();
