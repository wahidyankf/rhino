Feature: Governance root structure

  A repository's governance tree has five canonical layers and a shared registry
  of categories inside them. A repository needing a category the registry does
  not provide declares it in configuration rather than by creating a directory,
  because inferring categories from directories removes the only moment anyone
  considers whether the category should exist.

  These rules arrived with ose/repo-config/v2 and are checked only for a
  repository that declared it. A v1 repository adopted none of them, and
  enforcing a contract a repository never agreed to is what every optional
  section in this schema exists to prevent.

  Scenario: A tree using only canonical layers and registered categories passes
    Given the repository declares a v2 configuration
    And the repository contains:
      | path                                             | content     |
      | repo-governance/conventions/structure/naming.md  | # Naming    |
      | repo-governance/workflows/plan/plan-execution.md | # Execution |
    When I run the "governance-roots" validator
    Then the exit code is 0
    And there are no violations

  Scenario: A directory that is not a canonical layer is a finding
    Given the repository declares a v2 configuration
    And the repository contains:
      | path                                            | content  |
      | repo-governance/conventions/structure/naming.md | # Naming |
      | repo-governance/guidelines/structure/naming.md  | # Naming |
    When I run the "governance-roots" validator
    Then the only violation starts with "repo-governance/guidelines: is not a governed layer"

  Scenario: The five canonical layers are all accepted
    Given the repository declares a v2 configuration
    And the repository declares the local governance category "development/practice"
    And the repository declares the local governance category "principles/general"
    And the repository declares the local governance category "vision/direction"
    And the repository contains:
      | path                                             | content     |
      | repo-governance/conventions/structure/naming.md  | # Naming    |
      | repo-governance/development/practice/tasks.md    | # Tasks     |
      | repo-governance/principles/general/sufficiency.md | # Enough   |
      | repo-governance/vision/direction/where.md        | # Direction |
      | repo-governance/workflows/plan/plan-execution.md | # Execution |
    When I run the "governance-roots" validator
    Then the exit code is 0

  Scenario: A category outside the shared registry and undeclared is a finding
    Given the repository declares a v2 configuration
    And the repository contains:
      | path                                          | content |
      | repo-governance/development/practice/tasks.md | # Tasks |
    When I run the "governance-roots" validator
    Then the only violation starts with "repo-governance/development/practice: is not a registered category"

  Scenario: The same category declared under governance.local-categories passes
    Given the repository declares a v2 configuration
    And the repository declares the local governance category "development/practice"
    And the repository contains:
      | path                                          | content |
      | repo-governance/development/practice/tasks.md | # Tasks |
    When I run the "governance-roots" validator
    Then the exit code is 0
    And there are no violations

  Scenario: A declared category the repository never used is a finding
    Given the repository declares a v2 configuration
    And the repository declares the local governance category "development/practice"
    And the repository contains:
      | path                                            | content  |
      | repo-governance/conventions/structure/naming.md | # Naming |
    When I run the "governance-roots" validator
    Then the only violation starts with "development/practice: is declared and holds nothing"

  Scenario: A declaration is not a directory, so an empty one is still reported once
    Given the repository declares a v2 configuration
    And the repository declares the local governance category "development/practice"
    And the repository contains the empty directory "repo-governance/development/practice"
    And the repository contains:
      | path                                            | content  |
      | repo-governance/conventions/structure/naming.md | # Naming |
    When I run the "governance-roots" validator
    Then the only violation starts with "development/practice: is declared and holds nothing"

  Scenario: A governed directory holding nothing is a finding
    Given the repository declares a v2 configuration
    And the repository contains the empty directory "repo-governance/workflows/plan"
    And the repository contains:
      | path                                            | content  |
      | repo-governance/conventions/structure/naming.md | # Naming |
    When I run the "governance-roots" validator
    Then the only violation starts with "repo-governance/workflows/plan: holds no file"

  Scenario: An empty directory below a category is a finding too
    Given the repository declares a v2 configuration
    And the repository contains the empty directory "repo-governance/conventions/structure/naming"
    And the repository contains:
      | path                                            | content  |
      | repo-governance/conventions/structure/naming.md | # Naming |
    When I run the "governance-roots" validator
    Then the only violation starts with "repo-governance/conventions/structure/naming: holds no file"

  Scenario: An empty layer directory is reported as empty rather than as a category
    Given the repository declares a v2 configuration
    And the repository contains the empty directory "repo-governance/vision"
    And the repository contains:
      | path                                            | content  |
      | repo-governance/conventions/structure/naming.md | # Naming |
    When I run the "governance-roots" validator
    Then the only violation starts with "repo-governance/vision: holds no file"

  Scenario: A companion directory beside its document is not a category
    Given the repository declares a v2 configuration
    And the repository contains:
      | path                                                    | content    |
      | repo-governance/conventions/structure.md                | # Struct   |
      | repo-governance/conventions/structure/001-naming.md     | # Naming   |
    When I run the "governance-roots" validator
    Then the exit code is 0
    And there are no violations

  Scenario: A category holding a file directly is a live category
    Given the repository declares a v2 configuration
    And the repository contains:
      | path                                     | content  |
      | repo-governance/conventions/security.md  | # Secure |
    When I run the "governance-roots" validator
    Then the exit code is 0
    And there are no violations

  Scenario: A file directly under a layer is not a category
    Given the repository declares a v2 configuration
    And the repository contains:
      | path                             | content   |
      | repo-governance/workflows/README.md | # Index |
      | repo-governance/conventions/structure/naming.md | # Naming |
    When I run the "governance-roots" validator
    Then the exit code is 0

  Scenario: A repository with no governance tree has nothing to report
    Given the repository declares a v2 configuration
    And an empty repository
    When I run the "governance-roots" validator
    Then the exit code is 0
    And there are no violations

  Scenario: Findings are reported in path order
    Given the repository declares a v2 configuration
    And the repository contains:
      | path                                      | content |
      | repo-governance/zeta/structure/naming.md  | # Zeta  |
      | repo-governance/alpha/structure/naming.md | # Alpha |
    When I run the "governance-roots" validator
    Then there are 2 violations
    And the formatted violation starts with "repo-governance/alpha"

  Scenario: Two categories meaning the same thing under different words both pass
    Given the repository declares a v2 configuration
    And the repository declares the local governance category "workflows/planning"
    And the repository contains:
      | path                                             | content     |
      | repo-governance/workflows/plan/plan-execution.md | # Execution |
      | repo-governance/workflows/planning/grooming.md   | # Grooming  |
    When I run the "governance-roots" validator
    Then the exit code is 0
    And there are no violations

  Scenario: A v1 repository is refused rather than held to a contract it never adopted
    Given the repository declares a complete configuration
    And the repository contains:
      | path                                          | content |
      | repo-governance/development/practice/tasks.md | # Tasks |
    When I run the "governance-roots" validator
    Then the exit code is 2
    And stderr contains "ose/repo-config/v2"

  Scenario: A file directly under the governance root is not a layer
    Given the repository declares a v2 configuration
    And the repository contains:
      | path                                            | content  |
      | repo-governance/README.md                       | # Index  |
      | repo-governance/conventions/structure/naming.md | # Naming |
    When I run the "governance-roots" validator
    Then the exit code is 0
    And there are no violations
