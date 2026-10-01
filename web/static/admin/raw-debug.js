/* Pure offline preview: this module has no HTTP or device-send capability. */
(function (root, factory) {
  var api = factory();
  if (typeof module === 'object' && module.exports) module.exports = api;
  else root.NHRawDebug = api;
})(typeof window !== 'undefined' ? window : globalThis, function () {
  'use strict';
  function inspectHex(input, options) {
    var text = String(input == null ? '' : input).trim();
    if (!text || !/^[0-9a-fA-F\s]+$/.test(text)) throw new Error('只接受十六进制字节和空白，不接受0x、逗号或其他字符。');
    var hex = text.replace(/\s/g, '').toUpperCase();
    if (hex.length % 2 !== 0) throw new Error('Hex必须是偶数个字符。');
    if (hex.length > 1024) throw new Error('预览最多512字节。');
    var bytes = [];
    for (var i = 0; i < hex.length; i += 2) bytes.push(parseInt(hex.slice(i, i + 2), 16));
    var result = { hex: hex, byteCount: bytes.length, formattedHex: hex.match(/.{2}/g).join(' '), crcHex: null };
    if (options != null) {
      var start = options.crcStart, end = options.crcEnd;
      if (!Number.isInteger(start) || !Number.isInteger(end) || start < 0 || end < start || end >= bytes.length) throw new Error('请明确填写合法CRC覆盖区间（从0开始，包含起止字节）。');
      var crc = 0xFFFF;
      for (var j = start; j <= end; j++) {
        crc ^= bytes[j];
        for (var bit = 0; bit < 8; bit++) crc = (crc & 1) ? ((crc >>> 1) ^ 0xA001) : (crc >>> 1);
      }
      function byteHex(value) { return value.toString(16).toUpperCase().padStart(2, '0'); }
      result.crcHex = byteHex(crc & 255) + ' ' + byteHex((crc >>> 8) & 255);
    }
    return result;
  }
  function validateDownlink(input) {
    var result = inspectHex(input), bytes = result.hex.match(/.{2}/g).map(function (v) { return parseInt(v, 16); });
    var body = bytes;
    if (bytes[0] === 0x7E) {
      if (bytes.length < 2 || bytes[1] + 2 !== bytes.length) throw new Error('只允许一帧完整7E/LEN报文，禁止多帧或尾随字节。');
      body = bytes.slice(2);
    }
    if (body.length < 3) throw new Error('需要完整单帧与CRC');
    var crc = 0xFFFF;
    for (var j = 0; j < body.length - 2; j++) {
      crc ^= body[j];
      for (var bit = 0; bit < 8; bit++) crc = (crc & 1) ? ((crc >>> 1) ^ 0xA001) : (crc >>> 1);
    }
    if (body[body.length - 2] !== (crc & 255) || body[body.length - 1] !== ((crc >>> 8) & 255)) throw new Error('正文CRC错误；不自动修正或附加。');
    if (body.length === 5) {
      if (body[0] > 23 || body[1] > 59 || body[2] > 59) throw new Error('二进制时钟不合法');
      return result;
    }
    if (body.length < 8) throw new Error('此下行格式尚未确认，不能发送');
    if (body[2] === 2) throw new Error('BB02清缓存指令当前禁用');
    if (!((body[2] === 0 && body.length === 8) || (body[2] === 3 && body.length === 9) || (body[2] === 1 && body.length === 12))) throw new Error('只允许已核单帧校时、手动、阈值报文；不接受多帧或未知格式。');
    if (body[3] > 23 || body[4] > 59 || body[5] > 59) throw new Error('二进制时钟不合法');
    if (body[2] === 3 && !(body[6] & 128)) throw new Error('手动操作Tag固定最高位必须为1');
    return result;
  }
  return { inspectHex: inspectHex, validateDownlink: validateDownlink };

});
