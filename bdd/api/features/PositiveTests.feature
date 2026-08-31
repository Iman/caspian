# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 Iman Samizadeh

Feature: PositiveTests.feature
  The Caspian panel's HTTP endpoints, with no browser: what they answer when the
  request is well formed and the client is allowed to make it.

  The step vocabulary is apickli's, so that a reader who knows the project this
  suite is modelled on reads these files without a second thought. The library
  itself is not used, and features/support/world.js gives the two measured
  reasons.

  Background:
    Given I set headers to
      | name   | value            |
      | Accept | text/html         |

  # -------------------------------------------------------------------------
  # The door
  # -------------------------------------------------------------------------

  @smoke @ready @api-login-ok
  Scenario: signing in with the right password starts a session
    Given I have a form token from "/login"
    When I POST to "/login" with the form token and
      | name     | value                 |
      | password | correct-horse-battery |
    Then response code should be 303
    And response header "Location" should be "/"

  # -------------------------------------------------------------------------
  # The status document the dashboard script polls
  # -------------------------------------------------------------------------

  @smoke @ready @api-status-json
  Scenario: the status document carries the fields the dashboard script reads
    Given I am signed in as the panel owner
    When I GET "/status.json"
    Then response code should be 200
    And response header "Content-Type" should contain "application/json"
    And response body should be valid json
    And response body should have the keys
      | connected  |
      | running    |
      | word       |
      | shape      |
      | devices    |
      | deviceLine |
      | detected   |
      | problem    |
      | hasConfig  |
      | uptime     |
      | powerLabel |
      | heroClass  |
      | powerClass |
      | nextStep   |
      | trafficCut |
      | cutLabel   |
      | cutBanner  |
      | events     |
    And response body path "connected" should be the boolean false
    And response body path "devices" should be a number
    And the hero class should be one of the three the panel serves

  # The traffic light, from the side the browser suite cannot see: this is the
  # document panel.js assigns the bar's class from, so a page that never
  # reloads takes its colour from here.
  @ready @api-status-hero
  Scenario Outline: the status document reports the state of the box
    Given I am signed in as the panel owner
    And the appliance is <state>
    When I GET "/status.json"
    Then response code should be 200
    And response body path "heroClass" should be "<heroClass>"
    And response body path "running" should be the boolean <running>
    And the hero class should be one of the three the panel serves

    Examples:
      | state                          | heroClass | running |
      | switched off                   | off       | false   |
      | switched on and carrying traffic | ok      | true    |

  @ready @api-status-cut
  Scenario: the status document reports a deliberate cut as its own state
    Given I am signed in as the panel owner
    And the appliance is switched on and carrying traffic
    And client traffic has been cut
    When I GET "/status.json"
    Then response code should be 200
    And response body path "heroClass" should be "cut"
    And response body path "trafficCut" should be the boolean true
    # Connected answers "can client traffic leave", and it cannot, so a cut box
    # is not connected even though the tunnel is up. That distinction is the
    # whole reason the cut has a state of its own rather than reusing green.
    And response body path "connected" should be the boolean false

  # -------------------------------------------------------------------------
  # The help page
  #
  # It once answered 500, because it was not in pageNames in
  # internal/panel/assets.go and so was never parsed. The route existed, the
  # template existed, and the page was a server error.
  # -------------------------------------------------------------------------

  @smoke @ready @api-help
  Scenario: the help page is served
    Given I am signed in as the panel owner
    When I GET "/help"
    Then response code should be 200
    And response header "Content-Type" should contain "text/html"
    And response body should contain "</html>"

  # -------------------------------------------------------------------------
  # Advanced settings
  # -------------------------------------------------------------------------

  @ready @api-advanced-save
  Scenario: an advanced setting is saved and comes back on the next page
    Given I am signed in as the panel owner
    And I have a form token from "/?advanced=1"
    When I POST to "/advanced" with the form token and
      | name              | value  |
      | internet_interface | eth0  |
      | hotspot_interface  | wlan0 |
      | channel            | 6     |
      | panel_on_lan       | 1     |
    Then response code should be 303
    When I GET "/?advanced=1"
    Then response code should be 200
    And response body should contain "value=\"eth0\" selected"
    And response body should contain "value=\"6\" selected"
    And response body should contain "name=\"panel_on_lan\" value=\"1\" checked"
