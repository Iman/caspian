# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 Iman Samizadeh

Feature: NegativeTests.feature
  What the Caspian panel's endpoints refuse, and what they must never put in a
  response body.

  The panel is reachable by anyone within radio range of the box, so every
  refusal here is load bearing.

  Background:
    Given I set headers to
      | name   | value     |
      | Accept | text/html |

  # -------------------------------------------------------------------------
  # The door
  # -------------------------------------------------------------------------

  # The pair with the sign-in scenario in PositiveTests.feature is deliberate.
  # "The wrong password was refused" is also what a broken request, a wrong
  # address or a server that never started would produce, and that failure mode
  # reports success. The positive scenario is the control.
  @smoke @ready @api-login-wrong
  Scenario: a password that is not the one that was set is refused
    When I POST to "/login" with the wrong password
    Then response code should be 401
    And response body should carry the message "login.wrong.headline"
    And response body should not contain "hero"

  @smoke @ready @api-unauthenticated
  Scenario: the status document is not served to a client with no session
    When I GET "/status.json"
    Then response code should be 401
    And response body should be valid json
    And response body path "error" should be "not signed in"

  # -------------------------------------------------------------------------
  # Cross-site request forgery
  #
  # SameSite=Strict on the session cookie is the first lock and the per-session
  # form token is the second. A refusal here is deliberately NOT a redirect: a
  # redirect would look like it worked, and this is the case where something
  # other than the panel's own page submitted the form.
  # -------------------------------------------------------------------------

  @smoke @ready @api-csrf
  Scenario: a state-changing request with no form token is refused
    Given I am signed in as the panel owner
    When I POST to "/power" with no form token
    Then response code should be 403
    And response body should carry the message "problem.badform.headline"

  # -------------------------------------------------------------------------
  # Credentials
  #
  # The panel's absolute rule: no response body and no log line ever carries the
  # pasted config, the hotspot passphrase, the panel password or a session
  # token. The status document is polled every few seconds by a script, so it is
  # the response most likely to end up in somebody's browser cache, proxy log or
  # screen recording.
  # -------------------------------------------------------------------------

  @smoke @ready @api-secrets
  Scenario: the polled status document carries no credential
    Given I am signed in as the panel owner
    And the appliance is switched on and carrying traffic
    When I GET "/status.json"
    Then response code should be 200
    And no credential should appear anywhere in the response

  # -------------------------------------------------------------------------
  # Advanced settings that name something that is not there
  # -------------------------------------------------------------------------

  @ready @api-advanced-bad
  Scenario: an advanced setting naming an interface the box does not have is refused
    Given I am signed in as the panel owner
    And I have a form token from "/?advanced=1"
    When I POST to "/advanced" with the form token and
      | name               | value        |
      | internet_interface | eth-nonesuch |
    Then response code should be 303
    When I GET "/"
    Then response code should be 200
    And response body should carry the message "adv.badinternet.headline"
