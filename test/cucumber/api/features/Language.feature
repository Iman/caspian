Feature: Language preference over HTTP

  @api-language-default
  Scenario: a fresh browser gets English even with a Persian browser preference
    Given the test appliance has not been set up
    And I set headers to
      | name            | value |
      | Accept-Language | fa    |
    When I GET "/setup"
    Then response code should be 200
    And response body should contain '<html lang="en" dir="ltr">'

  @ready @api-language-choice
  Scenario: choosing Persian persists without changing the requested page
    When I GET "/login?lang=fa"
    Then response code should be 200
    And response body should contain '<html lang="fa" dir="rtl">'
    And response header "Set-Cookie" should contain "caspian_lang=fa"
    When I GET "/login"
    Then response body should contain '<html lang="fa" dir="rtl">'
    When I GET "/login?lang=unknown"
    Then response body should contain '<html lang="fa" dir="rtl">'
    When I GET "/login?lang=en"
    Then response body should contain '<html lang="en" dir="ltr">'
