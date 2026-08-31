// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh
//
// The profiles for the HTTP suite.
//
// FORCE_COLOR is set here rather than in run.sh, and it has to be. This project
// bans ANSI escape codes outright, in every file and every stream, and a bare
// `cucumber-js` typed by hand never goes through run.sh. Setting it in the
// config module is the only place that catches both.
process.env.FORCE_COLOR = '0';

const common = {
  require: ['features/step_definitions/**/*.js', 'features/support/**/*.js'],
  format: ['progress', 'summary'],
  publishQuiet: true,
};

module.exports = {
  default: { ...common },
  all: { ...common },
};
