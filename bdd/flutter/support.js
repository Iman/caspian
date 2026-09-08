// SPDX-License-Identifier: AGPL-3.0-or-later
'use strict';
const assert = require('node:assert/strict');
const path = require('node:path');
const fs = require('node:fs');
const os = require('node:os');
const { createHash } = require('node:crypto');
const { createRequire } = require('node:module');
const { spawn, spawnSync } = require('node:child_process');
const { createInterface } = require('node:readline');
const bdd = createRequire(path.resolve(__dirname, '../web/package.json'));
const { BeforeAll, AfterAll, Before, After, AfterStep, Given, When, Then, Status,
  setDefaultTimeout } = bdd('@cucumber/cucumber');
const { Builder, By, Key, until, logging } = bdd('selenium-webdriver');
const chrome = bdd('selenium-webdriver/chrome');

const repo = path.resolve(__dirname, '../..');
const password = 'correct-horse-battery';
let driver, server, binary, base, startupTimer;
let sourceHash;
setDefaultTimeout(150000);

async function control(action, values = {}) {
  const result = await fetch(base + '/__control/' + action, {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(values),
  });
  assert.equal(result.status, 200, await result.text());
}

async function browserRequest(route, form) {
  return driver.executeAsyncScript(function (route, values, callback) {
    const options = values === null ? {} : {
      method: 'POST', headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      body: new URLSearchParams(values).toString(),
    };
    fetch(route, { ...options, credentials: 'same-origin' }).then(async response => {
      const text = await response.text();
      let body;
      try { body = JSON.parse(text); } catch (_) { body = { text }; }
      callback({ status: response.status, body });
    }).catch(error => callback({ error: String(error) }));
  }, route, form ?? null);
}

async function state() {
  const response = await browserRequest('/api/v1/state');
  assert.ok(!response.error, response.error);
  return response.body;
}

const selector = id => `[flt-semantics-identifier="${id}"]`;
async function locator(id) {
  // ListTile/PopupMenu semantics can expose the user-facing role and name while
  // omitting an enclosing identifier in the web renderer.
  if (id === 'logout' || id === 'language' || id.startsWith('nav-') || id.endsWith('-expand')) {
    const current = await state();
    const name = id === 'language' ? (current.page.Lang === 'fa' ? 'زبان' : 'Language') :
      current.strings[id === 'logout' ? 'nav.signout' : id === 'config-expand' ?
        (current.page.HasConfig ? 'config.replace' : 'config.add') :
        id === 'hotspot-expand' ? 'wifi.change' : id.slice(4)];
    assert.ok(name, 'Missing accessible name for ' + id);
    const comparison = id === 'language' ? `contains(normalize-space(.),${JSON.stringify(name)})` :
      `normalize-space(.)=${JSON.stringify(name)}`;
    return By.xpath(`//*[@role="button" and ${comparison}]`);
  }
  return By.css(selector(id));
}
async function element(id) {
  return driver.wait(until.elementLocated(await locator(id)), 40000,
    'Missing Flutter accessible control: ' + id);
}

async function click(id) {
  const started = Date.now();
  let previousRect;
  let centered = false;
  const target = await locator(id);
  const node = await driver.wait(async () => {
    const matches = await driver.findElements(target);
    if (!matches.length) return false;
    let candidate = matches[0];
    if (!(await driver.executeScript('return arguments[0].hasAttribute("flt-tappable")', candidate))) {
      const children = await candidate.findElements(By.css('[flt-tappable]'));
      if (!children.length) return false;
      candidate = children[0];
    }
    if ((await candidate.getAttribute('aria-disabled')) === 'true') return false;
    // Center the real semantic control before WebDriver clicks. Otherwise its
    // automatic edge scrolling can race Flutter's canvas position update.
    if (!centered) {
      await driver.executeScript('arguments[0].scrollIntoView({block:"center",inline:"nearest"})', candidate);
      centered = true;
      return false;
    }
    const rect = JSON.stringify(await candidate.getRect());
    if (rect !== previousRect) { previousRect = rect; return false; }
    return candidate;
  }, 20000, 'Flutter control did not become interactive: ' + id);
  await node.click();
  console.log('Clicked ' + id + ' in ' + (Date.now() - started) + 'ms');
}

