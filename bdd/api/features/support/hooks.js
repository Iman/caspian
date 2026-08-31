// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh
//
// The hooks for the HTTP suite. Same shape as the browser suite's, minus the
// browser: the harness is started once, and the appliance behind it is thrown
// away and rebuilt before every scenario so that no scenario can see anything
// the one before it did.
//
// The harness is started here rather than by run.sh so that a bare
// `cucumber-js`, typed by hand with no wrapper, works. That is one of the four
// invocations the README documents, and a suite that only runs through its own
// shell script is a suite that cannot be run one scenario at a time.

'use strict';

const { BeforeAll, Before, AfterAll } = require('@cucumber/cucumber');
const { spawn, spawnSync } = require('child_process');
const path = require('path');
const os = require('os');
const fs = require('fs');
const readline = require('readline');
const { defectFor } = require('../../../mutation-support');

const repoRoot = path.resolve(__dirname, '..', '..', '..', '..');

let harness = null;
let harnessBinary = null;
let base = null;
const messages = { fa: {}, en: {} };

// buildHarness compiles the harness to a temporary file and returns its path.
//
// It is built and then run, rather than run through `go run`, because `go run`
// puts a wrapper process between this suite and the server. Killing the wrapper
// does not reliably kill what it started, and a harness left holding a port
// outlives the run that created it.
function buildHarness() {
  const out = path.join(os.tmpdir(), 'caspian-bdd-api-harness-' + process.pid);
  const built = spawnSync('go', ['build', '-o', out, './bdd/harness'], {
    cwd: repoRoot,
    encoding: 'utf8',
  });
  if (built.status !== 0) {
    throw new Error(
      'could not build the harness. Run this by hand to see why:\n' +
      '  go build -o /tmp/caspian-bdd-harness ./bdd/harness\n' +
      (built.stderr || '')
    );
  }
  return out;
}

function startHarness(binary) {
  return new Promise((resolve, reject) => {
    const proc = spawn(binary, ['-addr', '127.0.0.1:0'], {
      cwd: repoRoot,
      stdio: ['ignore', 'pipe', 'pipe'],
    });

    let settled = false;
    const failed = (err) => {
      if (settled) return;
      settled = true;
      reject(err);
    };

    const lines = readline.createInterface({ input: proc.stdout });
    lines.on('line', (line) => {
      if (settled) return;
      let parsed;
      try {
        parsed = JSON.parse(line);
      } catch (e) {
        return;
      }
      if (parsed && parsed.url) {
        settled = true;
        resolve({ proc, url: parsed.url });
      }
    });

    let stderr = '';
    proc.stderr.on('data', (chunk) => {
      stderr += chunk.toString();
    });
    proc.on('error', failed);
    proc.on('exit', (code) => {
      failed(new Error('the harness exited with code ' + code + ' before it was ready.\n' + stderr));
    });

    setTimeout(() => {
      failed(new Error('the harness did not print its address within 60 seconds.\n' + stderr));
    }, 60 * 1000);
  });
}

BeforeAll({ timeout: 180 * 1000 }, async function () {
  harnessBinary = buildHarness();
  const started = await startHarness(harnessBinary);
  harness = started.proc;
  base = started.url;

  for (const lang of ['fa', 'en']) {
    const res = await fetch(base + '/__control/messages?lang=' + lang);
    if (!res.ok) {
      throw new Error('the harness would not hand over the ' + lang + ' catalogue: ' + res.status);
    }
    messages[lang] = await res.json();
  }
});

Before({ timeout: 60 * 1000 }, async function (scenario) {
  this.base = base;
  this.messages = messages;
  // In an ordinary run this is the empty string, and the appliance is built
  // healthy. In a mutation run it is the defect named by this scenario's own
  // tag, and a scenario that names none fails here rather than passing
  // quietly against a build nothing was wrong with.
  const defect = defectFor('api', scenario.pickle.tags.map((t) => t.name));
  await this.reset(defect);
});

AfterAll({ timeout: 60 * 1000 }, async function () {
  if (harness) {
    harness.kill('SIGTERM');
    harness = null;
  }
  if (harnessBinary) {
    try {
      fs.unlinkSync(harnessBinary);
    } catch (e) {
      // A leftover binary in the temporary directory is untidy and harmless.
      // It must not turn a green run red.
    }
    harnessBinary = null;
  }
});
