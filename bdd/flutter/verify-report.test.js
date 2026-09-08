// SPDX-License-Identifier: AGPL-3.0-or-later
'use strict';
const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

// Synthetic reports test the report validator. They are not product results.
for (const [name, status, stale, expected] of [
  ['completed scenario', 'passed', false, 0],
  ['no scenarios', null, false, 1],
  ['skipped step', 'skipped', false, 1],
  ['failed step', 'failed', false, 1],
  ['stale report', 'passed', true, 1],
]) {
  test('report validator: ' + name, () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'caspian-report-validator-'));
    const report = path.join(dir, 'synthetic.json');
    try {
      const started = Date.now() - 1000;
      const elements = status === null ? [] : [{
        type: 'scenario', name: 'Synthetic validator fixture', steps: [{ result: { status } }],
      }];
      fs.writeFileSync(report, JSON.stringify([{ elements }]));
      if (stale) fs.utimesSync(report, new Date(0), new Date(0));
      const result = spawnSync(process.execPath, [path.join(__dirname, 'verify-report.js'), report, String(started)]);
      assert.equal(result.status, expected, result.stderr.toString());
    } finally { fs.rmSync(dir, { recursive: true, force: true }); }
  });
}