async function fill(id, value) {
  const started = Date.now();
  const node = await element(id);
  const tag = await node.getTagName();
  const input = ['input', 'textarea'].includes(tag) ? node : await node.findElement(By.css('input,textarea'));
  await driver.wait(until.elementIsEnabled(input), 15000);
  await input.click();
  await input.sendKeys(Key.chord(process.platform === 'darwin' ? Key.COMMAND : Key.CONTROL, 'a'));
  await input.sendKeys(Key.BACK_SPACE, value);
  console.log('Filled ' + id + ' in ' + (Date.now() - started) + 'ms');
}

async function waitState(predicate, description) {
  await driver.wait(async () => predicate(await state()), 15000, description);
}

async function open() {
  const started = Date.now();
  await driver.get(base + '/');
  await element('login-submit');
  console.log('Opened sign in in ' + (Date.now() - started) + 'ms');
}

async function signIn(value) {
  await fill('login-password', value);
  await click('login-submit');
}

BeforeAll({ timeout: 180000 }, async () => {
  assert.ok(fs.existsSync(path.join(repo, 'internal/panel/flutter/main.dart.js')),
    'Build and stage the current Flutter web app with bash scripts/build-ui.sh web first.');
  binary = path.join(os.tmpdir(), 'caspian-flutter-bdd-' + process.pid);
  sourceHash = createHash('sha256').update(fs.readFileSync(
    path.join(repo, 'internal/panel/flutter/main.dart.js'))).digest('hex');
  console.log('Building Flutter browser fixture');
  const build = spawnSync('go', ['build', '-tags', 'flutterui', '-o', binary, './bdd/harness'], {
    cwd: repo, encoding: 'utf8', timeout: 120000,
  });
  assert.equal(build.status, 0, build.stderr || String(build.error));
  server = spawn(binary, ['-addr', '127.0.0.1:0'], { cwd: repo, stdio: ['ignore', 'pipe', 'pipe'] });
  let errors = '';
  server.stderr.on('data', data => { errors += data; });
  base = await new Promise((resolve, reject) => {
    const lines = createInterface({ input: server.stdout });
    startupTimer = setTimeout(() => reject(new Error('Fixture did not start: ' + errors)), 30000);
    lines.on('line', line => {
      const message = JSON.parse(line);
      if (message.url) { clearTimeout(startupTimer); lines.close(); resolve(message.url); }
    });
    server.once('error', reject);
    server.once('exit', code => reject(new Error('Fixture exited ' + code + ': ' + errors)));
  });
  const servedSource = await fetch(base + '/main.dart.js');
  assert.equal(servedSource.status, 200);
  const servedHash = createHash('sha256').update(Buffer.from(await servedSource.arrayBuffer())).digest('hex');
  assert.equal(servedHash, sourceHash, 'Fixture did not serve the staged Flutter build');
  console.log('Verified served Flutter main.dart.js SHA-256: ' + sourceHash);
});

async function startBrowser() {
  console.log('Starting headless Chrome');
  const options = new chrome.Options().addArguments('--headless=new', '--no-sandbox',
    '--disable-dev-shm-usage', '--window-size=1440,1100');
  if (process.env.CHROME_BINARY) options.setChromeBinaryPath(process.env.CHROME_BINARY);
  const preferences = new logging.Preferences();
  preferences.setLevel(logging.Type.BROWSER, logging.Level.ALL);
  options.setLoggingPrefs(preferences);
  const builder = new Builder().forBrowser('chrome').setChromeOptions(options);
  if (process.env.CHROMEDRIVER) builder.setChromeService(new chrome.ServiceBuilder(process.env.CHROMEDRIVER));
  driver = await builder.build();
  console.log('Headless Chrome session created');
  await driver.manage().setTimeouts({ script: 20000, pageLoad: 30000 });
  console.log('Browser timeouts configured');
}

Before(async function () {
  await this.attach(JSON.stringify({ stagedAndServedMainSha256: sourceHash }), 'application/json');
  this.response = undefined;
  // Each scenario owns its browser. Navigation, logout and service restarts
  // inside a scenario still use the same browser and session.
  await startBrowser();
  console.log('Resetting temporary appliance');
  await control('reset', { defect: process.env.CASPIAN_BDD_DEFECT || '' });
});

AfterStep(function ({ pickleStep, result }) {
  console.log(result.status + ': ' + pickleStep.text);
  if (result.status === Status.FAILED) console.log(result.message || 'Step failed');
});

// Cucumber runs After hooks in reverse registration order. Collect evidence
// first, then close even when an assertion in that hook fails.
After(async function () {
  try { if (driver) await driver.quit(); } finally { driver = undefined; }
});

