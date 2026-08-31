# Provenance of the panel golden files

Same discipline as `internal/netcfg/testdata/PROVENANCE.md`, and for the same
reason: a fixture nobody can trace is a fixture whose class nobody can tell, so
it silently becomes evidence it is not. Every file in this directory is named
below, with what produced it and what a diff in it means.

`internal/panel/golden_test.go` enforces this file. `TestGolden_ProvenanceDocumentsEveryGolden`
fails on any file here that is not mentioned by name, and
`TestGolden_EveryGoldenFileIsProducedBySomeCase` fails on any file no case
generates, which is how a stale golden survives a rename and quietly stops being
checked while still reading in a diff as though it were.

## Class of every file here

GENERATED, not captured. Nothing in this directory came off hardware. Each file
is the output of this repository's own code, rendered through
`internal/panel`'s test harness against `FakePrivileged`, on a developer machine
with no Raspberry Pi, no root and no network. That is the whole class:

- these files prove the panel's output HAS NOT CHANGED;
- they prove NOTHING about whether the output is correct, whether a browser
  renders it as intended, or whether the appliance behaves as the page claims.

The Cucumber browser suite and the hardware harness answer those. Do not read a
green run here as either.

## Capture vantage

- Host: developer machine, darwin/arm64, Go 1.27.0.
- Repository state: commit `531a296` plus uncommitted working-tree changes.
- Date: 2026-08-30.

WHAT THAT SECOND LINE MEANS, STATED PLAINLY. At the moment these files were
generated the working tree was dirty with three agents' concurrent work, and
five of the files that decide what the panel renders were among the modified
ones:

    internal/panel/i18n.go
    internal/panel/i18n_messages.go
    internal/panel/templates/index.html
    internal/panel/templates/help.html
    internal/panel/assets/panel.css

So these goldens pin the working tree, not commit `531a296`. The FIRST diff
after that work lands is expected and is not a regression. Read it, confirm it
is the wording and layout work those changes describe, and run
`bash scripts/golden-update.sh`.

## How to regenerate

One command for every golden layer in the repository:

    bash scripts/golden-update.sh

This package alone:

    go test ./internal/panel -run Golden -update

Then READ THE DIFF. A golden accepted without reading is a golden that records
nothing.

## Redaction, and what it deliberately does not cover

The dashboard renders the hotspot passphrase in the clear, because somebody
standing at the box has to read it, and it encodes that passphrase into the join
QR code. Those bytes cannot be committed, so `redactPage` removes exactly three
things from the rendered HTML and nothing else:

1. the CSRF token, which is random per session;
2. the QR modules, replaced by a sha256 prefix of themselves plus their length,
   so a change to the code is still a diff while the payload never lands here;
3. the saved hotspot passphrase.

Nothing else is redacted. `status-*.json` has no redaction at all, because no
credential belongs in that document; if one ever appears it lands in the golden
and `test/goldenscan` fails on it. A global redactor would have hidden that
case, which is the one that matters.

`secret-exposure.txt` is what keeps the first case honest. It counts, per state
and per language, how many times each credential appears in the RAW body BEFORE
redaction, so a page that starts echoing the proxy UUID moves a zero to a one.

A note on one mistake this layer already made, kept because the guard is better
for it: the suggestion regexes originally ran unconditionally and matched the
hotspot form on a page whose hotspot IS saved, so the committed golden showed
`<redacted:suggested-ssid>` where the real page shows the network name. An SSID
is broadcast and is not a secret, and hiding it hid the very thing the file
exists to pin. `TestGolden_RedactionDoesNotHideSavedValues` is the guard that
stops it recurring.

## Sentinels

Every credential fed to the panel here is a sentinel: a value that occurs
nowhere else in the repository and cannot arise by accident. That is what lets
`test/goldenscan` report a hit with no false positives. They are declared in
`internal/panel/golden_test.go` and none of them is, or has ever been, a working
credential. They are deliberately NOT the package's own `testPassword`, whose
value is also the passphrase in `internal/hotspot/testdata/hostapd.golden` and
so could not tell the two apart.

