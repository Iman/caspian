# Caspian-BYOC BDD

Two Cucumber suites for the panel, in the shape of the `carcodeal-bdd` project
this is modelled on: a `web/` suite that drives a real browser, and an `api/`
suite that talks to the endpoints directly.

    bdd/
      harness/            a Go program that serves the REAL panel against fakes
      web/                browser BDD, CucumberJS plus selenium-webdriver
        features/
          PositiveTests.feature
          NegativeTests.feature
          step_definitions/steps.js
          support/world.js
          support/hooks.js
        cucumber.js
        package.json
        run.sh
      api/                HTTP BDD, CucumberJS with no browser
        features/
          PositiveTests.feature
          NegativeTests.feature
          step_definitions/steps.js
          support/world.js
          support/hooks.js
        cucumber.js
        package.json
        run.sh
      run-all.sh          both suites, one verdict
      mutation.sh         the proof that every scenario can fail
      mutation-report.js  turns a mutation run into the table
      mutation-support.js picks each scenario's defect from its own tag
      defects.json        the registry: one named defect per scenario tag

## Install

Node and npm, a Go toolchain, and Chrome. The browser suite runs Chrome
headless.

    cd bdd/web && npm install
    cd bdd/api && npm install

Measured on 2026-08-30, on the machine this was written on: Node 26.0.0, npm
11.12.1, Go 1.27.0, Chrome 151.0.7922.175, `@cucumber/cucumber` 13.2.1,
`selenium-webdriver` 4.48.0. Those are the versions it has been seen working
with. Older ones may well be fine and have not been tried here, which is a
different statement from "supported".

The `web` install is 104 packages and the `api` install is 87, and npm reported
no vulnerabilities for either.

The first browser run also downloads a chromedriver matching the installed
Chrome, through Selenium Manager, and caches it under `~/.cache/selenium`.
Measured the same day: it fetched chromedriver 151.0.7922.138. That is the only
thing either suite fetches at run time, and only once.

## Run

One command for everything:

    bash bdd/run-all.sh

Or one suite at a time, from `bdd/web` or `bdd/api`:

    npx cucumber-js
    npx cucumber-js --tags @smoke
    npx cucumber-js features/PositiveTests.feature
    npx cucumber-js features/PositiveTests.feature:66

Those are the same four invocations the reference project documents, and all
four work here. The last one runs the single scenario starting on that line.

`run.sh` in each directory is a convenience wrapper that installs dependencies
if they are missing and then does the same thing. It is not a required entry
point: the suites start their own appliance from `features/support/hooks.js`, so
`cucumber-js` on its own is enough.

Do not pipe any of these into `tail`, `tee`, `head` or `grep` when you care
about the exit code. A pipeline exits with the status of its last command, so
piping throws away the answer and hands you a green that means "tail worked".
`scripts/gate.sh` records what that trap has already cost this project.

    bash bdd/run-all.sh > bdd.log 2>&1; echo "exit: $?"

## No hardware, no root, no network

`bdd/harness` starts the real `internal/panel` on a loopback listener: the real
templates, the real stylesheet, the real script, the real QR encoder and the
real message catalogue, all through the same embedded filesystem the appliance
uses. What is faked is the machine underneath, through the `FakePrivileged` that
`internal/panel` already ships for its own tests, and a real state store over a
temporary directory.

No Raspberry Pi. No root. No radio. No `/dev/net/tun`. Nothing leaves the
machine except the one-off chromedriver download above.

The harness also mounts a `/__control/` API that the suites use to put the box
into a state and to inject a fault. It is mounted by that command and by nothing
else; `internal/panel` has no knowledge of it and the appliance does not link
the package.

## What the suites do NOT prove

They render in one browser, at one window size, in one version of Chrome. A
colour reported here is the colour that Chrome computed on this machine today.
It is not a claim about Safari, about a phone, or about a screen with a
different colour profile.

They read the DOM and the computed style. They are not a screen reader, so
"every control has a label" is a check on the markup a screen reader would
consume, not a recording of what one announced.

They say nothing whatever about traffic. No scenario captures an exit IP and
none can, which is the same limit `test/bdd` states in its own `doc.go` and for
the same reason.

## Proving the scenarios can fail

A scenario nobody has watched fail is not evidence. `test/bdd` enforces that in
Go with `TestEveryScenarioCanFail`. The same job, for these suites:

    bash bdd/mutation.sh          # every row
    bash bdd/mutation.sh web      # the browser suite only
    bash bdd/mutation.sh api      # the HTTP suite only

One cucumber run per suite, with `CASPIAN_MUTATION=1`. In that mode the `Before`
hook looks up the scenario's OWN tag in `bdd/defects.json` and rebuilds the
appliance carrying that defect, so every scenario runs against a build with its
own subject broken and nothing else. `mutation-report.js` then reads the JSON
report and prints one row per scenario: which scenario, which defect, whether it
went red, and whether that is what the registry said should happen.

A scenario whose tag is not in the registry throws in the `Before` hook. That is
the same rule `test/bdd` holds with `TestEveryScenarioNamesADefect`: there must
be no way to add a scenario without also adding the thing that proves it can
fail.

