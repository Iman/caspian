// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh
//
// Shared by both suites' hooks. It answers one question: for the scenario about
// to run, which defect should the appliance be built with.
//
// In an ordinary run the answer is "none", and CASPIAN_DEFECT is empty. In a
// mutation run, CASPIAN_MUTATION is set, and the answer comes from the
// scenario's own tag through bdd/defects.json.
//
// Doing it per scenario rather than per process is not only faster. Running one
// scenario per cucumber process meant one Chrome per scenario, which on this
// machine leaked browsers and wedged on the fourteenth row. One process, one
// browser, one appliance rebuilt between scenarios is the same isolation for a
// fraction of the cost, and it is what test/bdd's TestEveryScenarioCanFail does
// in Go.

'use strict';

const path = require('path');
const fs = require('fs');

const registryPath = path.resolve(__dirname, 'defects.json');
const registry = JSON.parse(fs.readFileSync(registryPath, 'utf8'));

// Tags that describe a scenario rather than name its subject. They are never a
// defect key, and a scenario carrying only these has named no defect.
const ORGANISING_TAGS = new Set(['@smoke', '@ready', '@known-defect']);

function mutating() {
  return process.env.CASPIAN_MUTATION === '1';
}

// defectFor returns the defect name for a scenario, and throws when a mutation
// run meets a scenario that names none.
//
// The throw is the point. It is the same rule test/bdd holds with
// TestEveryScenarioNamesADefect: there must be no way to add a scenario without
// also adding the thing that proves it can fail.
function defectFor(suite, tags) {
  if (!mutating()) {
    return process.env.CASPIAN_DEFECT || '';
  }
  const table = registry[suite];
  if (!table) {
    throw new Error('bdd/defects.json has no section for the suite "' + suite + '"');
  }
  const named = tags.filter((t) => !ORGANISING_TAGS.has(t));
  for (const tag of named) {
    if (table[tag]) {
      return table[tag].defect;
    }
  }
  throw new Error(
    'this scenario names no defect, so nobody has seen it fail. Its tags are [' +
      tags.join(', ') + ']. Add a row for one of them to bdd/defects.json, and ' +
      'implement the defect in defectsByName in bdd/harness/main.go.'
  );
}

function expectationFor(suite, tags) {
  const table = registry[suite] || {};
  for (const tag of tags.filter((t) => !ORGANISING_TAGS.has(t))) {
    if (table[tag]) {
      return { tag, defect: table[tag].defect, expect: table[tag].expect };
    }
  }
  return null;
}

module.exports = { mutating, defectFor, expectationFor, registry };
