Feature: Directory-map validation

  A repository declares which trees carry directory maps. Within a declared
  tree, every directory needs a README whose Directory Map section lists each
  direct sibling exactly once, with a resolvable relative target.

  Background:
    Given the repository declares the mapped tree "rules"

  Scenario: Directory-map inspection ignores other validators' concerns
    Given file "root-instructions.md" contains 800 words
    And the repository contains:
      | path              | content                                                |
      | rules/README.md   | # Rules                                                |
      | guides/diagram.md | ```mermaid\nflowchart LR\nclassDef unsafe fill:red\n``` |
    When I inspect directory maps
    Then 1 directories were inspected
    And the only violation is a missing directory map at "rules/README.md"

  Scenario: Complete maps cover direct files and directories
    Given the repository contains:
      | path                   | content                                                                     |
      | rules/README.md        | # Rules\n\n## Directory Map\n\n- [Nested](nested/README.md)\n- [Policy](policy.md) |
      | rules/nested/README.md | # Nested\n\n## Directory Map\n\nThis directory currently has no other entries.     |
      | rules/policy.md        | # Policy                                                                    |
    When I inspect directory maps
    Then 2 directories were inspected
    And there are no violations

  Scenario Outline: The selected directory must stay relative and inside the repository
    Given an empty repository
    When I inspect directory maps under the invalid "<location>" location
    Then an argument error is raised

    Examples:
      | location                 |
      | empty path               |
      | absolute repository root |
      | outside repository       |

  Scenario: Query and fragment suffixes do not change a sibling target
    Given the repository contains:
      | path            | content                                                    |
      | rules/README.md | # Rules\n\n## Directory Map\n\n- [Policy](policy.md?raw=1#details) |
      | rules/policy.md | # Policy                                                   |
    When I inspect directory maps
    Then there are no violations

  Scenario: Absolute URL and malformed map links are invalid
    Given the repository contains:
      | path            | content                                                                                                            |
      | rules/README.md | # Rules\n\n## Directory Map\n\n- [Absolute](/policy.md)\n- [URL](https://example.com/policy.md)\n- [Malformed](bad{nul}path.md) |
    When I inspect directory maps
    Then there are 3 violations
    And all violations are "invalid map entry"

  Scenario: Every directory in a mapped tree needs a README
    Given the repository contains:
      | path              | content                                     |
      | rules/README.md   | # Rules\n\n## Directory Map\n\n- [Nested](nested) |
      | rules/nested/policy.md | # Policy                               |
    When I inspect directory maps
    Then the only violation is a missing README at "rules/nested"

  Scenario: Every README needs a Directory Map section
    Given the repository contains:
      | path            | content |
      | rules/README.md | # Rules |
    When I inspect directory maps
    Then the only violation is a missing directory map at "rules/README.md"

  Scenario: Every direct sibling must appear in the map
    Given file "rules/README.md" has title "Rules" and an empty directory map
    And the repository contains:
      | path            | content  |
      | rules/policy.md | # Policy |
    When I inspect directory maps
    Then the only violation is a missing map entry from "rules/README.md" to "rules/policy.md"

  Scenario: Independent inspections report independent violations
    Given the repository declares a word-budget surface "**/*.md" failing above 40
    And file "rules/README.md" has an empty "Rules" directory map followed by 41 words
    And the repository contains:
      | path            | content  |
      | rules/policy.md | # Policy |
    When I inspect directory maps
    Then the violations include an overlong "rules/README.md" and its missing map entry for "rules/policy.md"

  Scenario: A map entry must exist
    Given the repository contains:
      | path            | content                                                |
      | rules/README.md | # Rules\n\n## Directory Map\n\n- [Old policy](old-policy.md) |
    When I inspect directory maps
    Then the only violation is an invalid map entry from "rules/README.md" to "old-policy.md"

  Scenario: A map entry must be a direct sibling
    Given the repository contains:
      | path                   | content                                                     |
      | rules/README.md        | # Rules\n\n## Directory Map\n\n- [Nested](nested/README.md)      |
      | rules/nested/README.md | # Nested\n\n## Directory Map\n\n- [Parent](../README.md)         |
    When I inspect directory maps
    Then the only violation is an invalid map entry from "rules/nested/README.md" to "../README.md"
