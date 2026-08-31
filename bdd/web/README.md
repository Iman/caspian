# Browser BDD

CucumberJS plus selenium-webdriver, driving a real headless Chrome against the
real panel.

## Run

    npx cucumber-js
    npx cucumber-js --tags @smoke
    npx cucumber-js features/PositiveTests.feature
    npx cucumber-js features/PositiveTests.feature:66

or `bash run.sh`, which installs dependencies first if they are missing and then
does the same thing.

The suite starts its own appliance and its own browser, so there is no server to
start first, no Selenium standalone jar, and no display to arrange.

## Scenarios

Positive:

1. I sign in with the password that was set and reach the dashboard
2. The dashboard is drawn in Persian before anybody chooses a language
3. The dashboard changes to English when English is chosen
4. The control bar is green when the box is carrying traffic
5. The control bar pulses when client traffic has been cut
6. The amber ground is painted for the waiting state
7. The power control names the action it will take, in both states
8. The client traffic control is a switch and not a button
9. The client traffic control reports whether traffic is flowing
10. The join code is drawn as a readable symbol rather than a black block
11. The advanced section opens on request and lists the interfaces the box found
12. The help page is served and drawn
13. The first thing the keyboard reaches is the link that jumps to the page
14. Every control on the dashboard that takes a value has a label

Negative:

1. The panel refuses a password that is not the one that was set
2. A switched off box does not draw the plain page ground
3. The dashboard does not claim a device is connected while the box is off
   (tagged `known-defect`, RED on this build, excluded from the default profile,
   see the repository's `bdd/README.md`)

## The parts that are not ordinary Cucumber

**Colours are read from the browser, not from the markup.** The control bar
scenarios call `getCssValue('background-color')` and compare it to the palette
role resolved through the same custom property the stylesheet uses. No hex value
appears anywhere in this suite, so a palette change does not break a scenario
and a broken cascade still does. That is the defect in commit `5c51497`: every
class was right and the paint was wrong.

**The join code is checked as pixels.** The step takes an element screenshot,
decodes the PNG in about eighty lines of `steps.js` with no image library, and
asserts that the four corners of the quiet zone are light, that the symbol is
neither nearly all dark nor nearly all light, and that it carries both dark and
light modules. Every markup check would pass a solid black square.

**Words come from the Go catalogue.** No Persian or English string is typed into
JavaScript. `features/support/hooks.js` fetches the catalogue from the harness
at the start of the run, so a reworded message is invisible to the suite and a
page drawn in the wrong language is not.

**One step reaches into the page, and says so in its own name.** "The control
bar is put into the waiting state by hand" exists because the panel cannot
produce that state; the repository's `bdd/README.md` has the measurement.
