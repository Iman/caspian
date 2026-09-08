# Flutter browser integration

Build the shared application, then run the browser scenarios:

```sh
bash scripts/build-ui.sh web dev
bash bdd/flutter/run.sh
```

The runner needs Flutter, Go, Node.js, and Chrome. It installs the locked
dependencies from `bdd/web/package-lock.json` when `node_modules` is absent.
Selenium Manager locates Chrome and its driver. Set `CHROME_BINARY` and
`CHROMEDRIVER` to use explicit local executables.

The harness serves the compiled Flutter application through the production Go
API. Passwords, sessions, CSRF checks, validation, and saved settings use the real
handlers and a temporary state store. Only privileged operating system actions
use the fake backend. These scenarios do not verify Wi-Fi hardware, routing,
firewalls, or native desktop installation.

Each scenario starts a fresh browser. Navigation and service restarts within a
scenario retain that browser. The runner records phase timings and verifies that
the served JavaScript matches the staged build by SHA-256.

Each run writes a new `reports/run.*/cucumber.json`. The report gate rejects zero
scenarios, stale reports, failures, and skipped steps. Browser resource checks
reject external asset requests and font or manifest loading errors. Failures
include a screenshot, browser log, and accessibility DOM when available.

Use Cucumber tags for a focused run, for example:

```sh
bash bdd/flutter/run.sh --tags @login
node --test --test-reporter=tap bdd/flutter/verify-report.test.js
```

The report validator tests use synthetic report fixtures. Their results do not
count as application integration tests.
