// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh
//
// The world every step definition runs against.
//
// It holds three things: the browser, the address of the appliance under test,
// and the message catalogue the panel actually ships. The last of those is the
// one worth explaining.
//
// Assertions here never compare against a Persian or English string typed into
// JavaScript. They compare against the words the Go catalogue resolves, fetched
// from the harness at the start of the run. A copy of the catalogue in this
// directory would be a second source of truth that nothing keeps in step, and
// the first reworded message would turn scenarios red for a reason that has
// nothing to do with the panel being broken. This way a rewording is invisible
// to the suite and a page drawn in the wrong language is not.

'use strict';

const { setWorldConstructor, setDefaultTimeout, World } = require('@cucumber/cucumber');
const { By, until, Key } = require('selenium-webdriver');

// Chrome is started once for the whole run and the session is cleared between
// scenarios; see hooks.js for why that is safe and what it would cost to get
// wrong. Ninety seconds is the bound on one scenario, not on the run.
setDefaultTimeout(90 * 1000);

class CaspianWorld extends World {
  constructor(options) {
    super(options);
    // Filled in by the hooks. Kept on the world rather than in module scope so
    // that a step reads this.driver and not a global that could be from the
    // previous run.
    this.driver = null;
    this.base = null;
    this.messages = { fa: {}, en: {} };
    // The language the scenario believes the page is in. It starts at the
    // product default and moves when a step chooses the other one, so an
    // assertion holds in whichever language the page is actually in.
    this.lang = 'fa';
  }

  // ---- addresses ----------------------------------------------------------

  url(path) {
    return this.base + path;
  }

  // ---- the message catalogue ---------------------------------------------

  msg(key) {
    const table = this.messages[this.lang];
    if (!table || !(key in table)) {
      throw new Error(
        'the harness did not export the message key "' + key + '" for language ' + this.lang +
        '. Add it to exportedKeys in bdd/harness/main.go.'
      );
    }
    return table[key];
  }

  msgIn(lang, key) {
    const table = this.messages[lang];
    if (!table || !(key in table)) {
      throw new Error('no message "' + key + '" for language ' + lang);
    }
    return table[key];
  }

  // ---- driving the appliance underneath ----------------------------------

  async control(path, body) {
    const res = await fetch(this.url('/__control/' + path), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body || {}),
    });
    const text = await res.text();
    if (!res.ok) {
      throw new Error('control ' + path + ' answered ' + res.status + ': ' + text);
    }
    return text;
  }

  // reset throws the appliance away and builds a new one: set up, with a
  // hotspot and a config, switched off.
  //
  // The defect is passed in rather than read here, because in a mutation run it
  // depends on which scenario is about to run. See bdd/mutation-support.js.
  async reset(defect) {
    await this.control('reset', { defect: defect || '' });
  }

  async setState(state) {
    await this.control('state', state);
  }

  // ---- the browser --------------------------------------------------------

  async goto(path) {
    await this.driver.get(this.url(path));
  }

  async find(selector) {
    return this.driver.wait(until.elementLocated(By.css(selector)), 15 * 1000);
  }

  async present(selector) {
    const found = await this.driver.findElements(By.css(selector));
    return found.length > 0;
  }

  async text(selector) {
    const el = await this.find(selector);
    return (await el.getText()).trim();
  }

  async attr(selector, name) {
    const el = await this.find(selector);
    return el.getAttribute(name);
  }

  async click(selector) {
    const el = await this.find(selector);
    await el.click();
  }

  // css returns a COMPUTED property, which is the whole reason this suite
  // exists. A class name says what the page intended; this says what the
  // browser drew after the cascade had its say.
  async css(selector, property) {
    const el = await this.find(selector);
    return el.getCssValue(property);
  }

  // palette resolves a custom property to the colour it actually paints.
  //
  // getPropertyValue on a custom property hands back the SPECIFIED value, so
  // --accent-on comes back as the literal text "var(--sage)" and comparing it
  // to a computed rgb() string would fail for the wrong reason. The way to get
  // the resolved colour is to make the browser resolve it: paint it on a
  // throwaway element and read the computed background back off that.
  async palette(token) {
    return this.driver.executeScript(
      'const probe = document.createElement("div");' +
      'probe.style.position = "absolute";' +
      'probe.style.visibility = "hidden";' +
      'probe.style.background = "var(" + arguments[0] + ")";' +
      'document.body.appendChild(probe);' +
      'const paint = getComputedStyle(probe).backgroundColor;' +
      'probe.remove();' +
      'return paint;',
      token
    );
  }

  // signIn goes through the real form, in the real browser, exactly as a person
  // would. Nothing here reaches into the session store or forges a cookie: a
  // cookie no browser would send back would pass such a test, which is the same
  // argument internal/panel/panel_test.go makes about its cookie jar.
  async signIn(password) {
    await this.goto('/login');
    const field = await this.find('#password');
    await field.clear();
    await field.sendKeys(password);
    await this.click('form[action="/login"] button[type="submit"]');
  }

  async signedIn() {
    return this.present('#hero');
  }
}

setWorldConstructor(CaspianWorld);

module.exports = { CaspianWorld, By, until, Key };
