# SPDX-License-Identifier: AGPL-3.0-or-later
Feature: Optional SNI spoofing
  @smoke @ready @sni
  Scenario: save and disable a spoof name beside an existing config
    Given I am signed in as the panel owner
    When I save spoof name " COVER.Example.Invalid. "
    Then the spoof name is "cover.example.invalid"
    When I save spoof name ""
    Then the spoof name is ""

  @ready @sni
  Scenario: an invalid spoof name does not replace the saved value
    Given I am signed in as the panel owner
    When I save spoof name "cover.example.invalid"
    And I save spoof name "https://bad.example.invalid"
    Then the spoof name is "cover.example.invalid"
