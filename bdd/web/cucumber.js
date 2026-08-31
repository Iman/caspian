// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh
//
// The profiles, so that a bare `cucumber-js` does the right thing. That is what
// the README documents and what a person types.
//
// FORCE_COLOR is set here rather than in run.sh, and it has to be. This project
// bans ANSI escape codes outright, in every file and every stream, and a bare
// `cucumber-js` typed by hand never goes through run.sh. Setting it in the
// config module is the only place that catches both. The deprecated
// formatOptions.colorsEnabled would do the same job and print a deprecation
// warning while doing it.
process.env.FORCE_COLOR = '0';

// THE DEFAULT PROFILE EXCLUDES ONE SCENARIO, AND THAT IS NOT TIDINESS.
//
// bdd/web/features/NegativeTests.feature ends with a scenario tagged
// known-defect: the device count that claims a phone is connected while the
// appliance is switched off. It is RED on this build, on purpose, because it
// describes what the panel should do and the panel does something else.
// Measured 2026-08-30 through the real panel: with the engine stopped and the
// hotspot down, the dashboard renders "1" on the devices tile and
// "1 device connected" on the line under the hotspot card.
//
// It is excluded from the default run rather than deleted or softened, because
// a suite that is permanently red teaches people to ignore it, and a suite that
// has quietly dropped its only failing scenario teaches them something worse.
//
// It gets a profile of its own rather than a tag expression on the command
// line. A profile's tags and the command line's tags are ANDed, so
//
//     cucumber-js --tags @known-defect
//
// resolves to "not @known-defect and @known-defect", matches nothing, and exits
// 0 having run no scenarios. That is a false green of exactly the shape
// scripts/gate.sh warns about in its header, and it is why bdd/mutation.sh
// refuses a run that reports zero scenarios. Use:
//
//     cucumber-js --profile defect
//
// and expect it to fail until the count is gated on the hotspot being up.

const common = {
  require: ['features/step_definitions/**/*.js', 'features/support/**/*.js'],
  format: ['progress', 'summary'],
  publishQuiet: true,
};

module.exports = {
  // Everything that should be green.
  default: { ...common, tags: 'not @known-defect' },

  // The known open defect, alone. Expected to FAIL.
  defect: { ...common, tags: '@known-defect' },

  // Everything, green and red together, for a reader who wants the whole
  // picture in one report.
  all: { ...common },
};
