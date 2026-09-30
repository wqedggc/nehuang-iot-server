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
  return { inspectHex: inspectHex };
});
