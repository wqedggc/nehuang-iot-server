const assert = require('node:assert/strict');
const { inspectHex } = require('../static/admin/raw-debug.js');
const v = inspectHex('01 03\n00 00 00 0A');
assert.equal(v.hex, '01030000000A');
assert.equal(v.byteCount, 6);
assert.equal(v.crcHex, null);
for (const input of ['', '0x01', '01ZZ', '0', '<script>', '01,02']) assert.throws(() => inspectHex(input));
assert.throws(() => inspectHex('AA'.repeat(513)));
assert.equal(inspectHex('AA'.repeat(512)).byteCount, 512);
const crc = inspectHex('01 03 00 00 00 0A', { crcStart: 0, crcEnd: 5 });
assert.equal(crc.crcHex, 'C5 CD');
assert.equal(crc.hex, '01030000000A'); // no CRC appended, no send channel
for (const option of [{crcStart: '', crcEnd: 1}, {crcStart: 1}, {crcStart: -1, crcEnd: 1}, {crcStart: 2, crcEnd: 1}, {crcStart: 0, crcEnd: 6}]) assert.throws(() => inspectHex('01 03 00 00 00 0A', option));
assert.equal(inspectHex('7E 02', {crcStart: 0, crcEnd: 1}).hex, '7E02'); // header not stripped
console.log('PASS raw debug validation, explicit CRC range, unchanged payload, 512-byte bound');
