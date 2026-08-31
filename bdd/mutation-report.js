// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh
//
// Reads a Cucumber JSON report from a mutation run and prints the table, then
// exits non-zero if any row did not get the result its registry entry expects.
//
// Usage: node bdd/mutation-report.js <suite> <report.json>
//
// The table is the deliverable. A run where every scenario still passes with its
// subject deliberately broken is a suite that proves nothing, and the point of
// printing one row per scenario is that a reader can see WHICH scenario was
// watched failing and WHAT was broken to make it fail.

'use strict';

const fs = require('fs');
const { expectationFor } = require('./mutation-support');

const suite = process.argv[2];
const reportPath = process.argv[3];

if (!suite || !reportPath) {
  console.error('usage: node bdd/mutation-report.js <suite> <report.json>');
  process.exit(2);
}

let report;
try {
  report = JSON.parse(fs.readFileSync(reportPath, 'utf8'));
} catch (e) {
  console.error('could not read the cucumber report at ' + reportPath + ': ' + e.message);
  process.exit(2);
}

const rows = [];
let failed = 0;
let scenarios = 0;

for (const feature of report) {
  for (const element of feature.elements || []) {
    if (element.type !== 'scenario') continue;
    scenarios += 1;

    const tags = (element.tags || []).map((t) => t.name);
    const entry = expectationFor(suite, tags);

    // A scenario ran and matched no registry entry. That should be impossible,
    // because the Before hook throws for exactly this case, but a guard that
    // only lives in one place is a guard that can be routed around.
    if (!entry) {
      rows.push({
        scenario: element.name,
        tag: '(none)',
        defect: '(none)',
        want: '(none)',
        got: '(none)',
        verdict: 'NAMES NO DEFECT',
      });
      failed += 1;
      continue;
    }

    const steps = element.steps || [];
    const anyFailed = steps.some((s) => s.result && s.result.status === 'failed');
    const got = anyFailed ? 'red' : 'green';
    const verdict = got === entry.expect ? 'ok' : 'WANTED ' + entry.expect;
    if (verdict !== 'ok') {
      failed += 1;
    }

    rows.push({
      scenario: element.name,
      tag: entry.tag,
      defect: entry.defect,
      want: entry.expect,
      got,
      verdict,
    });
  }
}

// A report with no scenarios in it exits 0 from cucumber and proves nothing.
// That false green was hit while building this suite, so it is checked here as
// well as in mutation.sh.
if (scenarios === 0) {
  console.error('the ' + suite + ' mutation run executed NO scenarios, so it tested nothing');
  process.exit(1);
}

const width = (key, header) =>
  Math.max(header.length, ...rows.map((r) => String(r[key]).length));

const w = {
  scenario: Math.min(width('scenario', 'SCENARIO'), 72),
  tag: width('tag', 'TAG'),
  defect: width('defect', 'DEFECT INJECTED'),
};

const pad = (s, n) => {
  s = String(s);
  return s.length > n ? s.slice(0, n - 3) + '...' : s.padEnd(n);
};

console.log('');
console.log('MUTATION TABLE (' + suite + ')');
console.log(
  pad('SCENARIO', w.scenario) + '  ' + pad('TAG', w.tag) + '  ' +
  pad('DEFECT INJECTED', w.defect) + '  ' + pad('WANT', 5) + '  ' + pad('GOT', 5) + '  RESULT'
);
for (const r of rows) {
  console.log(
    pad(r.scenario, w.scenario) + '  ' + pad(r.tag, w.tag) + '  ' +
    pad(r.defect, w.defect) + '  ' + pad(r.want, 5) + '  ' + pad(r.got, 5) + '  ' + r.verdict
  );
}
console.log('');
console.log(scenarios + ' scenarios, ' + (scenarios - failed) + ' behaved as the registry says');

if (failed > 0) {
  console.log('');
  console.log(
    'mutation (' + suite + '): FAILED. A scenario that wanted red and got green does ' +
    'not test what it says.'
  );
  process.exit(1);
}
console.log('mutation (' + suite + '): passed');
