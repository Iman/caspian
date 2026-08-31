# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 Iman Samizadeh

Feature: NegativeTests.feature
  What the Caspian panel must NOT do: the refusals, and the things it must not
  claim.

  The last scenario in this file is RED on this build. It carries the tag
  "known-defect", which the default profile excludes; cucumber.js and the README
  both explain why. It is here rather than deleted because the behaviour it
  describes is what the panel should do, and the panel does something else.
  A line of a feature description cannot begin with the tag character, which is
  why that tag is quoted here rather than written out.

  Background:
    Given a Caspian panel that has been set up

  # -------------------------------------------------------------------------
  # The door
  # -------------------------------------------------------------------------

  # The pair with the sign-in scenario in PositiveTests.feature is deliberate.
  # "The wrong password was refused" is also what a broken form, a wrong address
  # or a server that never started would produce, and that failure mode reports
  # success. The positive scenario is the control: same browser, same form,
  # right password, and a dashboard.
  @smoke @ready @wrong-password
  Scenario: the panel refuses a password that is not the one that was set
    When I open the sign-in page
    And I sign in with the wrong password
    Then the sign-in page is still showing
    And the page says the password is not right
    And the dashboard is not showing

  # -------------------------------------------------------------------------
  # A switched off appliance must not look like the page it sits on
  # -------------------------------------------------------------------------

  # The stylesheet's own comment on .hero-off says it: "Red rather than the page
  # ground, because an appliance that is doing nothing for you should not look
  # like the page it sits on." For the whole life of the project it drew the
  # page ground anyway. This is the scenario that would have said so.
  @smoke @ready @hero-off
  Scenario: a switched off box does not draw the plain page ground
    Given I am signed in
    And the box is switched off
    When I open the dashboard
    Then the control bar carries the "off" state
    And the control bar is not painted the page ground
    And the control bar is painted the coral from the palette

  # -------------------------------------------------------------------------
  # A KNOWN OPEN DEFECT, PINNED HERE RATHER THAN DESCRIBED SOMEWHERE
  # -------------------------------------------------------------------------

  # This scenario FAILS on this build. That is the finding, not a broken test.
  #
  # Measured 2026-08-30 through the real panel: with the engine stopped and the
  # hotspot not running, the dashboard renders "1" on the devices tile and
  # "1 device connected" on the line under the hotspot card.
  #
  # Why it happens, read rather than guessed:
  #
  #   internal/hotspot/supervisor.go  Status sets st.Devices from the lease file
  #                                   before deciding why the hotspot is not
  #                                   running, so the count is never gated on it
  #   internal/hotspot/supervisor.go  devices() reads the DHCP lease file from
  #                                   disk, and dnsmasq.go's own comment records
  #                                   that "the lease file remains" after the
  #                                   hotspot goes down
  #   internal/hotspot/leases.go      ActiveLeases filters on lease EXPIRY only
  #   internal/panel/view.go          the devices tile is the raw integer
  #   internal/panel/words.go         DeviceCountLine keys on the count alone
  #
  # So a box that was on, had a phone joined, and was then switched off keeps
  # reporting that phone for as long as its lease has not expired. Nothing can
  # be joined to a network that is not being broadcast.
  #
  # The fix is a decision for the maintainer, not for this suite. Gating the
  # count on the hotspot running is one line in the panel; saying "0 devices,
  # the hotspot is off" is a different and possibly better answer. Either way
  # this scenario goes green when it is made, and until then it says out loud
  # that the dashboard is claiming something untrue.
  @known-defect @device-count
  Scenario: the dashboard does not claim a device is connected while the box is off
    Given I am signed in
    And the box is switched on and carrying traffic
    And the hotspot reports 1 joined device
    And the box is switched off
    When I open the dashboard
    Then the control bar carries the "off" state
    And the page does not claim any device is connected
