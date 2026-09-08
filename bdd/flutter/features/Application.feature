Feature: One Caspian application controls the gateway
  The compiled Flutter web application uses the production Go API and state store.
  Only privileged operating system operations are simulated.

  @onboarding
  Scenario: First-run guidance stays inside the browser application
    Given Caspian is a new appliance
    Then the browser offers no operating system installation controls
    When I create a Caspian password
    Then the connection setup checklist is visible
    And setup does not claim the connection is ready
    When I save a valid proxy configuration named "First connection"
    And I set the hotspot name to "Caspian-first-run" and password to "fictional-hotspot-pass"
    Then the backend stores configuration "First connection" and hotspot "Caspian-first-run"
    When I switch Caspian on
    Then the gateway is running
    And setup does not claim the connection is ready
    When a simulated device joins the hotspot
    Then setup confirms the connection is ready
    When I finish the connection setup checklist
    Then the Caspian dashboard is visible
    And the browser offers no operating system installation controls

  @setup
  Scenario: First setup creates a password and survives a service restart
    Given Caspian has no password
    When I create a Caspian password
    Then the Caspian dashboard is visible
    When I sign out of Caspian
    And the Caspian service restarts
    And I sign in with the correct password
    Then the Caspian dashboard is visible

  @login
  Scenario: Sign in and sign out through the application
    Given I open Caspian
    When I sign in with the correct password
    Then the Caspian dashboard is visible
    When I sign out of Caspian
    Then the Caspian sign in form is visible
    And the browser cannot read authenticated settings

  @configuration
  Scenario: Save a configuration and hotspot settings
    Given I am signed in to Caspian
    When I save a valid proxy configuration named "Browser test"
    And I set the hotspot name to "Caspian-browser" and password to "fictional-hotspot-pass"
    Then the backend stores configuration "Browser test" and hotspot "Caspian-browser"
    When the Caspian service restarts
    And I sign in with the correct password
    Then the backend stores configuration "Browser test" and hotspot "Caspian-browser"

  @power
  Scenario: Start, cut traffic, restore traffic, and stop
    Given I am signed in to Caspian
    When I switch Caspian on
    Then the gateway is running
    When I toggle traffic
    Then traffic is cut
    When I toggle traffic
    Then traffic is restored
    When I switch Caspian off
    Then the gateway is stopped

  @language
  Scenario: Persian applies right-to-left layout and English restores it
    Given I am signed in to Caspian
    When I select the Persian language
    Then the application uses Persian and right-to-left text
    When I select the English language
    Then the application uses English and left-to-right text

  @wrong_password
  Scenario: Wrong password cannot open the dashboard
    Given I open Caspian
    When I sign in with an incorrect password
    Then the Caspian sign in form is visible
    And the application reports a problem
    And the browser cannot read authenticated settings

  @invalid_configuration
  Scenario: Invalid configuration leaves the saved configuration intact
    Given I am signed in to Caspian
    When I submit an invalid proxy configuration
    Then the application reports a problem
    And the backend retains the original configuration

  @setup_mismatch
  Scenario: Mismatched setup passwords do not create an account
    Given Caspian has no password
    When I submit different setup passwords
    Then the Caspian setup form is visible
    And the application reports a problem

  @privileged_failure
  Scenario: Privileged service refusal is shown without claiming connection
    Given I am signed in to Caspian
    And the privileged service refuses to start
    When I switch Caspian on
    Then the application reports a problem
    And the gateway is stopped

  @csrf
  Scenario: Browser mutation without a CSRF token is refused
    Given I am signed in to Caspian
    When the browser submits power without a CSRF token
    Then the request is forbidden
    And the gateway is stopped
