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
// Requests remain on this origin: status polling and the same form actions
// used by the no-JavaScript fallback. Form submissions never block the UI.
// Content-Security-Policy served with every page allows connections to 'self'
// and nowhere else, so this file cannot grow an outbound request by accident.

(function () {
  "use strict";

  var generation = 0;
  function initialize() {
  var currentGeneration = ++generation;
  var busy = false;
  var feedback = document.getElementById("action-feedback");

  function message(key) {
    if (feedback) {
      feedback.hidden = false;
      feedback.textContent = feedback.dataset[key];
    }
  }

  var identifierForm = document.getElementById("identifier-generator");
  var identifierButton = document.getElementById("generate-identifiers");
  var identifierStatus = document.getElementById("identifier-status");
  var uuidOutput = document.getElementById("generated-uuid");
  var imeiOutput = document.getElementById("generated-imei");
  var copyButtons = document.querySelectorAll("[data-copy-target]");
  var generatingIdentifiers = false;

  function identifierMessage(key) {
    if (identifierStatus) {
      identifierStatus.hidden = false;
      identifierStatus.textContent = identifierStatus.dataset[key];
    }
  }

  function selectAndCopy(field) {
    field.focus();
    field.select();
    field.setSelectionRange(0, field.value.length);
    var copied = false;
    try {
      copied = document.execCommand && document.execCommand("copy");
    } catch (_) {
      copied = false;
    }
    identifierMessage(copied ? "copied" : "failed");
  }

  function copyIdentifier(field) {
    if (!field || !field.value) return;
    var clipboard = window.navigator && window.navigator.clipboard;
    if (window.isSecureContext && clipboard && clipboard.writeText) {
      clipboard.writeText(field.value)
        .then(function () { identifierMessage("copied"); })
        .catch(function () { selectAndCopy(field); });
      return;
    }
    // Clipboard access is commonly unavailable at a hotspot's plain-HTTP IP.
    // The selection API plus execCommand is the local fallback there, and the
    // selected text remains visible for manual copying if the browser refuses.
    selectAndCopy(field);
  }

  copyButtons.forEach(function (button) {
    button.onclick = function () {
      copyIdentifier(document.getElementById(button.dataset.copyTarget));
    };
  });

  if (identifierForm && identifierButton && uuidOutput && imeiOutput &&
      window.fetch && window.AbortController) {
    identifierForm.onsubmit = function (event) {
      event.preventDefault();
      if (generatingIdentifiers) return;

      var endpoint = new URL(identifierForm.action, window.location.href);
      if (endpoint.origin !== window.location.origin) return;
      generatingIdentifiers = true;
      identifierButton.disabled = true;
      copyButtons.forEach(function (button) { button.disabled = true; });
      identifierForm.setAttribute("aria-busy", "true");
      identifierMessage("working");

      var controller = new AbortController();
      var timeout = window.setTimeout(function () { controller.abort(); }, 8000);
      fetch(endpoint.href, {
        signal: controller.signal,
        credentials: "same-origin",
        headers: { Accept: "application/json" },
        cache: "no-store"
      }).then(function (response) {
        if (currentGeneration !== generation) return null;
        if (response.status === 401) {
          window.location.reload();
          return null;
        }
        if (!response.ok || new URL(response.url).origin !== window.location.origin ||
            !(response.headers.get("Content-Type") || "").includes("application/json")) {
          throw new Error("unexpected identifier response");
        }
        return response.json();
      }).then(function (identifiers) {
        if (!identifiers || currentGeneration !== generation) return;
        if (typeof identifiers.uuid !== "string" ||
            !/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/.test(identifiers.uuid) ||
            typeof identifiers.imei !== "string" || !/^[0-9]{15}$/.test(identifiers.imei)) {
          throw new Error("invalid identifier response");
        }
        uuidOutput.value = identifiers.uuid;
        imeiOutput.value = identifiers.imei;
        copyButtons.forEach(function (button) { button.disabled = false; });
        identifierMessage("ready");
      }).catch(function () {
        if (currentGeneration === generation) identifierMessage("failed");
      }).finally(function () {
        window.clearTimeout(timeout);
        if (currentGeneration === generation) {
          identifierButton.disabled = false;
          identifierForm.removeAttribute("aria-busy");
          generatingIdentifiers = false;
        }
      });
    };
  }

  document.onsubmit = function (event) {
    var form = event.target;
    if (!form || form.method.toLowerCase() !== "post" || !window.fetch || !window.AbortController) return;
    var target = new URL(form.action, window.location.href);
    if (target.origin !== window.location.origin) return;
    event.preventDefault();
    if (busy) return;
    busy = true;
    // Capture the intended action before a status poll can update its hidden
    // input. Disable submissions, not text selection, scrolling or navigation.
    var data = new URLSearchParams(new FormData(form));
    if (event.submitter && event.submitter.name) data.append(event.submitter.name, event.submitter.value);
    var buttons = document.querySelectorAll('button[type="submit"], input[type="submit"]');
    var previouslyDisabled = Array.from(buttons, function (button) { return button.disabled; });
    buttons.forEach(function (button) { button.disabled = true; });
    form.setAttribute("aria-busy", "true");
    var stopping = target.pathname === "/power" && data.get("on") === "0";
    message(stopping ? "stopping" : "working");
    var controller = new AbortController();
    var timeout = window.setTimeout(function () { controller.abort(); }, 60000);
    fetch(target.href, {
      method: "POST", body: data, credentials: "same-origin",
      cache: "no-store", signal: controller.signal
    }).then(function (response) {
      if (new URL(response.url).origin !== window.location.origin ||
          !(response.headers.get("Content-Type") || "").includes("text/html")) throw new Error("unexpected response");
      return response.text().then(function (html) { return {html: html, url: response.url}; });
    }).then(function (result) {
      var page = new DOMParser().parseFromString(result.html, "text/html");
      if (!page.getElementById("main")) throw new Error("incomplete response");
      // Server-rendered validation errors and notices stay intact. No scripts
      // from the response are evaluated, and no credentials enter history.
      window.clearTimeout(timer);
      document.body.replaceWith(document.importNode(page.body, true));
      document.title = page.title;
      document.documentElement.lang = page.documentElement.lang;
      document.documentElement.dir = page.documentElement.dir;
      window.history.replaceState(null, "", result.url);
      initialize();
    }).catch(function () {
      // A timeout is an unknown result, not permission to replay a mutation.
      message(stopping ? "stoppedRemote" : "failed");
      form.removeAttribute("aria-busy");
      buttons.forEach(function (button, index) { button.disabled = previouslyDisabled[index]; });
      busy = false;
    }).finally(function () { window.clearTimeout(timeout); });
  };

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
    if (currentGeneration !== generation) return;
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

    if (powerButton && powerValue && !busy) {
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
    if (currentGeneration !== generation) return;
    var controller = new AbortController();
    var timeout = window.setTimeout(function () { controller.abort(); }, 8000);
    fetch("/status.json", {
      signal: controller.signal,
      credentials: "same-origin",
      headers: { Accept: "application/json" },
      cache: "no-store"
    })
      .then(function (res) {
        if (currentGeneration !== generation) return null;
        if (res.status === 401) {
          // The session ended. A full load takes the browser to the sign-in
          // page, which is the server's decision to make, not this file's.
          if (!busy) window.location.reload();
          return null;
        }
        if (!res.ok) {
          throw new Error("status " + res.status);
        }
        return res.json();
      })
      .then(function (status) {
        if (status && currentGeneration === generation && !busy) {
          apply(status);
          delay = POLL_MS;
          if (feedback) feedback.hidden = true;
        }
      })
      .catch(function () {
        if (!busy && currentGeneration === generation) message("stale");
        delay = Math.min(delay * 2, BACKOFF_MAX_MS);
      })
      .then(function () {
        window.clearTimeout(timeout);
        schedule(delay);
      });
  }

  // Stop polling while the tab is hidden. A phone left on this page in a
  // pocket should not keep a Raspberry Pi busy.
  document.onvisibilitychange = function () {
    if (document.visibilityState === "visible") {
      delay = POLL_MS;
      schedule(0);
    } else if (timer !== null) {
      window.clearTimeout(timer);
      timer = null;
    }
  };

  schedule(delay);
  }
  initialize();
})();
