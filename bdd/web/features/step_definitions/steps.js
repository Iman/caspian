// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh
//
// The step definitions for the browser suite.
//
// Two rules run through all of them.
//
// First, a step asserts what the BROWSER SHOWS, not what the HTML said. That is
// the whole reason this suite exists next to internal/panel's own tests, which
// already go through a real server, a real cookie jar and the real markup. The
// defect that produced this suite, commit 5c51497, left every string and every
// class correct and painted the wrong colour, because a later rule at equal
// specificity won the cascade. Anything that compares markup is blind to it.
//
// Second, a step never compares against a Persian or English string written
// here. It compares against the catalogue the panel actually ships, fetched
// from the harness at the start of the run. See the note in support/world.js.

'use strict';

const assert = require('node:assert/strict');
const zlib = require('node:zlib');
const { Given, When, Then } = require('@cucumber/cucumber');
const { By, until } = require('selenium-webdriver');

// The palette roles the feature files name, and the custom property each one
// resolves to. The words are the ones the stylesheet's own comments use for
// them, so a reader of panel.css and a reader of a feature file are talking
// about the same thing.
//
// No hex value appears in this suite. The step resolves the token in the
// browser and compares the two computed colours, so changing the palette does
// not break a scenario and a broken cascade still does.
const PALETTE = {
  green: '--accent-on',
  coral: '--accent-warn',
  yellow: '--accent-note',
  'page ground': '--ground',
};

function tokenFor(name) {
  const token = PALETTE[name];
  if (!token) {
    throw new Error(
      'no palette role called "' + name + '". Known roles: ' + Object.keys(PALETTE).join(', ')
    );
  }
  return token;
}

// rgbChannels turns "rgb(161, 204, 166)" or "rgba(161, 204, 166, 1)" into
// numbers, so a colour can be compared as a colour rather than as a string.
// Chrome returns rgb() for an opaque colour and rgba() when an alpha is
// involved, and the two spellings of the same paint must not disagree.
function rgbChannels(value) {
  const m = String(value).match(/rgba?\(([^)]+)\)/);
  if (!m) {
    throw new Error('not a colour this suite can read: ' + value);
  }
  const parts = m[1].split(',').map((p) => parseFloat(p.trim()));
  return { r: parts[0], g: parts[1], b: parts[2], a: parts.length > 3 ? parts[3] : 1 };
}

function sameColour(a, b) {
  const x = rgbChannels(a);
  const y = rgbChannels(b);
  return x.r === y.r && x.g === y.g && x.b === y.b && x.a === y.a;
}

// ---------------------------------------------------------------------------
// Given: the state of the appliance
// ---------------------------------------------------------------------------

Given('a Caspian panel that has been set up', async function () {
  // The hooks already built one. This step exists so the feature file says out
  // loud what it starts from, and it checks the premise rather than assuming
  // it: a harness that came up wrong should fail here, with this sentence, and
  // not three steps later inside an assertion about a colour.
  const res = await fetch(this.url('/__control/health'));
  assert.equal(res.status, 200, 'the appliance under test did not answer');
});

Given('I am signed in', async function () {
  await this.signIn(process.env.CASPIAN_PASSWORD || 'correct-horse-battery');
  await this.driver.wait(until.elementLocated(By.css('#hero')), 15 * 1000);
});

Given('the box is switched off', async function () {
  await this.setState({ engine: 'stopped', hotspot: false });
});

Given('the box is switched on and carrying traffic', async function () {
  await this.setState({ engine: 'running', hotspot: true, ssid: 'Caspian-test' });
});

Given('client traffic has been cut', async function () {
  await this.setState({ cut: true });
});

Given('the hotspot reports {int} joined device(s)', async function (n) {
  await this.setState({ devices: n });
});

// ---------------------------------------------------------------------------
// When: what a person does
// ---------------------------------------------------------------------------

When('I open the sign-in page', async function () {
  await this.goto('/login');
});

When('I open the dashboard', async function () {
  await this.goto('/');
});

When('I open the dashboard with the advanced section showing', async function () {
  await this.goto('/?advanced=1');
});

When('I open the help page', async function () {
  await this.goto('/help');
});

