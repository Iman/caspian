# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 Iman Samizadeh

Feature: PositiveTests.feature
  What the Caspian panel does when everything is working, seen through a real
  browser with the real stylesheet applied.

  Every scenario carries a tag of its own as well as @smoke or @ready. The tag
  is not decoration: bdd/mutation.sh uses it to run that one scenario against a
  build with its subject deliberately broken, and requires it to go red. A
  scenario nobody has watched fail is not evidence, and the tag is how the
  watching is automated.

  Background:
    Given a Caspian panel that has been set up

  # -------------------------------------------------------------------------
  # Signing in
  # -------------------------------------------------------------------------

  @smoke @ready @right-password
  Scenario: I sign in with the password that was set and reach the dashboard
    Given I open the sign-in page
    When I sign in with the right password
    Then the dashboard is showing
    And the control bar is on the page

  # -------------------------------------------------------------------------
  # Persian first
  # -------------------------------------------------------------------------

  @smoke @ready @persian-default
  Scenario: the dashboard is drawn in Persian before anybody chooses a language
    Given I am signed in
    When I open the dashboard
    Then the page is drawn in Persian
    And the page reads right to left
    And the power control carries the Persian word for it

  @ready @english-switch
  Scenario: the dashboard changes to English when English is chosen
    Given I am signed in
    And I open the dashboard
    When I choose the other language
    Then the page is drawn in English
    And the page reads left to right
    And the power control carries the English word for it

  # -------------------------------------------------------------------------
  # The control bar is a traffic light
  #
  # These assert the COMPUTED background colour and not the class, and that is
  # the reason this suite exists. Commit 5c51497 records the defect: the four
  # .hero-<state> rules each set a background and ".hero { background:
  # var(--ground) }" was written after them at equal specificity, so it won
  # every time. The classes were right, the stylesheet held all four rules, and
  # every state drew the page ground. Nothing that compares markup can see that.
  #
  # No hex value appears in these scenarios either. Each names a role from the
  # palette, and the step resolves that role in the browser through the same
  # custom property the stylesheet uses. A palette change does not break them;
  # a broken cascade still does.
  # -------------------------------------------------------------------------

  @smoke @ready @hero-ok
  Scenario: the control bar is green when the box is carrying traffic
    Given I am signed in
    And the box is switched on and carrying traffic
    When I open the dashboard
    Then the control bar carries the "ok" state
    And the control bar is painted the green from the palette
    And the control bar is not painted the page ground

  @ready @hero-cut
  Scenario: the control bar pulses when client traffic has been cut
    Given I am signed in
    And the box is switched on and carrying traffic
    And client traffic has been cut
    When I open the dashboard
    Then the control bar carries the "cut" state
    And the control bar is animated by the cut pulse
    And the control bar is painted between the coral and the yellow of the pulse
    And the control bar is not painted the page ground

  # READ THIS BEFORE TRUSTING THE AMBER STATE.
  #
  # heroClass in internal/panel/words.go can return four values. The panel only
  # ever serves three of them. Measured 2026-08-30 by requesting the dashboard
  # and /status.json for all 32 combinations of engine phase, hotspot up or
  # down, traffic cut or not, and the privileged status call failing or not:
  # every response carried "ok", "off" or "cut", and none carried "wait".
  #
  # So this scenario cannot switch the box into the waiting state, because there
  # is no way to. It checks the half that is still real and still worth
  # guarding: that the amber rule paints amber and is not beaten by the cascade,
  # so the state works on the day something makes it reachable. The step that
  # puts the class on says "by hand" in its own name, so no reader can mistake
  # this for a state the product produces.
  @ready @hero-wait
  Scenario: the amber ground is painted for the waiting state
    Given I am signed in
    And the box is switched off
    And I open the dashboard
    When the control bar is put into the waiting state by hand
    Then the control bar is painted the yellow from the palette
    And the control bar is not painted the page ground

  # -------------------------------------------------------------------------
  # The two controls
  # -------------------------------------------------------------------------

  @smoke @ready @power-label
  Scenario: the power control names the action it will take, in both states
    Given I am signed in
    And the box is switched off
    When I open the dashboard
    Then the power control offers to switch the box on
    When I press the power control
    Then the power control offers to switch the box off

  @ready @cut-switch
  Scenario: the client traffic control is a switch and not a button
    Given I am signed in
    And the box is switched on and carrying traffic
    When I open the dashboard
    Then the client traffic control is a switch
    And the client traffic control has an accessible name
    And the client traffic control is labelled with the thing it switches

  @ready @cut-state
  Scenario: the client traffic control reports whether traffic is flowing
    Given I am signed in
    And the box is switched on and carrying traffic
    When I open the dashboard
    Then the client traffic control reports that traffic is flowing
    When I press the client traffic control
    Then the client traffic control reports that traffic is stopped
    And the control bar carries the "cut" state

  # -------------------------------------------------------------------------
  # Joining the hotspot
  #
  # This one looks at PIXELS, and it has to. The join code is inline SVG whose
  # only variable parts are integers, so every check that reads the markup
  # passes whatever colour the thing ends up. The standard requires a light
  # quiet zone around the symbol, and internal/panel/qr/svg.go paints it rather
  # than leaving it transparent precisely so a phone pointed at a coloured page
  # still sees a border. A rule that made that rect follow the foreground colour
  # would paint the whole symbol black, the page would still render, every
  # markup test would still pass, and no phone would ever read it.
  # -------------------------------------------------------------------------

  @smoke @ready @qr-render
  Scenario: the join code is drawn as a readable symbol rather than a black block
    Given I am signed in
    When I open the dashboard
    Then the join code is drawn
    And the join code has a light quiet zone
    And the join code is not a solid block
    And the join code carries both dark and light modules

  # -------------------------------------------------------------------------
  # Advanced, and the help page
  # -------------------------------------------------------------------------

  @ready @advanced
  Scenario: the advanced section opens on request and lists the interfaces the box found
    Given I am signed in
    When I open the dashboard with the advanced section showing
    Then the advanced section is showing
    And the advanced section lists the interface "eth0"
    And the advanced section lists the interface "wlan0"

  # The help page once answered 500, because it was not in pageNames in
  # internal/panel/assets.go and so was never parsed. The route existed, the
  # template existed, and the page was a server error.
  @smoke @ready @help
  Scenario: the help page is served and drawn
    Given I am signed in
    When I open the help page
    Then the help page answers 200
    And the help page is showing

  # -------------------------------------------------------------------------
  # Keyboard and screen reader basics
  # -------------------------------------------------------------------------

  @ready @skip-link
  Scenario: the first thing the keyboard reaches is the link that jumps to the page
    Given I am signed in
    When I open the dashboard
    Then the first thing the keyboard reaches is the link that jumps to the page
    And the skip link carries the words for it

  @ready @labels
  Scenario: every control on the dashboard that takes a value has a label
    Given I am signed in
    When I open the dashboard with the advanced section showing
    Then every control that takes a value has a label
