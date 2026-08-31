// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh
//
// The step definitions for the HTTP suite.
//
// The vocabulary is apickli's, on purpose, so that a reader who knows the
// project this suite is modelled on reads these feature files without a second
// thought. The library itself is not used, and support/world.js says why with
// the numbers.
//
// The panel-specific steps are the ones apickli has no vocabulary for: signing
// in through a form, carrying a session cookie, and scraping the per-session
// CSRF token out of rendered HTML because that is the only way a client can get
// one.

'use strict';

const assert = require('node:assert/strict');
const { Given, When, Then } = require('@cucumber/cucumber');

// ---------------------------------------------------------------------------
// Given: the request, and the state of the appliance
// ---------------------------------------------------------------------------

Given('I set headers to', async function (table) {
  for (const row of table.hashes()) {
    this.headers[row.name] = row.value;
  }
});

Given('I am signed in as the panel owner', async function () {
  const res = await this.signIn(process.env.CASPIAN_PASSWORD || 'correct-horse-battery');
  assert.equal(
    res.status, 303,
    'signing in answered ' + res.status + ' rather than a redirect, so the rest of this ' +
      'scenario would be testing a signed out client'
  );
});

Given('I have a form token from {string}', async function (path) {
  await this.tokenOn(path);
});

Given('the appliance is switched on and carrying traffic', async function () {
  await this.setState({ engine: 'running', hotspot: true, ssid: 'Caspian-test' });
});

Given('the appliance is switched off', async function () {
  await this.setState({ engine: 'stopped', hotspot: false });
});

Given('client traffic has been cut', async function () {
  await this.setState({ cut: true });
});

// ---------------------------------------------------------------------------
// When: the request
// ---------------------------------------------------------------------------

When('I GET {string}', async function (path) {
  await this.get(path);
});

When('I POST to {string} with no form token', async function (path) {
  await this.postForm(path, { on: '1' });
});

When('I POST to {string} with the form token and', async function (path, table) {
  const fields = { csrf: this.csrf };
  for (const row of table.hashes()) {
    fields[row.name] = row.value;
  }
  await this.postForm(path, fields);
});

When('I POST to {string} with the wrong password', async function (path) {
  const token = await this.tokenOn('/login');
  await this.postForm(path, { csrf: token, password: 'correct-horse-batteries' });
});

// ---------------------------------------------------------------------------
// Then: the response, in apickli's words
// ---------------------------------------------------------------------------

Then('response code should be {int}', async function (code) {
  assert.equal(
    this.response.status, code,
    'the response body began ' + JSON.stringify(this.responseBody.slice(0, 200))
  );
});

Then('response header {string} should be {string}', async function (name, value) {
  assert.equal(this.response.headers.get(name), value, 'the ' + name + ' response header');
});

Then('response header {string} should contain {string}', async function (name, value) {
  const got = this.response.headers.get(name) || '';
  assert.ok(got.includes(value), 'the ' + name + ' header is ' + JSON.stringify(got));
});

Then('response body should be valid json', async function () {
  this.json();
});

Then('response body path {string} should be {string}', async function (path, expected) {
  const got = path.split('.').reduce((o, k) => (o === undefined || o === null ? o : o[k]), this.json());
  assert.equal(String(got), expected, 'the value at ' + path);
});

Then('response body path {string} should be the boolean {word}', async function (path, expected) {
  const got = path.split('.').reduce((o, k) => (o === undefined || o === null ? o : o[k]), this.json());
  assert.equal(typeof got, 'boolean', 'the value at ' + path + ' is ' + JSON.stringify(got));
  assert.equal(got, expected === 'true', 'the value at ' + path);
});

Then('response body path {string} should be a number', async function (path) {
  const got = path.split('.').reduce((o, k) => (o === undefined || o === null ? o : o[k]), this.json());
  assert.equal(typeof got, 'number', 'the value at ' + path + ' is ' + JSON.stringify(got));
});

Then('response body should contain {string}', async function (text) {
  assert.ok(
    this.responseBody.includes(text),
    'the response body does not contain ' + JSON.stringify(text) +
      '. It began ' + JSON.stringify(this.responseBody.slice(0, 200))
  );
});

Then('response body should not contain {string}', async function (text) {
  assert.ok(
    !this.responseBody.includes(text),
    'the response body contains ' + JSON.stringify(text) + ', and it must not'
  );
});

Then('response body should carry the message {string}', async function (key) {
  const expected = this.msg(key);
  assert.ok(
    this.responseBody.includes(expected),
    'the response does not carry ' + JSON.stringify(expected) + ' for the key ' + key
  );
});

Then('response body should have the keys', async function (table) {
  const body = this.json();
  const missing = [];
  for (const row of table.raw()) {
    const key = row[0];
    if (!(key in body)) {
      missing.push(key);
    }
  }
  assert.deepEqual(
    missing, [],
    'the response is missing ' + missing.join(', ') + '. It carried: ' + Object.keys(body).join(', ')
  );
});

Then('the hero class should be one of the three the panel serves', async function () {
  // Measured 2026-08-30 across all 32 combinations of engine phase, hotspot up
  // or down, traffic cut or not, and the privileged status call failing or not:
  // the panel served "ok", "off" and "cut", and never "wait". heroClass in
  // internal/panel/words.go can return "wait", and nothing that reaches a
  // client ever does.
  //
  // This step asserts the MEASURED set. If a change ever makes the waiting
  // state reachable, this fails, and the person who made it should widen the
  // set and delete this paragraph.
  const served = ['ok', 'off', 'cut'];
  const got = this.json().heroClass;
  assert.ok(
    served.includes(got),
    'status.json carried heroClass ' + JSON.stringify(got) + '. The measured set the panel ' +
      'serves is ' + served.join(', ') + '.'
  );
});

Then('no credential should appear anywhere in the response', async function () {
  // The panel's absolute rule: no response body and no log line ever carries
  // the pasted config, the hotspot passphrase or the panel password. The
  // parts, not only the whole, because "the config was not echoed" and "no
  // piece of the config was echoed" are different claims and the second is the
  // one that matters.
  const secrets = [
    'correct-horse-battery',
    'sun-rope-glass-mint',
    '11111111-2222-4333-8444-555555555555',
    '0a1b2c3d4e',
    'Q0FTUElBTi1GQUtFLVJFQUxJVFktUFVCS0VZLTMyMzI',
  ];
  for (const secret of secrets) {
    assert.ok(
      !this.responseBody.includes(secret),
      'the response carries ' + JSON.stringify(secret)
    );
  }
});