When('I sign in with the wrong password', async function () {
  await this.signIn('correct-horse-batteries');
});

When('I sign in with the right password', async function () {
  await this.signIn(process.env.CASPIAN_PASSWORD || 'correct-horse-battery');
});

When('I choose the other language', async function () {
  // The link in the rail, clicked the way a person clicks it, rather than a
  // navigation to a URL this file made up. If the link ever stops pointing
  // where it should, this step fails, which is the point.
  await this.click('a[href^="/?lang="]');
  await this.driver.wait(until.elementLocated(By.css('#hero')), 15 * 1000);
  const lang = await this.attr('html', 'lang');
  this.lang = lang;
});

When('I press the power control', async function () {
  await this.click('#power-button');
  await this.driver.wait(until.elementLocated(By.css('#hero')), 15 * 1000);
});

When('I press the client traffic control', async function () {
  await this.click('#cut-button');
  await this.driver.wait(until.elementLocated(By.css('#hero')), 15 * 1000);
});

When('the control bar is put into the waiting state by hand', async function () {
  // READ THE COMMENT ABOVE THIS SCENARIO IN control_bar BEFORE COPYING THIS.
  //
  // This is the only step in the suite that reaches into the page instead of
  // driving it, and it is here because the waiting state is one the panel
  // cannot be asked to produce. It is named "by hand" so that no reader can
  // mistake what it does for something the product did.
  await this.driver.executeScript(
    'const hero = document.getElementById("hero");' +
    'hero.classList.remove("hero-ok", "hero-off", "hero-cut");' +
    'hero.classList.add("hero-wait");'
  );
});

// ---------------------------------------------------------------------------
// Then: signing in
// ---------------------------------------------------------------------------

Then('the sign-in page is still showing', async function () {
  const found = await this.present('#password');
  assert.ok(found, 'expected the password field to still be on the page');
});

Then('the page says the password is not right', async function () {
  const text = await this.text('.problem');
  const expected = this.msg('login.wrong.headline');
  assert.ok(
    text.includes(expected),
    'expected the page to carry ' + JSON.stringify(expected) + ', and it read ' + JSON.stringify(text)
  );
});

Then('the dashboard is not showing', async function () {
  const found = await this.present('#hero');
  assert.ok(!found, 'the control bar is on the page, so this browser reached the dashboard');
});

Then('the dashboard is showing', async function () {
  await this.driver.wait(until.elementLocated(By.css('#hero')), 15 * 1000);
});

Then('the control bar is on the page', async function () {
  const found = await this.present('#hero');
  assert.ok(found, 'no control bar on the page');
});

// ---------------------------------------------------------------------------
// Then: language and direction
// ---------------------------------------------------------------------------

Then('the page is drawn in {word}', async function (language) {
  const want = { Persian: 'fa', English: 'en' }[language];
  if (!want) {
    throw new Error('this suite knows Persian and English, not ' + language);
  }
  const got = await this.attr('html', 'lang');
  assert.equal(got, want, 'the document language attribute');
  this.lang = got;
});

Then('the page reads {word} to {word}', async function (from, to) {
  const want = from === 'right' && to === 'left' ? 'rtl' : 'ltr';
  const got = await this.attr('html', 'dir');
  assert.equal(got, want, 'the document direction attribute');

  // The attribute is what the template sets. This is what the browser did with
  // it, which is the claim that matters for a layout written in logical
  // properties.
  const computed = await this.css('body', 'direction');
  assert.equal(computed, want, 'the computed direction on the body');
});

Then('the power control carries the {word} word for it', async function (language) {
  const lang = { Persian: 'fa', English: 'en' }[language];
  const shown = await this.text('#power-label');
  const on = this.msgIn(lang, 'power.on');
  const off = this.msgIn(lang, 'power.off');
  assert.ok(
    shown === on || shown === off,
    'the power control reads ' + JSON.stringify(shown) +
      ', which is neither ' + JSON.stringify(on) + ' nor ' + JSON.stringify(off)
  );
});

// ---------------------------------------------------------------------------
// Then: the control bar
// ---------------------------------------------------------------------------

