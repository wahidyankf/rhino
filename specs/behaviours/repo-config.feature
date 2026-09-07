Feature: Repository configuration contract

  Every value RHINO enforces arrives from the consuming repository's
  repo-config.yml. There is no default for any of them, so a configuration
  RHINO cannot understand is a fault to report rather than a gap to fill in.

  Scenario: A well-formed configuration validates
    Given the repository declares a complete configuration
    When I run the "repo-config" validator
    Then the exit code is 0

  Scenario: An absent configuration file is a configuration fault
    Given the repository has no configuration file
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr names the missing configuration file

  Scenario: An unrecognized schema identifier is refused
    Given the repository declares the schema "rhino/repo-config/v99"
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr names the unrecognized schema

  Scenario: The predecessor schema spelling is accepted as an alias
    Given the repository declares the schema "rhino-cli/repo-config/v1"
    When I run the "repo-config" validator
    Then the exit code is 0

  Scenario: An unknown key inside an owned section is refused
    Given the repository declares a complete configuration
    And the configuration adds the unknown key "node-label-grapheme" to "md-mermaid"
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr names the unknown key and its position

  Scenario: An unknown top-level section is ignored
    Given the repository declares a complete configuration
    And the configuration adds the top-level section "env-contract"
    When I run the "repo-config" validator
    Then the exit code is 0

  Scenario: A missing required key is refused
    Given the repository declares a complete configuration
    And the configuration omits "md-mermaid.edge-label-graphemes"
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr names the missing key

  Scenario: A value of the wrong type is refused
    Given the repository declares a complete configuration
    And the configuration sets "md-mermaid.node-label-graphemes" to "thirty-two"
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr names the offending key and its position

  Scenario Outline: A declared path may not escape the repository root
    Given the repository declares a complete configuration
    And the configuration sets "governance-directory-map.trees" to "<path>"
    When I run the "repo-config" validator
    Then the exit code is 2

    Examples:
      | path            |
      | ../outside      |
      | /absolute       |

  Scenario: An empty harness roster is a legal declaration
    Given the repository declares a configuration with an empty harness roster and no canonical skill or agent root
    When I run the "repo-config" validator
    Then the exit code is 0

  Scenario: An empty harness roster alongside a declared canonical root is refused
    Given the repository declares a configuration with an empty harness roster and a canonical skills root
    When I run the "repo-config" validator
    Then the exit code is 2
    And stderr names the canon that has nowhere to be reconciled

  Scenario: A validator refuses to run against a configuration it cannot read
    Given the repository declares a complete configuration
    And the configuration adds the unknown key "trees-list" to "governance-directory-map"
    When I run the "directory-map" validator
    Then the exit code is 2
    And no directories were inspected
