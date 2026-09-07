Feature: Directory-map validation

  A repository declares which trees carry directory maps. Within a declared
  tree, every directory needs a README whose Directory Map section lists each
  direct sibling exactly once, with a resolvable relative target.

  Background:
    Given the repository declares the mapped tree "rules"

  Scenario: Directory-map inspection ignores other validators' concerns
    Given the repository declares a word-budget surface "**/*.md" failing above 40
    And the repository declares the accessible palette
    And file "root-instructions.md" contains 41 words
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
    When I inspect the word budget
    Then the only violation is a 41-word limit for "rules/README.md"
    When I inspect directory maps
    Then the only violation is a missing map entry from "rules/README.md" to "rules/policy.md"

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

  Scenario: A selected location that is not a directory is refused
    Given the repository contains:
      | path            | content                                          |
      | rules/README.md | # Rules\n\n## Directory Map\n\nNo other entries. |
    When I run the directory-map validator for "rules/README.md"
    Then the exit code is 2
    And stderr contains "is not a directory in this repository"

  Scenario: An excluded directory inside a mapped tree is listed but not inspected
    Given the repository declares excluded scan directories:
      * build-output
    And the repository contains:
      | path                         | content                                                    |
      | rules/README.md              | # Rules\n\n## Directory Map\n\n- [Generated](build-output/README.md) |
      | rules/build-output/README.md | # Generated                                                |
    When I inspect directory maps
    Then the exit code is 0
    And 1 directories were inspected

  Scenario Outline: A map section ends at the next heading
    Given the repository contains:
      | path            | content    |
      | rules/README.md | <readme>   |
      | rules/policy.md | # Policy   |
    When I inspect directory maps
    Then the exit code is 0

    Examples:
      | readme                                                                                     |
      | # Rules\n\n## Directory Map\n\n- [Policy](policy.md)\n\n## Notes\n\n- [Ghost](ghost.md)   |
      | # Rules\n\n## Directory Map\n\n- [Policy](policy.md)\n\n# Appendix\n\n- [Ghost](ghost.md) |

  Scenario: A map entry may not reach past a sibling README
    Given the repository contains:
      | path            | content                                                       |
      | rules/README.md | # Rules\n\n## Directory Map\n\n- [Deep](nested/deep/page.md) |
    When I inspect directory maps
    Then the only violation is an invalid map entry from "rules/README.md" to "nested/deep/page.md"

  Scenario: A map entry containing JSON metacharacters survives the JSON rendering
    Given the repository holds a map entry whose target needs escaping
    When I invoke the CLI with "governance|directory-map|validate|--output|json"
    Then the exit code is 1
    And the first stdout JSON violation kind is "invalid-map-entry"
    And stdout escapes the quote, backslash, tab, and control character