Then('the control bar carries the {string} state', async function (state) {
  const classes = await this.attr('#hero', 'class');
  assert.ok(
    classes.split(/\s+/).includes('hero-' + state),
    'the control bar is classed ' + JSON.stringify(classes) + ', with no hero-' + state
  );
});

Then('the control bar is painted the {} from the palette', async function (role) {
  const painted = await this.css('#hero', 'background-color');
  const expected = await this.palette(tokenFor(role));
  assert.ok(
    sameColour(painted, expected),
    'the control bar is painted ' + painted + ' and the ' + role + ' in the palette is ' +
      expected + '. The class can still be right when this is wrong: that is commit 5c51497.'
  );
});

Then('the control bar is not painted the page ground', async function () {
  const painted = await this.css('#hero', 'background-color');
  const ground = await this.palette('--ground');
  assert.ok(
    !sameColour(painted, ground),
    'the control bar is painted the page ground (' + painted + '), so the state says nothing. ' +
      'This is the exact defect commit 5c51497 removed: a background on .hero beats the four ' +
      '.hero-<state> rules because it is written after them at equal specificity.'
  );
});

Then('the control bar is animated by the cut pulse', async function () {
  const name = await this.css('#hero', 'animation-name');
  assert.equal(name, 'cut-pulse', 'the animation on the control bar');
});

Then('the control bar is painted between the coral and the yellow of the pulse', async function () {
  // The cut state animates between two measured grounds, so its computed colour
  // is a different value at every frame and cannot be compared to one constant.
  // What IS constant is that every frame lies on the segment between the two,
  // channel by channel. A flattened cut state, or one that fell through to the
  // page ground, leaves that box.
  const painted = rgbChannels(await this.css('#hero', 'background-color'));
  const coral = rgbChannels(await this.palette('--accent-warn'));
  const yellow = rgbChannels(await this.palette('--accent-note'));

  for (const channel of ['r', 'g', 'b']) {
    const low = Math.min(coral[channel], yellow[channel]);
    const high = Math.max(coral[channel], yellow[channel]);
    assert.ok(
      painted[channel] >= low - 1 && painted[channel] <= high + 1,
      'the ' + channel + ' channel of the control bar is ' + painted[channel] +
        ', outside the pulse range ' + low + ' to ' + high +
        '. The whole paint was rgb(' + painted.r + ', ' + painted.g + ', ' + painted.b + ').'
    );
  }
});

// ---------------------------------------------------------------------------
// Then: the two controls
// ---------------------------------------------------------------------------

Then('the power control offers to switch the box {word}', async function (direction) {
  const key = direction === 'on' ? 'power.on' : 'power.off';
  const shown = await this.text('#power-label');
  assert.equal(shown, this.msg(key), 'the word on the power control');

  // And the form posts the OPPOSITE of the state it is in, which is the part a
  // label alone cannot tell you.
  const value = await this.attr('#power-value', 'value');
  assert.equal(value, direction === 'on' ? '1' : '0', 'the value the power form would post');
});

Then('the client traffic control is a switch', async function () {
  const role = await this.attr('#cut-button', 'role');
  assert.equal(role, 'switch', 'the role of the client traffic control');
});

Then('the client traffic control has an accessible name', async function () {
  const label = await this.attr('#cut-button', 'aria-label');
  assert.ok(label && label.trim().length > 0, 'the control has no aria-label');
  const cut = this.msg('cut.button');
  const restore = this.msg('cut.restore');
  assert.ok(
    label === cut || label === restore,
    'the accessible name is ' + JSON.stringify(label) +
      ', which is neither ' + JSON.stringify(cut) + ' nor ' + JSON.stringify(restore)
  );
});

Then('the client traffic control is labelled with the thing it switches', async function () {
  const shown = await this.text('#cut-button .toggle-text');
  assert.equal(shown, this.msg('cut.switchlabel'), 'the visible label on the switch');
});

Then('the client traffic control reports that traffic is {word}', async function (word) {
  const want = word === 'flowing' ? 'true' : 'false';
  const checked = await this.attr('#cut-button', 'aria-checked');
  assert.equal(
    checked, want,
    'aria-checked on the client traffic control. On means traffic is flowing, ' +
      'which is the way round every switch a reader has used works.'
  );
});

