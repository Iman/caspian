# SPDX-License-Identifier: AGPL-3.0-or-later
Feature: Optional SNI spoofing
  @smoke @ready @sni @sni-save
  Scenario: save and disable a spoof name beside an existing config
    Given I am signed in
    When I save spoof name " COVER.Example.Invalid. "
    Then the spoof name is "cover.example.invalid"
    When I save spoof name ""
    Then the spoof name is ""

  @ready @sni @sni-invalid
  Scenario: an invalid spoof name does not replace the saved value
    Given I am signed in
    When I save spoof name "cover.example.invalid"
    And I save spoof name "https://bad.example.invalid"
    Then the spoof name is "cover.example.invalid"

  @ready @sni @dpi-save
  Scenario Outline: split settings are independent of the fake name
    Given I am signed in
    And a TLS config is stored for splitting
    When I save DPI options "<name>" TCP "<tcp>" TLS records "<tls>"
    Then the spoof name is "<name>"
    And TCP split is "<tcp>" and TLS-record split is "<tls>"
    When I save DPI options "" TCP "off" TLS records "off"
    Then TCP split is "off" and TLS-record split is "off"

    Examples:
      | name                  | tcp | tls |
      |                       | on  | off |
      |                       | off | on  |
      |                       | on  | on  |
      | cover.example.invalid | on  | off |
      | cover.example.invalid | off | on  |
      | cover.example.invalid | on  | on  |

  @ready @sni @dpi-invalid
  Scenario: invalid fake name preserves enabled split settings
    Given I am signed in
    And a TLS config is stored for splitting
    When I save DPI options "cover.example.invalid" TCP "on" TLS records "on"
    And I save spoof name "https://bad.example.invalid"
    Then the spoof name is "cover.example.invalid"
    And TCP split is "on" and TLS-record split is "on"
