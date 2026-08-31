// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh
//
// The world for the HTTP suite: one request at a time, and the response it got.
//
// # Why this is not apickli
//
// The project this suite is modelled on drives its API features through
// apickli, and the step vocabulary below is deliberately apickli's, so that a
// reader who knows that project reads these feature files without a second
// thought: "I set headers to", "response code should be", "response body path
// ... should be".
//
// The library itself is not used, for two measured reasons.
//
// Installing it on 2026-08-30 pulled 394 packages and npm reported 15
// vulnerabilities, 4 of them critical, through request, superagent 3.8.3,
// multer 1.4.4 and core-js 2. The same install without it is 104 packages and
// npm reports none. That is a poor trade for a project whose whole posture is
// that it fetches nothing it does not need.
//
// The second reason is that it would not do the job. Every state-changing route
// on this panel is behind a session cookie AND a per-session CSRF token that
// has to be scraped out of the rendered HTML of the form. apickli's canned
// Gherkin is built for JSON request and response bodies and has no vocabulary
// for that, so the interesting half of these scenarios would have had to be
// hand-written anyway.
//
// # The cookie jar
//
// Written out rather than pulled in, and it is fifteen lines. Node's fetch does
// not keep cookies between calls by design, so the jar is one Map from name to
// value and a header built from it. That is enough for a single-origin suite
// talking to one panel, and being small it is also readable, which matters more
// here than generality.

'use strict';

const { setWorldConstructor, setDefaultTimeout, World } = require('@cucumber/cucumber');

setDefaultTimeout(60 * 1000);

class CaspianApiWorld extends World {
  constructor(options) {
    super(options);
    this.base = null;
    this.messages = { fa: {}, en: {} };

    // The request being built up, and the response that came back.
    this.headers = {};
    this.body = null;
    this.response = null;
    this.responseBody = '';

    this.cookies = new Map();
    // The CSRF token most recently scraped from a rendered form.
    this.csrf = '';
  }

  url(path) {
    return this.base + path;
  }

  msg(key, lang) {
    const table = this.messages[lang || 'fa'];
    if (!table || !(key in table)) {
      throw new Error(
        'the harness did not export the message key "' + key + '". ' +
        'Add it to exportedKeys in bdd/harness/main.go.'
      );
    }
    return table[key];
  }

  // ---- the cookie jar -----------------------------------------------------

  cookieHeader() {
    const pairs = [];
    for (const [name, value] of this.cookies) {
      pairs.push(name + '=' + value);
    }
    return pairs.join('; ');
  }

  rememberCookies(res) {
    // getSetCookie returns every Set-Cookie header separately, which a single
    // get('set-cookie') would have joined into one unparseable string.
    const raw = typeof res.headers.getSetCookie === 'function' ? res.headers.getSetCookie() : [];
    for (const line of raw) {
      const [pair] = line.split(';');
      const eq = pair.indexOf('=');
      if (eq < 0) continue;
      const name = pair.slice(0, eq).trim();
      const value = pair.slice(eq + 1).trim();
      if (value === '' || /Max-Age=0/i.test(line)) {
        this.cookies.delete(name);
      } else {
        this.cookies.set(name, value);
      }
    }
  }

  // ---- requests -----------------------------------------------------------

  async request(method, path, body) {
    const headers = Object.assign({}, this.headers);
    const jar = this.cookieHeader();
    if (jar) {
      headers.Cookie = jar;
    }
    const init = { method, headers, redirect: 'manual' };
    if (body !== undefined && body !== null) {
      init.body = body;
    }
    const res = await fetch(this.url(path), init);
    this.rememberCookies(res);
    this.response = res;
    this.responseBody = await res.text();
    return res;
  }

  async get(path) {
    return this.request('GET', path, null);
  }

  async postForm(path, fields) {
    const form = new URLSearchParams();
    for (const [k, v] of Object.entries(fields)) {
      form.set(k, String(v));
    }
    const saved = this.headers['Content-Type'];
    this.headers['Content-Type'] = 'application/x-www-form-urlencoded';
    try {
      return await this.request('POST', path, form.toString());
    } finally {
      if (saved === undefined) {
        delete this.headers['Content-Type'];
      } else {
        this.headers['Content-Type'] = saved;
      }
    }
  }

  // json parses the response body, and says what it actually got when it is
  // not JSON. "Unexpected token <" tells a reader nothing about which endpoint
  // answered with a login page.
  json() {
    try {
      return JSON.parse(this.responseBody);
    } catch (e) {
      throw new Error(
        'the response body is not JSON. It began: ' +
        JSON.stringify(this.responseBody.slice(0, 200))
      );
    }
  }

  // ---- the panel's own doors ---------------------------------------------

  // tokenOn scrapes the CSRF token out of a rendered page, which is the only
  // way a client can get one. Reaching into the session store for it would be
  // checking that this panel's reader agrees with its own writer.
  async tokenOn(path) {
    await this.get(path);
    const m = this.responseBody.match(/name="csrf" value="([^"]*)"/);
    if (!m) {
      throw new Error('no CSRF token in the page at ' + path);
    }
    if (!m[1]) {
      throw new Error('the CSRF token on ' + path + ' is empty');
    }
    this.csrf = m[1];
    return this.csrf;
  }

  async signIn(password) {
    const token = await this.tokenOn('/login');
    return this.postForm('/login', { csrf: token, password });
  }

  // ---- driving the appliance underneath ----------------------------------

  async control(path, body) {
    const res = await fetch(this.url('/__control/' + path), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body || {}),
    });
    if (!res.ok) {
      throw new Error('control ' + path + ' answered ' + res.status + ': ' + (await res.text()));
    }
  }

  // The defect is passed in rather than read here, because in a mutation run it
  // depends on which scenario is about to run. See bdd/mutation-support.js.
  async reset(defect) {
    await this.control('reset', { defect: defect || '' });
    this.cookies.clear();
    this.csrf = '';
    this.headers = {};
    this.response = null;
    this.responseBody = '';
  }

  async setState(state) {
    await this.control('state', state);
  }
}

setWorldConstructor(CaspianApiWorld);

module.exports = { CaspianApiWorld };
