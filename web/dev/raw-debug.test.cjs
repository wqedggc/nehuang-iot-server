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

const { validateDownlink } = require('../static/admin/raw-debug.js');
for (const hex of ['7E09000103122501818B92', '000103122501818B92', '7E051439002394', '7E0C000101140C1F010401403D09']) assert.equal(validateDownlink(hex).hex, hex);
for (const hex of ['0100020C0000C071', '7E080100020C0000C071', '0100020C0000C0710100030C0000004C90', '7E080100020C0000C0717E090100030C0000004C90', '7E09000103122501818B9200', '7E09000103122501818B927E09000103122501818B92', '7E09000103122501818B00', 'F0', '0103']) assert.throws(() => validateDownlink(hex));
console.log('PASS safe single-frame transmit policy and alternate BB02/multi-frame/trailing bypasses');
