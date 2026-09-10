Feature: README-index validation

  Every directory under a declared tree carries a README. Presence only, and
  deliberately weaker than directory-map validation: a repository with a hundred
  READMEs and no directory-map sections can adopt this rung today and the
  stronger one later. Both exist so a repository can choose which it is on.

  Scenario: Every directory under a declared tree carries a README
    Given the repository declares the README-index tree "rules"
    And the repository contains:
      | path                   | content  |
      | rules/README.md        | # Rules  |
      | rules/nested/README.md | # Nested |
      | rules/nested/policy.md | # Policy |
    When I run the "readme-index" validator
    Then the exit code is 0
    And 2 directories were inspected

  Scenario: A directory with no README is a finding
    Given the repository declares the README-index tree "rules"
    And the repository contains:
      | path                   | content  |
      | rules/README.md        | # Rules  |
      | rules/nested/policy.md | # Policy |
    When I run the "readme-index" validator
    Then the only violation starts with "rules/nested: missing README"

  Scenario: A README with no directory map is still a README
    Given the repository declares the README-index tree "rules"
    And the repository contains:
      | path            | content |
      | rules/README.md | # Rules |
    When I run the "readme-index" validator
    Then the exit code is 0
    And 1 directories were inspected

  Scenario: An excluded directory inside a declared tree is not inspected
    Given the repository declares the README-index tree "rules"
    And the repository declares excluded scan directories:
      * build-output
    And the repository contains:
      | path                         | content     |
      | rules/README.md              | # Rules     |
      | rules/build-output/report.md | # Generated |
    When I run the "readme-index" validator
    Then the exit code is 0
    And 1 directories were inspected

  Scenario: Directories outside every declared tree are not inspected
    Given the repository declares the README-index tree "rules"
    And the repository contains:
      | path               | content |
      | rules/README.md    | # Rules |
      | guides/overview.md | # Guide |
    When I run the "readme-index" validator
    Then the exit code is 0
    And 1 directories were inspected

  Scenario: Each declared tree is walked
    Given the repository declares the README-index tree "rules"
    And the repository declares the README-index tree "guides"
    And the repository contains:
      | path               | content |
      | rules/README.md    | # Rules |
      | guides/overview.md | # Guide |
    When I run the "readme-index" validator
    Then the only violation starts with "guides: missing README"
    And 2 directories were inspected

  Scenario: A declared tree that holds nothing inspects nothing
    Given the repository declares the README-index tree "rules"
    And an empty repository
    When I run the "readme-index" validator
    Then the exit code is 0
    And 0 directories were inspected

  Scenario: Omitting the section refuses rather than assuming a tree
    Given the repository declares a complete configuration
    When I run the "readme-index" validator
    Then the exit code is 2
    And stderr contains "md-readme-index"