It was one cucumber process per scenario at first, which meant one Chrome per
scenario. On this machine that leaked browsers and wedged on the fourteenth row,
measured 2026-08-30. One process, one browser, and an appliance rebuilt between
scenarios gives the same isolation without the cost.

The defects live in `bdd/harness/main.go` and each one is a plausible fault
rather than a random mutation. The headline example puts back exactly the
declaration that commit `5c51497` removed:

    .hero { background: var(--ground); }

which is why the control bar drew the page ground in every state for the whole
life of the project while every class and every string was correct.

That is also the reason the control bar scenarios assert the COMPUTED background
colour rather than the class. Nothing that compares markup can see a defect
whose only symptom is which of two equally specific rules won.

## One scenario is red on purpose

`bdd/web/features/NegativeTests.feature` ends with a scenario tagged
`known-defect`. It FAILS on this build, and that is the finding rather than a
broken test.

Measured 2026-08-30 through the real panel: with the engine stopped and the
hotspot not running, the dashboard renders `1` on the devices tile and
`1 device connected` on the line under the hotspot card.

The mechanism, read rather than guessed:

| file | what it does |
| --- | --- |
| `internal/hotspot/supervisor.go` | `Status` sets `st.Devices` from the lease file before deciding why the hotspot is not running, so the count is never gated on it |
| `internal/hotspot/supervisor.go` | `devices()` reads the DHCP lease file from disk, and `dnsmasq.go`'s own comment records that the lease file remains after the hotspot goes down |
| `internal/hotspot/leases.go` | `ActiveLeases` filters on lease expiry only |
| `internal/panel/view.go` | the devices tile is the raw integer |
| `internal/panel/words.go` | `DeviceCountLine` keys on the count alone |

So a box that was on, had a phone joined, and was then switched off keeps
reporting that phone until its lease expires. Nothing can be joined to a network
that is not being broadcast.

The default profile excludes it, so a normal run is meaningful. Run it
deliberately:

    cd bdd/web && npx cucumber-js --profile defect

It is a profile rather than `--tags @known-defect`, because a profile's tags and
the command line's tags are ANDed: `--tags @known-defect` against a default
profile carrying `not @known-defect` matches nothing and exits 0 having run no
scenarios. That false green was hit while building this, and it is why both
`run-all.sh` and `mutation-report.js` refuse a run that executed nothing.

The fix is the maintainer's decision, not this suite's. Gating the count on the
hotspot running is one line; saying "0 devices, the hotspot is off" is a
different and possibly better answer. Either way the scenario goes green when it
is made.

## A second finding, recorded because it outlives the suite

`heroClass` in `internal/panel/words.go` can return four values. The panel only
ever serves three of them.

Measured 2026-08-30 by requesting the dashboard and `/status.json` for all 32
combinations of engine phase, hotspot up or down, traffic cut or not, and the
privileged status call failing or not: every response carried `ok`, `off` or
`cut`, and none carried `wait`.

`wait` needs `Running` true and `Connected` false with no cut. `Connected` is
left false only when the fault branch of `fillStatus` is taken first, and that
branch is only ever reached with a zero `SystemStatus`, in which `Running` is
false. All three call sites of `fillStatus` are fed from `Panel.status`, and one
of them does not call it at all unless the fault is none.

The amber state was introduced by commit `5c51497`, whose comment above
`heroClass` says amber means "switched on but the tunnel is not up yet" and that
a box that was starting used to draw the page ground and now does not. A
starting box draws `hero-off`, measured. The scenario tagged `hero-wait` says so
in full and checks the half that is still real: that the amber rule paints amber
and is not beaten by the cascade, so the state works on the day it becomes
reachable.

Nothing here changes `internal/panel`. Both findings are for the maintainer.

## A third finding: the help page has no h1

`internal/panel/templates/help.html` contains no `<h1>` element. Measured
2026-08-30, in the working tree and at HEAD:

| template | h1 elements |
| --- | --- |
| `index.html` | 1 |
| `login.html` | 1 |
| `setup.html` | 1 |
| `help.html` | 0 |
| `problem.html` | 0 |

A page whose headings start at `h2` gives a screen reader no top-level label for
the document, and a reader navigating by heading lands inside a section rather
than at the start. It is not something this suite should fix, and the file was
off limits to the work that found it, so the scenario asserts what the page does
have instead: the document title, and that at least one section heading
rendered. If an `h1` is added, that assertion still passes and can be tightened.

## Why not apickli

The reference project's `api/` suite uses apickli, and the step vocabulary here
is deliberately apickli's, so those feature files read the same. The library
itself is not used, for two measured reasons.

Installing it on 2026-08-30 pulled 394 packages and npm reported 15
vulnerabilities, 4 of them critical, through `request`, `superagent` 3.8.3,
`multer` 1.4.4 and `core-js` 2. The same install without it is 104 packages and
npm reports none.

And it would not have done the job. Every state-changing route on this panel is
behind a session cookie AND a per-session CSRF token that has to be scraped out
of the rendered HTML of the form. apickli's canned Gherkin is built for JSON
bodies and has no vocabulary for that, so the interesting half of these
scenarios would have been hand-written anyway.