## TWO KNOWN DEFECTS, PINNED AS THEY BEHAVE TODAY

A golden records what the code DOES, not what it SHOULD do. Both defects below
are pinned in their current, wrong form on purpose, so that fixing either one
produces a diff in this directory. Neither is fixed. Neither is being worked
around here.

### Defect A: the device count is reported while the appliance is off

VERIFIED in source on 2026-08-30:

- `internal/panel/words.go`, `DeviceCountLine`, switches on `h.Devices` alone.
  `HotspotStatus.Running` is never read.
- `internal/panel/view.go`, `fillStatus`, sets `d.DeviceLine` from
  `st.Hotspot` with no gate on `Running`.
- `internal/panel/view.go`, `fillTiles`, sets the devices tile from
  `st.Hotspot.Devices` with no gate on `Running`.
- `internal/panel/view.go`, `newStatusJSON`, copies `st.Hotspot.Devices`
  straight into the polled document.

So with the engine stopped, the hotspot down, and one lease still counted, the
page and the status document both say a device is connected on a box that is
switched off.

Pinned by:

    page-defect-devices-while-off-fa.html
    page-defect-devices-while-off-en.html
    status-defect-devices-while-off-fa.json
    status-defect-devices-while-off-en.json

The English status document currently reads, in the same object:

    "running": false,  "word": "Off",  "uptime": "Not running",
    "devices": 1,      "deviceLine": "1 device connected"

WHAT THE FIX WILL LOOK LIKE: `deviceLine` and `devices` become zero, or the
sentence gains a qualifier, when `Running` is false. When that lands these four
files change. That diff is the point of them.

WHAT IS NOT ESTABLISHED HERE: whether the privileged service actually reports a
non-zero `Devices` while the hotspot is down on real hardware. This layer runs
against `FakePrivileged` and can only prove what the PANEL does with such a
status. The panel half is the half pinned.

### Defect B: two shipped strings say a restart restores client traffic

VERIFIED in source on 2026-08-30. The four strings, quoted verbatim from
`internal/panel/i18n_messages.go`:

- `cut.banner`, English: "Client traffic is cut. Your devices are still
  connected to this WiFi and can open this page, but nothing reaches the
  internet. Restarting this machine also restores it."
- `cut.banner`, Persian: ends "راه‌اندازی دوباره این دستگاه هم آن را برمی‌گرداند."
- `help.controls.cut`, English: "Stops client traffic at once while leaving the
  hotspot up. Devices stay joined and can still open this page, but nothing
  reaches the internet. Restarting the machine also undoes it."
- `help.controls.cut`, Persian: ends "راه‌اندازی دوباره دستگاه هم آن را برمی‌گرداند."

Why they are wrong. The cut flag itself does die on a restart, and
`internal/privsvc/cut.go` says so in its own words: it is a field on `Service`,
it reaches no file, and that is deliberate. But nothing brings the appliance
back up. `internal/state`'s `State` has no field recording that the box was
running, and the only callers of the privileged `Start` are
`internal/panel/handlers.go`'s power handler and the wire server dispatch behind
it, both driven by a POST from the panel. `packaging/caspian.service` starts the
privileged SERVICE at boot; it does not start the tunnel or the hotspot.

So after a restart the cut is gone and so is everything else: the box comes back
OFF and client traffic does not flow until somebody presses the switch. The
sentence tells a user that a reboot will restore their internet. It will not.

Pinned by:

    page-connected-traffic-cut-fa.html   (cut.banner, in place, in Persian)
    page-connected-traffic-cut-en.html   (cut.banner, in place, in English)
    page-help-fa.html                    (help.controls.cut, in Persian)
    page-help-en.html                    (help.controls.cut, in English)
    status-connected-traffic-cut-fa.json (cutBanner, as the poll sends it)
    status-connected-traffic-cut-en.json (cutBanner, as the poll sends it)

REMEDIATION, which is the message owner's to make and not this layer's: either
the sentence stops promising restoration, or the product starts restoring. The
strings live in `internal/panel/i18n_messages.go` under the keys `cut.banner`
and `help.controls.cut`, in both language maps. This layer does not change them;
it makes the change visible when somebody does.