// ---------------------------------------------------------------------------
// Then: the device count
// ---------------------------------------------------------------------------

Then('the device count reads {string}', async function (expected) {
  const shown = await this.text('#device-count');
  assert.equal(shown, expected, 'the number on the devices tile');
});

Then('the page does not claim any device is connected', async function () {
  const tile = await this.text('#device-count');
  const line = await this.text('#device-line');
  assert.equal(
    tile, '0',
    'the devices tile reads ' + JSON.stringify(tile) + ' while the appliance is switched off ' +
      'and the hotspot is down. Nothing can be joined to a network that is not being broadcast.'
  );
  assert.equal(
    line, this.msg('devices.none'),
    'the line under the hotspot card reads ' + JSON.stringify(line) +
      ' while the appliance is switched off. Expected ' + JSON.stringify(this.msg('devices.none'))
  );
});

// ---------------------------------------------------------------------------
// Then: the join code
// ---------------------------------------------------------------------------

Then('the join code is drawn', async function () {
  const found = await this.present('.qr-figure svg.qr');
  assert.ok(found, 'no join code on the page');
  const box = await (await this.find('.qr-figure svg.qr')).getRect();
  assert.ok(box.width > 40 && box.height > 40, 'the join code is ' + box.width + ' by ' + box.height);
});

// The three pixel assertions share one reading of the image, cached on the
// world for the scenario. Taking three screenshots of the same element would be
// three chances for the page to have moved underneath them.
async function qrPixels(world) {
  if (world._qr) {
    return world._qr;
  }
  const el = await world.find('.qr-figure svg.qr');
  const png = Buffer.from(await el.takeScreenshot(), 'base64');
  world._qr = decodePNG(png);
  return world._qr;
}

// decodePNG reads a truecolour or greyscale PNG far enough to sample pixels.
//
// Written out rather than taken from a package on purpose. It is about eighty
// lines, it has no dependencies, and the alternative is adding an image library
// to the dependency graph of a project whose whole posture is that it fetches
// nothing it does not need. Chrome writes 8-bit RGBA here; the other colour
// types are refused loudly rather than guessed at.
function decodePNG(buf) {
  const signature = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]);
  if (!buf.subarray(0, 8).equals(signature)) {
    throw new Error('the screenshot is not a PNG');
  }
  let offset = 8;
  let width = 0;
  let height = 0;
  let depth = 0;
  let colourType = 0;
  const idat = [];

  while (offset < buf.length) {
    const length = buf.readUInt32BE(offset);
    const type = buf.toString('ascii', offset + 4, offset + 8);
    const data = buf.subarray(offset + 8, offset + 8 + length);
    if (type === 'IHDR') {
      width = data.readUInt32BE(0);
      height = data.readUInt32BE(4);
      depth = data.readUInt8(8);
      colourType = data.readUInt8(9);
      if (data.readUInt8(12) !== 0) {
        throw new Error('this reader does not handle interlaced PNG');
      }
    } else if (type === 'IDAT') {
      idat.push(data);
    } else if (type === 'IEND') {
      break;
    }
    offset += 12 + length;
  }

  if (depth !== 8 || (colourType !== 6 && colourType !== 2)) {
    throw new Error('this reader handles 8-bit RGB and RGBA only, and got depth ' + depth + ' type ' + colourType);
  }
  const channels = colourType === 6 ? 4 : 3;
  const raw = zlib.inflateSync(Buffer.concat(idat));
  const stride = width * channels;
  const pixels = Buffer.alloc(height * stride);

  // Undo the per-scanline filters. This is the whole of the PNG filtering
  // specification for 8-bit samples, and it is five cases.
  let pos = 0;
  for (let y = 0; y < height; y++) {
    const filter = raw[pos];
    pos += 1;
    for (let x = 0; x < stride; x++) {
      const value = raw[pos + x];
      const left = x >= channels ? pixels[y * stride + x - channels] : 0;
      const up = y > 0 ? pixels[(y - 1) * stride + x] : 0;
      const upLeft = x >= channels && y > 0 ? pixels[(y - 1) * stride + x - channels] : 0;
      let out;
      switch (filter) {
        case 0: out = value; break;
        case 1: out = value + left; break;
        case 2: out = value + up; break;
        case 3: out = value + ((left + up) >> 1); break;
        case 4: {
          const p = left + up - upLeft;
          const pa = Math.abs(p - left);
          const pb = Math.abs(p - up);
          const pc = Math.abs(p - upLeft);
          const predictor = pa <= pb && pa <= pc ? left : pb <= pc ? up : upLeft;
          out = value + predictor;
          break;
        }
        default:
          throw new Error('unknown PNG scanline filter ' + filter);
      }
      pixels[y * stride + x] = out & 0xff;
    }
    pos += stride;
  }

  return {
    width,
    height,
    channels,
    at(x, y) {
      const i = y * stride + x * channels;
      return { r: pixels[i], g: pixels[i + 1], b: pixels[i + 2] };
    },
  };
}

