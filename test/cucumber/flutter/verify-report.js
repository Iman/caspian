// SPDX-License-Identifier: AGPL-3.0-or-later
'use strict';
const fs = require('node:fs');
const assert = require('node:assert/strict');
const [report, started] = process.argv.slice(2);
assert.ok(fs.statSync(report).mtimeMs >= Number(started), 'BDD report is stale');
const features = JSON.parse(fs.readFileSync(report, 'utf8'));
const scenarios = features.flatMap(feature => feature.elements || [])
  .filter(element => element.type === 'scenario');
assert.ok(scenarios.length > 0, 'No Flutter BDD scenarios executed');
const failed = scenarios.filter(scenario => !scenario.steps?.length ||
  scenario.steps.some(step => step.result?.status !== 'passed'));
const steps = scenarios.reduce((count, scenario) => count + scenario.steps.length, 0);
console.log(`Flutter BDD: ${scenarios.length} scenarios, ${steps} steps, ${failed.length} failed scenarios`);
assert.equal(failed.length, 0, 'Failed or skipped scenarios: ' + failed.map(scenario => scenario.name).join(', '));
