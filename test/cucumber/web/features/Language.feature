Feature: Panel language and responsive layout

  @ready @language-responsive
  Scenario Outline: the language control fits the page
    Given I am signed in
    When I use a viewport width of <width>
    And I open "<path>" in "<language>"
    Then the language control fits without horizontal scrolling
    And language controls have touch-sized targets
    And the language label names its dropdown

    Examples:
      | width | path          | language |
      | 320   | /             | en       |
      | 320   | /?advanced=1  | fa       |
      | 390   | /help         | en       |
      | 390   | /             | fa       |
      | 768   | /?advanced=1  | en       |
      | 768   | /help         | fa       |
      | 1280  | /             | en       |
      | 1280  | /             | fa       |

  @ready @language-persistence
  Scenario: a keyboard choice persists on the same page without JavaScript
    Given I am signed in
    When I open "/help" in "en"
    And browser scripting is disabled
    And I choose Persian with the keyboard
    Then the page is drawn in Persian
    And the page reads right to left
    And I remain on "/help"
    When I reload the page without language parameters
    Then the page is drawn in Persian

  @language-responsive
  Scenario: first setup fits a narrow phone
    Given the test appliance has not been set up
    When I use a viewport width of 320
    And I open "/setup" in "fa"
    Then the language control fits without horizontal scrolling
    And language controls have touch-sized targets
    And the language label names its dropdown
