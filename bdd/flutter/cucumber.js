// SPDX-License-Identifier: AGPL-3.0-or-later
'use strict';
process.env.FORCE_COLOR = '0';
module.exports = {
  default: {
    paths: ['features/**/*.feature'],
    require: ['support.js'],
    format: ['progress', 'summary'],
    parallel: 0,
  },
};