function luminance(p) {
  // Rec. 601 luma, which is enough to sort black modules from white ones.
  return 0.299 * p.r + 0.587 * p.g + 0.114 * p.b;
}

Then('the join code has a light quiet zone', async function () {
  const img = await qrPixels(this);
  // The quiet zone is four modules on every side. The smallest symbol the panel
  // can produce is 21 modules, so the zone is at least 4/29 of the width, and
  // sampling two per cent in from a corner is safely inside it whatever version
  // the encoder chose.
  const inset = Math.max(1, Math.round(img.width * 0.02));
  const corners = [
    ['top left', inset, inset],
    ['top right', img.width - 1 - inset, inset],
    ['bottom left', inset, img.height - 1 - inset],
    ['bottom right', img.width - 1 - inset, img.height - 1 - inset],
  ];
  for (const [name, x, y] of corners) {
    const p = img.at(x, y);
    assert.ok(
      luminance(p) > 200,
      'the ' + name + ' of the quiet zone is rgb(' + p.r + ', ' + p.g + ', ' + p.b + '), which is not light. ' +
        'A dark quiet zone is a QR code no phone will lock on to, and the page still renders.'
    );
  }
});

Then('the join code is not a solid block', async function () {
  const img = await qrPixels(this);
  let dark = 0;
  let total = 0;
  for (let y = 0; y < img.height; y += 2) {
    for (let x = 0; x < img.width; x += 2) {
      total += 1;
      if (luminance(img.at(x, y)) < 128) {
        dark += 1;
      }
    }
  }
  const share = dark / total;
  assert.ok(
    share > 0.05 && share < 0.75,
    'the join code is ' + Math.round(share * 100) + ' per cent dark. A readable symbol is neither ' +
      'nearly all dark nor nearly all light, and this one is one of those.'
  );
});

Then('the join code carries both dark and light modules', async function () {
  const img = await qrPixels(this);
  let sawDark = false;
  let sawLight = false;
  for (let y = 0; y < img.height && !(sawDark && sawLight); y += 2) {
    for (let x = 0; x < img.width; x += 2) {
      const l = luminance(img.at(x, y));
      if (l < 128) sawDark = true;
      if (l > 200) sawLight = true;
    }
  }
  assert.ok(sawDark, 'the join code has no dark modules at all');
  assert.ok(sawLight, 'the join code has no light modules at all');
});

// ---------------------------------------------------------------------------
// Then: the advanced section and the help page
// ---------------------------------------------------------------------------

Then('the advanced section is showing', async function () {
  const found = await this.present('section.advanced');
  assert.ok(found, 'the advanced section is not on the page');
  const heading = await this.text('#advanced-heading');
  assert.equal(heading, this.msg('advanced.heading'), 'the advanced heading');
});

Then('the advanced section lists the interface {string}', async function (name) {
  const options = await this.driver.findElements(
    By.css('#internet_interface option, #hotspot_interface option')
  );
  const values = [];
  for (const option of options) {
    values.push(await option.getAttribute('value'));
  }
  assert.ok(
    values.includes(name),
    'the interface lists offer ' + JSON.stringify(values) + ', with no ' + JSON.stringify(name)
  );
});

