/**
 * 涅凰智农 IoT 云平台 — 前端版本号
 * 修改此文件后，所有引用此文件的页面会自动更新版本号
 * 格式：主版本.次版本.修订号 (YYYYMMDD)
 */
(function (global) {
  'use strict';

  const VERSION = '0.1.0';
  const BUILD_DATE = '20260629';
  const VERSION_STRING = VERSION + ' (' + BUILD_DATE + ')';

  global.APP_VERSION = VERSION;
  global.APP_BUILD_DATE = BUILD_DATE;
  global.APP_VERSION_STRING = VERSION_STRING;

})(window);