An incidental observation, pinned rather than argued: `cutBanner` is present in
`status.json` in EVERY state, including states where `trafficCut` is false. The
script needs the sentence before the state changes, so that is defensible; it is
recorded here because it is now frozen and a future reader should know it was
noticed rather than missed.

## Every file

### Rendered pages

Redacted HTML of one state in one language. Filename is
`page-<state>-<lang>.html`. Each carries its own header comment naming the
state, the language, the request path and what it is a picture of.

    page-first-run-setup-fa.html            first run, no password chosen, /setup
    page-first-run-setup-en.html
    page-signed-out-login-fa.html           a password exists, no session, /login
    page-signed-out-login-en.html
    page-signed-in-off-fa.html              configured and switched off
    page-signed-in-off-en.html
    page-running-not-connected-fa.html      switch pressed, tunnel not up: the amber state
    page-running-not-connected-en.html
    page-connected-fa.html                  working: tunnel up, hotspot up, three devices
    page-connected-en.html
    page-connected-traffic-cut-fa.html      running and deliberately carrying nothing; defect B
    page-connected-traffic-cut-en.html
    page-advanced-fa.html                   the advanced view, fixed engine log
    page-advanced-en.html
    page-help-fa.html                       the help page; defect B
    page-help-en.html
    page-defect-devices-while-off-fa.html   defect A
    page-defect-devices-while-off-en.html

### The polled document

`status-<state>-<lang>.json`, the body of `GET /status.json`, indented for a
readable diff. `TestGolden_StatusJSONGoldenIsTheWireBytes` compacts each
committed file and compares it against a live response, so the indentation costs
nothing: a golden that no longer compacts to what the endpoint sends is a golden
of something the product does not do, and that test fails on it.

    status-signed-in-off-fa.json            status-signed-in-off-en.json
    status-running-not-connected-fa.json    status-running-not-connected-en.json
    status-connected-fa.json                status-connected-en.json
    status-connected-traffic-cut-fa.json    status-connected-traffic-cut-en.json
    status-defect-devices-while-off-fa.json status-defect-devices-while-off-en.json

### The contracts

    status-shape.txt

Every key of `status.json` with the JSON type behind it, taken from a live
response rather than from the struct, so it records what a browser receives
including tags and omissions. Then `heroClass` and `powerClass` evaluated over
their WHOLE domain, three booleans and two booleans, so the tables are exhaustive
rather than sampled, and the resulting value sets:

    heroClass  cut off ok wait
    powerClass danger go ok

`assets/panel.js` keys on both. A value added here with no stylesheet rule behind
it is a control that draws as nothing.

Note pinned in that file and worth repeating: `events` serialises as `null`, not
`[]`, when the log is empty, because the field is a nil slice.

    message-keys.txt

Every i18n key in the catalogue, per language, plus a section listing any key
missing from a language. KEYS ONLY, never the text. Text is edited constantly and
a golden of it would diff on every wording change, which trains people to run
`-update` without reading. A key is different: a key that disappears renders
`!!some.key!!` to exactly the users the default language exists to serve.

    routes.txt

Every route the panel serves, with the OBSERVED status code for a browser with
no session, and for a signed-in browser posting with no CSRF token. Observed,
not read off the route table: the table is what the code intends, the codes are
what a browser gets.

Two rows there are worth knowing rather than rediscovering. `GET /setup` answers
303 to a browser with no session, because setup is already finished in the probe.
`POST /login` and `POST /setup` answer 403 without a token even though both are
public routes, because they carry their own form token.

    secret-exposure.txt

Per state and per language, how many times each credential appears in the RAW
body before redaction. `hotspot-passphrase` is deliberately non-zero on the
dashboard. Every other row must stay zero on every page, forever.

    wifi-join.txt

The string the join QR encodes, with the passphrase replaced by a placeholder,
plus the byte length of the emitted SVG. The QR modules in the page goldens are
a digest, which detects a change and explains nothing; this is the readable half,
so a change to the escaping, the security type or the hidden flag is a sentence
somebody can read rather than a hash that moved.