// A GET from inside the page, so the status code is the one the panel actually
// answered with. The browser does not expose the status of the navigation it
// just made, and a rendered page is not proof of a 200: the help page's own
// history is that it answered 500 while the route and the template both
// existed, because it was missing from pageNames in internal/panel/assets.go.
Then('the help page answers 200', async function () {
  const status = await this.driver.executeAsyncScript(
    'const done = arguments[arguments.length - 1];' +
    'fetch("/help", { credentials: "same-origin" })' +
    '  .then(r => done(r.status)).catch(e => done(-1));'
  );
  assert.equal(status, 200, 'the status code the panel answered /help with');
});

Then('the help page is showing', async function () {
  // The title, not a heading. This page carries no h1: measured 2026-08-30,
  // internal/panel/templates/help.html has none in the working tree and none at
  // HEAD, while index.html, login.html and setup.html each have one. That is a
  // real gap and it is not this suite's to close, so the assertion is made
  // against what the page does have rather than against what it ought to.
  const title = await this.driver.getTitle();
  assert.equal(title, this.msg('help.title'), 'the title of the help page');

  const sections = await this.driver.findElements(By.css('main h2'));
  assert.ok(
    sections.length > 0,
    'the help page rendered with no section headings at all, which is what a ' +
      'template that failed to parse would also produce'
  );
});

// ---------------------------------------------------------------------------
// Then: keyboard and screen reader basics
// ---------------------------------------------------------------------------

Then('the first thing the keyboard reaches is the link that jumps to the page', async function () {
  // Tab once from the top of the document and ask what has focus. This is the
  // claim a keyboard user cares about, and it is not the same claim as "a skip
  // link exists in the markup": a link that is present but ordered after the
  // rail is a link nobody reaches.
  const focused = await this.driver.executeScript(
    'document.body.focus();' +
    'const focusable = document.querySelectorAll("a[href], button, input, select, textarea");' +
    'return focusable.length ? focusable[0].outerHTML : "";'
  );
  assert.ok(
    focused.includes('class="skip"'),
    'the first thing in the focus order is ' + JSON.stringify(focused.slice(0, 120))
  );
  const href = await this.attr('a.skip', 'href');
  assert.ok(href.endsWith('#main'), 'the skip link points at ' + href);
  const target = await this.present('#main');
  assert.ok(target, 'the skip link points at #main and there is no #main on the page');
});

Then('the skip link carries the words for it', async function () {
  const text = await this.text('a.skip');
  assert.equal(text, this.msg('nav.skip'), 'the words on the skip link');
});

Then('every control that takes a value has a label', async function () {
  // A control is labelled when a screen reader has something to announce for
  // it. There are four ways for that to be true and this checks all of them,
  // because failing a control that carries an aria-label would be a false
  // alarm, and passing one that carries nothing would be worse.
  const unlabelled = await this.driver.executeScript(`
    const out = [];
    const controls = document.querySelectorAll("input, select, textarea");
    for (const c of controls) {
      if (c.type === "hidden") continue;
      const byFor = c.id ? document.querySelector('label[for="' + c.id + '"]') : null;
      const wrapped = c.closest("label");
      const aria = c.getAttribute("aria-label");
      const ariaBy = c.getAttribute("aria-labelledby");
      const labelledBy = ariaBy ? document.getElementById(ariaBy) : null;
      if (byFor || wrapped || (aria && aria.trim()) || labelledBy) continue;
      out.push(c.tagName.toLowerCase() + "#" + (c.id || "(no id)") +
        " name=" + (c.name || "(no name)") + " type=" + (c.type || "(none)"));
    }
    return out;
  `);

  assert.deepEqual(
    unlabelled, [],
    'these controls have nothing a screen reader can announce for them: ' + unlabelled.join(', ')
  );

  // And the positive control. An empty list is also what a broken selector, a
  // page that failed to render, or a document with no controls at all would
  // produce, and that failure mode reports success.
  const counted = await this.driver.executeScript(
    'return document.querySelectorAll("input:not([type=hidden]), select, textarea").length;'
  );
  assert.ok(counted > 0, 'this page has no controls on it, so the check above proved nothing');
});
