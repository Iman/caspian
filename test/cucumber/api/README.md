# HTTP BDD

CucumberJS against the panel's endpoints, with no browser.

## Run

    npx cucumber-js
    npx cucumber-js --tags @smoke
    npx cucumber-js features/PositiveTests.feature
    npx cucumber-js features/PositiveTests.feature:36

or `bash run.sh`, which installs dependencies first if they are missing and then
does the same thing.

The suite starts its own appliance, so there is no server to start first.

## Scenarios

Positive:

1. Signing in with the right password starts a session
2. The status document carries the fields the dashboard script reads
3. The status document reports the state of the box (a Scenario Outline over
   switched off and carrying traffic)
4. The status document reports a deliberate cut as its own state
5. The help page is served
6. An advanced setting is saved and comes back on the next page

Negative:

1. A password that is not the one that was set is refused
2. The status document is not served to a client with no session
3. A state-changing request with no form token is refused
4. The polled status document carries no credential
5. An advanced setting naming an interface the box does not have is refused

## The step vocabulary is apickli's, and apickli is not installed

The project this suite is modelled on drives its API features through apickli,
so the steps here are deliberately spelled the same way: "I set headers to",
"response code should be", "response body should be valid json", "response body
path ... should be". A reader who knows that project reads these files without a
second thought.

The library itself is not used. Two measured reasons, both from 2026-08-30:

- Installing it alongside cucumber and selenium-webdriver pulled 394 packages
  and npm reported 15 vulnerabilities, 4 of them critical, through `request`,
  `superagent` 3.8.3, `multer` 1.4.4 and `core-js` 2. The same install without
  it is 104 packages and npm reports none. This suite, which needs no browser
  at all, installs 87 packages and npm reports none.
- It would not have done the job. Every state-changing route on this panel is
  behind a session cookie AND a per-session CSRF token that has to be scraped
  out of the rendered HTML of the form. apickli's canned Gherkin is built for
  JSON bodies and has no vocabulary for that.

The cookie jar in `features/support/world.js` is fifteen lines, because Node's
`fetch` deliberately keeps no cookies between calls and a single-origin suite
talking to one panel needs nothing more.

## What this suite covers that the browser suite cannot

The browser sees the page after the script has run. This one sees the status
code, the headers, and the document `panel.js` polls, which is where the
dashboard gets its state from between reloads.

It also holds the panel's absolute rule about credentials: no response body ever
carries the pasted config, the hotspot passphrase, the panel password or a
session token. The polled document is the one most likely to end up in a browser
cache, a proxy log or a screen recording, so that is where the scenario looks.