After(async function (scenario) {
  const browserLogs = driver ? await driver.manage().logs().get(logging.Type.BROWSER) : [];
  if (scenario.result?.status === Status.FAILED && driver) {
    const save = (suffix, data) => {
      if (process.env.CASPIAN_BDD_REPORT_DIR) fs.writeFileSync(path.join(
        process.env.CASPIAN_BDD_REPORT_DIR, scenario.pickle.id + suffix), data);
    };
    try {
      const screenshot = Buffer.from(await driver.takeScreenshot(), 'base64');
      save('.png', screenshot); await this.attach(screenshot, 'image/png');
    } catch (_) {}
    await this.attach(JSON.stringify(browserLogs), 'application/json');
    try {
      const source = await driver.getPageSource();
      save('.html', source); await this.attach(source, 'text/html');
    } catch (_) {}
    try {
      const snapshot = await state();
      console.log('Failed scenario API view: ' + snapshot.view);
      console.log('Failed scenario semantic identifiers: ' + JSON.stringify(await driver.executeScript(
        'return Array.from(document.querySelectorAll("[flt-semantics-identifier]"), e => e.getAttribute("flt-semantics-identifier"))')));
    } catch (_) {}
  }
  if (scenario.result?.status === Status.PASSED && driver) {
    const resources = await driver.executeScript(
      'return performance.getEntriesByType("resource").map(entry => entry.name)');
    const external = resources.filter(url => /^https?:/.test(url) && new URL(url).origin !== base);
    assert.deepEqual(external, [], 'Flutter requested external resources');
    const assetErrors = browserLogs.filter(entry =>
      /font|manifest|content security policy/i.test(entry.message) &&
      /404|failed|violat|not found/i.test(entry.message));
    await this.attach(JSON.stringify({ resources, browserLogs }), 'application/json');
    const examples = [...new Set(assetErrors.map(entry => entry.message))].slice(0, 3);
    assert.equal(assetErrors.length, 0,
      'Flutter asset/CSP errors (' + assetErrors.length + '): ' + examples.join('\n'));
  }
});

AfterAll({ timeout: 30000 }, async () => {
  clearTimeout(startupTimer);
  try { if (driver) await driver.quit(); } finally {
    if (server) server.kill('SIGTERM');
    if (binary) fs.rmSync(binary, { force: true });
  }
});

Given('I open Caspian', open);
Given('I am signed in to Caspian', async () => {
  await open(); await signIn(password); await element('power');
});
Given('Caspian has no password', async () => {
  await control('unconfigured'); await driver.get(base + '/'); await element('setup-submit');
});
Given('Caspian is a new appliance', async () => {
  await control('first-run'); await driver.get(base + '/'); await element('setup-submit');
});
Given('the privileged service refuses to start', () => control('state', { start_fault: true }));

When('I create a Caspian password', async () => {
  await fill('setup-password', password); await fill('setup-confirm', password); await click('setup-submit');
});
When('I submit different setup passwords', async () => {
  await fill('setup-password', password); await fill('setup-confirm', password + '-different'); await click('setup-submit');
});
When('I sign in with the correct password', () => signIn(password));
When('I sign in with an incorrect password', () => signIn('incorrect-fictional-password'));
When('I sign out of Caspian', () => click('logout'));
When('the Caspian service restarts', async () => {
  await control('restart'); await driver.get(base + '/'); await element('login-submit');
});

When('I save a valid proxy configuration named {string}', async name => {
  await click('nav-config.heading');
  if (!(await driver.findElements(By.css(selector('config-config')))).length) await click('config-expand');
  const link = 'vless://' + '11111111-2222-4333-8444-555555555555' + '@example.invalid:443?type=tcp&security=none';
  await fill('config-config', link); await fill('config-label', name); await click('config-submit');
  await waitState(s => s.page.ConfigName === name, 'Saved proxy configuration was not returned by the backend');
});
When('I set the hotspot name to {string} and password to {string}', async (ssid, passphrase) => {
  await click('nav-wifi.heading');
  if (!(await driver.findElements(By.css(selector('hotspot-ssid')))).length) await click('hotspot-expand');
  await fill('hotspot-ssid', ssid); await fill('hotspot-passphrase', passphrase); await click('hotspot-submit');
  await waitState(s => s.page.SSID === ssid, 'Saved hotspot name was not returned by the backend');
});
When('I submit an invalid proxy configuration', async () => {
  await click('nav-config.heading'); await click('config-expand');
  await fill('config-config', 'this is not a proxy configuration'); await click('config-submit');
});
When('I switch Caspian on', async () => { await click('nav-status.heading'); await click('power'); });
When('I switch Caspian off', async () => { await click('nav-status.heading'); await click('power'); });
When('I toggle traffic', () => click('traffic'));
When('I select the Persian language', async () => {
  await click('language');
  const node = await driver.wait(until.elementLocated(By.xpath('//*[@role="menuitem" and (contains(.,"فارسی") or contains(@aria-label,"فارسی"))]')), 10000);
  await node.click();
});
When('I select the English language', async () => {
  await click('language');
  const node = await driver.wait(until.elementLocated(By.xpath('//*[@role="menuitem" and (contains(.,"English") or contains(@aria-label,"English"))]')), 10000);
  await node.click();
});
When('the browser submits power without a CSRF token', async function () {
  this.response = await browserRequest('/api/v1/power', { on: '1' });
});
When('a simulated device joins the hotspot', () => control('state', { devices: 1 }));
When('I finish the connection setup checklist', () => click('checklist-ready'));

