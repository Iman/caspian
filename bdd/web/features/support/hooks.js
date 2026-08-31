// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh
//
// The hooks: what starts, what is thrown away between scenarios, and what stops.
//
// # The appliance
//
// bdd/harness is a Go program that serves the REAL panel against fakes. It is
// started here rather than by run.sh so that a bare `cucumber-js`, typed by
// hand with no wrapper, works. That is one of the four invocations the README
// documents, and a suite that only runs through its own shell script is a suite
// that cannot be run one scenario at a time.
//
// It needs no Raspberry Pi, no root and no network: the radio, the engine and
// the interfaces are the FakePrivileged that internal/panel already ships for
// its own tests, and the listener is on loopback.
//
// # One browser, cleared between scenarios
//
// Chrome is started once and the session is cleared before each scenario, which
// is worth justifying because the cheaper-looking alternative is wrong.
//
// Every scenario runs against the same host and port, and cookies ignore the
// port: RFC 6265 scopes them to the host. So a scenario that signed in would
// leave its session cookie for the next one. Two things stop that. The harness
// is RESET before each scenario, which throws away the store, the sessions and
// the fake and builds new ones, so a carried-over token names a session that no
// longer exists. And the browser's cookies are deleted as well, so the next
// scenario starts from no cookie at all rather than from one that happens to be
// rejected. Either alone would probably do; both together mean scenario order
// cannot change a result, which is the property that matters.

'use strict';

const { BeforeAll, Before, After, AfterAll, Status } = require('@cucumber/cucumber');
const { Builder } = require('selenium-webdriver');
const chrome = require('selenium-webdriver/chrome');
const { spawn, spawnSync } = require('child_process');
const path = require('path');
const os = require('os');
const fs = require('fs');
const readline = require('readline');
const { defectFor } = require('../../../mutation-support');

// The repository root, found from this file rather than from the working
// directory, so the suite can be run from anywhere.
const repoRoot = path.resolve(__dirname, '..', '..', '..', '..');

let harness = null;
let harnessBinary = null;
let driver = null;
let base = null;
const messages = { fa: {}, en: {} };

// buildHarness compiles the harness to a temporary file and returns its path.
//
// It is built and then run, rather than run through `go run`, because `go run`
// puts a wrapper process between this suite and the server. Killing the wrapper
// does not reliably kill what it started, and a harness left holding a port
// outlives the run that created it. Building first costs a second on a cold
// cache and nothing afterwards.
function buildHarness() {
  const out = path.join(os.tmpdir(), 'caspian-bdd-harness-' + process.pid);
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

    // The harness prints one line of JSON with its address. Reading it beats
    // guessing a port, which is the kind of thing that works on a developer
    // machine and collides in CI.
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
      failed(new Error(
        'the harness exited with code ' + code + ' before it was ready.\n' +
        'Run it by hand to see why:  go run ./bdd/harness\n' + stderr
      ));
    });

    setTimeout(() => {
      failed(new Error('the harness did not print its address within 60 seconds.\n' + stderr));
    }, 60 * 1000);
  });
}

async function loadMessages(url) {
  for (const lang of ['fa', 'en']) {
    const res = await fetch(url + '/__control/messages?lang=' + lang);
    if (!res.ok) {
      throw new Error('the harness would not hand over the ' + lang + ' catalogue: ' + res.status);
    }
    messages[lang] = await res.json();
  }
}

BeforeAll({ timeout: 180 * 1000 }, async function () {
  harnessBinary = buildHarness();
  const started = await startHarness(harnessBinary);
  harness = started.proc;
  base = started.url;
  await loadMessages(base);

  const options = new chrome.Options().addArguments(
    // Headless is not optional here. The suite has to run with no display, in
    // CI and over ssh, and a scenario that silently opened a window on somebody
    // machine would be a scenario nobody runs.
    '--headless=new',
    '--no-sandbox',
    '--disable-dev-shm-usage',
    // A fixed window, so a computed colour, an element screenshot and a
    // responsive breakpoint are not functions of whatever size Chrome felt
    // like. panel.css has a breakpoint at 640px and another at a wider size;
    // 1280 is above both, which is the desktop layout these scenarios describe.
    '--window-size=1280,1024'
  );
  driver = await new Builder().forBrowser('chrome').setChromeOptions(options).build();
});

Before({ timeout: 60 * 1000 }, async function (scenario) {
  this.driver = driver;
  this.base = base;
  this.messages = messages;
  this.lang = 'fa';

  // In an ordinary run this is the empty string, and the appliance is built
  // healthy. In a mutation run it is the defect named by this scenario's own
  // tag, and a scenario that names none fails here rather than passing
  // quietly against a build nothing was wrong with.
  const defect = defectFor('web', scenario.pickle.tags.map((t) => t.name));
  await this.reset(defect);

  // Navigate first: a driver that has never visited the origin has no cookie
  // store to clear, and deleteAllCookies would be a silent no-op.
  await this.goto('/login');
  await driver.manage().deleteAllCookies();
});

After(async function (scenario) {
  // A screenshot of the failing page, attached to the report. This is the one
  // thing a browser suite can give a reader that a log line cannot: what the
  // page actually looked like when the assertion went red.
  if (scenario.result && scenario.result.status === Status.FAILED && driver) {
    try {
      const shot = await driver.takeScreenshot();
      this.attach(Buffer.from(shot, 'base64'), 'image/png');
    } catch (e) {
      // A screenshot that cannot be taken must not replace the real failure
      // with its own. The scenario has already failed for a stated reason.
    }
  }
});

AfterAll({ timeout: 60 * 1000 }, async function () {
  if (driver) {
    await driver.quit();
    driver = null;
  }
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
