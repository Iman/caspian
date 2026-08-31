// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh
//
// The whole of the panel's JavaScript. There is no framework, no build step and
// no second file, and it does exactly one job: keep the status line, the device
// count and the switch label current without the user reloading the page.
//
// Everything on the page works with this file absent or blocked. The status is
// rendered by the server on every load, the switch is a form post, the advanced
// toggle is a link, and the disclosures are <details> elements. That is not
// caution for its own sake: the panel has to work when the box is in a bad
// state, and the fewer things that have to succeed before the switch is usable,
// the better.
//
// It makes exactly one request, to /status.json on this same origin. The
// Content-Security-Policy served with every page allows connections to 'self'
// and nowhere else, so this file cannot grow an outbound request by accident.

(function () {
  "use strict";

  var POLL_MS = 5000;
  var BACKOFF_MAX_MS = 60000;

  var dot = document.getElementById("status-dot");
  var word = document.getElementById("status-word");
  var deviceLine = document.getElementById("device-line");
  var deviceCount = document.getElementById("device-count");
  var detected = document.getElementById("detected");
  var powerButton = document.getElementById("power-button");
  var powerLabel = document.getElementById("power-label");
  var powerValue = document.getElementById("power-value");
  var cutButton = document.getElementById("cut-button");
  var cutLabel = document.getElementById("cut-label");
  var cutValue = document.getElementById("cut-value");
  var cutBanner = document.getElementById("cut-banner");
  var cutMark = document.getElementById("cut-mark");
  var nextStep = document.getElementById("nextstep-text");

  // Nothing to update on the sign-in and setup screens.
  if (!dot || !word) {
    return;
  }

  var delay = POLL_MS;
  var timer = null;

  // One loop, always. Every path that wants the next poll goes through here, so
  // waking the tab cannot leave two timers running and double the request rate
  // for the rest of the session.
  function schedule(ms) {
    if (timer !== null) {
      window.clearTimeout(timer);
    }
    timer = window.setTimeout(poll, ms);
  }

  function setAttr(el, name, value) {
    if (el && typeof value === "string" && el.getAttribute(name) !== value) {
      el.setAttribute(name, value);
    }
  }

  function setText(el, value) {
    if (el && typeof value === "string" && el.textContent !== value) {
      el.textContent = value;
    }
  }

  function apply(status) {
    setText(word, status.word);

    if (typeof status.shape === "string") {
      // The class is rebuilt from a fixed list rather than interpolated, so a
      // value from the server cannot become an arbitrary class name.
      var shape = "off";
      if (status.shape === "on" || status.shape === "working" || status.shape === "trouble") {
        shape = status.shape;
      }
      dot.className = "dot dot-" + shape;
    }

    // The instruction changes with the box. Somebody watching this page while
    // it connects should see the next thing to do, not the last one.
    setText(nextStep, status.nextStep);
    setText(deviceLine, status.deviceLine);
    setText(detected, status.detected);
    if (deviceCount && typeof status.devices === "number") {
      // The tile shows the bare number; the sentence under the QR shows the
      // words. Both come from the server already in the reader's language, so
      // nothing here formats or pluralises anything: this file has no idea
      // which language the page is in and must not learn.
      setText(deviceCount, String(status.devices));
    }

    if (powerButton && powerValue) {
      // running, not connected: the cut makes connected false while the box
      // is still on, and a switch keyed on connected offers to start a box
      // that never stopped.
      var running = status.running === true;
      powerValue.value = running ? "0" : "1";
      // The label element, not the button. The button also holds the power
      // glyph, and writing textContent onto the button removed it.
      //
      // The words come from the server. They were written into this file in
      // English, so five seconds after a Persian page loaded the switch
      // relabelled itself in English and stayed that way.
      setText(powerLabel, status.powerLabel);
      // The class comes from the server, like the label: three states cannot
      // be derived from one boolean, and a second opinion about which is
      // which is how the page and the box start disagreeing.
      powerButton.className = "big " + (status.powerClass || (running ? "danger" : "go"));
    }

    // The control bar's ground. Same rule again: the class comes from the
    // server. Without this the bar kept whatever ground the page was rendered
    // with, so a box that came up green while the page was open went on showing
    // the amber it started in until somebody reloaded.
    var hero = document.getElementById("hero");
    if (hero && status.heroClass) {
      hero.className = "tile hero hero-" + status.heroClass;
    }

    // The client-traffic control. Same rule as the switch: the words come from
    // the server already translated, and the label element is written rather
    // than the button, which also holds nothing else today but would lose it.
    if (cutButton && cutValue) {
      var cut = status.trafficCut === true;
      cutValue.value = cut ? "0" : "1";
      // On means traffic is flowing, so the class is the negation of the cut.
      cutButton.className = "toggle" + (cut ? "" : " toggle-on");
      cutButton.setAttribute("aria-checked", cut ? "false" : "true");
      setAttr(cutButton, "aria-label", status.cutLabel);
    }
    if (cutBanner) {
      cutBanner.hidden = status.trafficCut !== true;
    }
  }

  function poll() {
    fetch("/status.json", {
      credentials: "same-origin",
      headers: { Accept: "application/json" },
      cache: "no-store"
    })
      .then(function (res) {
        if (res.status === 401) {
          // The session ended. A full load takes the browser to the sign-in
          // page, which is the server's decision to make, not this file's.
          window.location.reload();
          return null;
        }
        if (!res.ok) {
          throw new Error("status " + res.status);
        }
        return res.json();
      })
      .then(function (status) {
        if (status) {
          apply(status);
          delay = POLL_MS;
        }
      })
      .catch(function () {
        // A failed poll is not worth telling the user about: the page they are
        // looking at was rendered by the server and is still true enough. Back
        // off so that a box whose panel process is struggling is not asked
        // every five seconds by every open tab.
        delay = Math.min(delay * 2, BACKOFF_MAX_MS);
      })
      .then(function () {
        schedule(delay);
      });
  }

  // Stop polling while the tab is hidden. A phone left on this page in a
  // pocket should not keep a Raspberry Pi busy.
  document.addEventListener("visibilitychange", function () {
    if (document.visibilityState === "visible") {
      delay = POLL_MS;
      schedule(0);
    } else if (timer !== null) {
      window.clearTimeout(timer);
      timer = null;
    }
  });

  schedule(delay);
})();