Then('the browser offers no operating system installation controls', async () => {
  assert.equal((await driver.findElements(By.css('[flt-semantics-identifier^="onboarding-"]'))).length, 0);
  assert.equal((await driver.findElements(By.xpath('//*[@role="button" and (contains(.,"Install services") or contains(.,"Install and continue") or contains(@aria-label,"Install services") or contains(@aria-label,"Install and continue"))]'))).length, 0);
});
Then('the connection setup checklist is visible', async () => {
  for (const step of ['config', 'hotspot', 'start', 'join']) await element('checklist-' + step);
});
Then('setup does not claim the connection is ready', async () => {
  assert.equal((await driver.findElements(By.css(selector('checklist-ready')))).length, 0);
});
Then('setup confirms the connection is ready', async () => {
  await element('checklist-ready');
  const current = await state();
  assert.equal(current.page.Running, true);
  assert.equal(current.page.Connected, true);
  assert.ok(current.page.DeviceCount > 0);
});

Then('the Caspian dashboard is visible', async () => {
  await element('power'); await element('logout');
  assert.equal((await state()).view, 'dashboard');
});
Then('the Caspian sign in form is visible', () => element('login-submit'));
Then('the Caspian setup form is visible', () => element('setup-submit'));
Then('the browser cannot read authenticated settings', async () => {
  const current = await state();
  assert.equal(current.view, 'login');
  assert.ok(!current.page.Passphrase, 'Unauthenticated API exposed hotspot password');
  assert.ok(!current.page.ConfigSummary, 'Unauthenticated API exposed saved configuration');
});
Then('the application reports a problem', async () => {
  await driver.wait(() => driver.executeScript(function (target) {
    const node = document.querySelector(target);
    return node && ((node.textContent || '') + ' ' + (node.getAttribute('aria-label') || '')).trim();
  }, selector('problem')), 15000, 'Problem region is absent or empty');
});
Then('the backend stores configuration {string} and hotspot {string}', async (name, ssid) => {
  const current = await state(); assert.equal(current.page.ConfigName, name); assert.equal(current.page.SSID, ssid);
});
Then('the backend retains the original configuration', async () => assert.equal((await state()).page.ConfigName, 'Home'));
Then('the gateway is running', () => waitState(s => s.page.Running === true, 'Gateway did not start'));
Then('the gateway is stopped', () => waitState(s => s.page.Running === false && s.page.Connected === false, 'Gateway still claims to run'));
Then('traffic is cut', () => waitState(s => s.page.TrafficCut === true, 'Traffic was not cut'));
Then('traffic is restored', () => waitState(s => s.page.TrafficCut === false, 'Traffic was not restored'));
Then('the request is forbidden', function () { assert.equal(this.response.status, 403); });

async function assertLanguage(language, direction) {
  await driver.wait(async () => {
    const rail = await (await element('nav-status.heading')).getRect();
    const power = await (await element('power')).getRect();
    return direction === 'rtl' ? rail.x > power.x : rail.x < power.x;
  }, 15000, 'Rendered application did not move its navigation for ' + direction);
  const response = await browserRequest('/api/v1/state?lang=' + language);
  assert.equal(response.body.page.Lang, language);
  assert.equal(response.body.page.Dir, direction);
  const power = await element('power');
  const label = (await power.getAttribute('aria-label') || '') + ' ' + (await power.getText());
  assert.ok(label?.includes(response.body.strings['power.on']), 'Power label did not change to the selected language');
}
Then('the application uses Persian and right-to-left text', () => assertLanguage('fa', 'rtl'));
Then('the application uses English and left-to-right text', () => assertLanguage('en', 'ltr'));
