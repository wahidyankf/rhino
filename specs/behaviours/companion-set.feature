Feature: Ordered companion sets

  A governed document too large for its budget splits into an entrypoint plus a
  sibling directory of modules named exactly after it. The directory carries a
  README indexing its modules, and its filenames carry ordinal prefixes when,
  and only when, the entrypoint declares a reading order.

  What RHINO does not decide is whether a set genuinely needs ordering. That the
  third module assumes the first is a claim about content, and a validator that
  guessed at it would be asserting that word shapes prove meaning.

  These rules arrived with ose/repo-config/v2 and are checked only for a
  repository that declared it.

  Scenario: An ordered set the entrypoint declares an order for passes
    Given the repository declares a v2 configuration
    And file "repo-governance/conventions/plans.md" contains this Markdown:
      """
      # Plans

      ## Modules

      1. [Lifecycle](plans/001-lifecycle.md)
      2. [Documents](plans/002-documents.md)
      """
    And file "repo-governance/conventions/plans/README.md" contains this Markdown:
      """
      # Plans Modules

      - [Lifecycle](001-lifecycle.md)
      - [Documents](002-documents.md)
      """
    And the repository contains:
      | path                                             | content      |
      | repo-governance/conventions/plans/001-lifecycle.md | # Lifecycle |
      | repo-governance/conventions/plans/002-documents.md | # Documents |
    When I run the "governance-companions" validator
    Then the exit code is 0
    And there are no violations

  Scenario: An unordered set with no declared order and no ordinals passes
    Given the repository declares a v2 configuration
    And file "repo-governance/conventions/security.md" contains this Markdown:
      """
      # Security

      - [Secrets](security/secrets.md)
      - [Scanning](security/scanning.md)
      """
    And file "repo-governance/conventions/security/README.md" contains this Markdown:
      """
      # Security Modules

      - [Secrets](secrets.md)
      - [Scanning](scanning.md)
      """
    And the repository contains:
      | path                                            | content    |
      | repo-governance/conventions/security/secrets.md | # Secrets  |
      | repo-governance/conventions/security/scanning.md | # Scanning |
    When I run the "governance-companions" validator
    Then the exit code is 0
    And there are no violations

  Scenario: A companion directory carrying a suffix is a finding
    Given the repository declares a v2 configuration
    And file "repo-governance/conventions/plans.md" contains this Markdown:
      """
      # Plans

      - [Lifecycle](plans-details/lifecycle.md)
      """
    And the repository contains:
      | path                                                     | content     |
      | repo-governance/conventions/plans-details/README.md      | # Modules   |
      | repo-governance/conventions/plans-details/lifecycle.md   | # Lifecycle |
    When I run the "governance-companions" validator
    Then the only violation starts with "repo-governance/conventions/plans-details: is named after `plans.md` with a suffix"

  Scenario: Every recognized suffix is reported the same way
    Given the repository declares a v2 configuration
    And file "repo-governance/conventions/plans.md" contains this Markdown:
      """
      # Plans

      - [Lifecycle](plans-parts/lifecycle.md)
      """
    And the repository contains:
      | path                                                   | content     |
      | repo-governance/conventions/plans-parts/README.md      | # Modules   |
      | repo-governance/conventions/plans-parts/lifecycle.md   | # Lifecycle |
    When I run the "governance-companions" validator
    Then the only violation starts with "repo-governance/conventions/plans-parts: is named after `plans.md` with a suffix"

  Scenario: A companion directory with no README is a finding
    Given the repository declares a v2 configuration
    And file "repo-governance/conventions/security.md" contains this Markdown:
      """
      # Security

      - [Secrets](security/secrets.md)
      """
    And the repository contains:
      | path                                            | content   |
      | repo-governance/conventions/security/secrets.md | # Secrets |
    When I run the "governance-companions" validator
    Then the only violation starts with "repo-governance/conventions/security: carries no README"

  Scenario: A module with no ordinal in a set declaring an order is a finding
    Given the repository declares a v2 configuration
    And file "repo-governance/conventions/plans.md" contains this Markdown:
      """
      # Plans

      ## Modules

      1. [Lifecycle](plans/001-lifecycle.md)
      2. [Documents](plans/documents.md)
      """
    And file "repo-governance/conventions/plans/README.md" contains this Markdown:
      """
      # Plans Modules

      - [Lifecycle](001-lifecycle.md)
      - [Documents](documents.md)
      """
    And the repository contains:
      | path                                               | content     |
      | repo-governance/conventions/plans/001-lifecycle.md | # Lifecycle |
      | repo-governance/conventions/plans/documents.md     | # Documents |
    When I run the "governance-companions" validator
    Then the only violation starts with "repo-governance/conventions/plans/documents.md: carries no ordinal prefix"

  Scenario: An ordinal in a set declaring no order is a finding
    Given the repository declares a v2 configuration
    And file "repo-governance/conventions/security.md" contains this Markdown:
      """
      # Security

      - [Secrets](security/001-secrets.md)
      """
    And file "repo-governance/conventions/security/README.md" contains this Markdown:
      """
      # Security Modules

      - [Secrets](001-secrets.md)
      """
    And the repository contains:
      | path                                                | content   |
      | repo-governance/conventions/security/001-secrets.md | # Secrets |
    When I run the "governance-companions" validator
    Then the only violation starts with "repo-governance/conventions/security/001-secrets.md: carries an ordinal prefix"

  Scenario: Ordinals that do not start at one are a finding
    Given the repository declares a v2 configuration
    And file "repo-governance/conventions/plans.md" contains this Markdown:
      """
      # Plans

      ## Modules

      1. [Lifecycle](plans/002-lifecycle.md)
      2. [Documents](plans/003-documents.md)
      """
    And file "repo-governance/conventions/plans/README.md" contains this Markdown:
      """
      # Plans Modules

      - [Lifecycle](002-lifecycle.md)
      - [Documents](003-documents.md)
      """
    And the repository contains:
      | path                                               | content     |
      | repo-governance/conventions/plans/002-lifecycle.md | # Lifecycle |
      | repo-governance/conventions/plans/003-documents.md | # Documents |
    When I run the "governance-companions" validator
    Then the only violation starts with "repo-governance/conventions/plans: ordinals are not contiguous from 001"

  Scenario: A gap in the ordinals is a finding
    Given the repository declares a v2 configuration
    And file "repo-governance/conventions/plans.md" contains this Markdown:
      """
      # Plans

      ## Modules

      1. [Lifecycle](plans/001-lifecycle.md)
      2. [Documents](plans/003-documents.md)
      """
    And file "repo-governance/conventions/plans/README.md" contains this Markdown:
      """
      # Plans Modules

      - [Lifecycle](001-lifecycle.md)
      - [Documents](003-documents.md)
      """
    And the repository contains:
      | path                                               | content     |
      | repo-governance/conventions/plans/001-lifecycle.md | # Lifecycle |
      | repo-governance/conventions/plans/003-documents.md | # Documents |
    When I run the "governance-companions" validator
    Then the only violation starts with "repo-governance/conventions/plans: ordinals are not contiguous from 001"

  Scenario: One ordinal used twice is a finding
    Given the repository declares a v2 configuration
    And file "repo-governance/conventions/plans.md" contains this Markdown:
      """
      # Plans

      ## Modules

      1. [Lifecycle](plans/001-lifecycle.md)
      2. [Documents](plans/001-documents.md)
      """
    And file "repo-governance/conventions/plans/README.md" contains this Markdown:
      """
      # Plans Modules

      - [Lifecycle](001-lifecycle.md)
      - [Documents](001-documents.md)
      """
    And the repository contains:
      | path                                               | content     |
      | repo-governance/conventions/plans/001-lifecycle.md | # Lifecycle |
      | repo-governance/conventions/plans/001-documents.md | # Documents |
    When I run the "governance-companions" validator
    Then the only violation starts with "repo-governance/conventions/plans: ordinals are not contiguous from 001"

  Scenario: A module the index does not link is a finding
    Given the repository declares a v2 configuration
    And file "repo-governance/conventions/security.md" contains this Markdown:
      """
      # Security

      - [Secrets](security/secrets.md)
      """
    And file "repo-governance/conventions/security/README.md" contains this Markdown:
      """
      # Security Modules

      - [Secrets](secrets.md)
      """
    And the repository contains:
      | path                                             | content    |
      | repo-governance/conventions/security/secrets.md  | # Secrets  |
      | repo-governance/conventions/security/scanning.md | # Scanning |
    When I run the "governance-companions" validator
    Then the only violation starts with "repo-governance/conventions/security/scanning.md: is not indexed"

  Scenario: An indexed module that is not there is a finding
    Given the repository declares a v2 configuration
    And file "repo-governance/conventions/security.md" contains this Markdown:
      """
      # Security

      - [Secrets](security/secrets.md)
      """
    And file "repo-governance/conventions/security/README.md" contains this Markdown:
      """
      # Security Modules

      - [Secrets](secrets.md)
      - [Scanning](scanning.md)
      """
    And the repository contains:
      | path                                            | content   |
      | repo-governance/conventions/security/secrets.md | # Secrets |
    When I run the "governance-companions" validator
    Then the only violation starts with "repo-governance/conventions/security: indexes `scanning.md`, which is not there"

  Scenario: A directory with no sibling document is not a companion set
    Given the repository declares a v2 configuration
    And the repository contains:
      | path                                             | content   |
      | repo-governance/conventions/security/secrets.md  | # Secrets |
    When I run the "governance-companions" validator
    Then the exit code is 0
    And there are no violations

  Scenario: Three related modules with no declared order are not judged
    Given the repository declares a v2 configuration
    And file "repo-governance/conventions/plans.md" contains this Markdown:
      """
      # Plans

      - [First](plans/lifecycle.md)
      - [Then](plans/documents.md)
      - [Finally](plans/archival.md)
      """
    And file "repo-governance/conventions/plans/README.md" contains this Markdown:
      """
      # Plans Modules

      - [Lifecycle](lifecycle.md)
      - [Documents](documents.md)
      - [Archival](archival.md)
      """
    And the repository contains:
      | path                                           | content     |
      | repo-governance/conventions/plans/lifecycle.md | # Lifecycle |
      | repo-governance/conventions/plans/documents.md | # Documents |
      | repo-governance/conventions/plans/archival.md  | # Archival  |
    When I run the "governance-companions" validator
    Then the exit code is 0
    And there are no violations

  Scenario: A companion set outside the governance tree is not this command's
    Given the repository declares a v2 configuration
    And file "plans/in-progress/example/tech-docs.md" contains this Markdown:
      """
      # Technical Documentation

      - [Shape](tech-docs/shape.md)
      """
    And the repository contains:
      | path                                       | content  |
      | plans/in-progress/example/tech-docs/shape.md | # Shape |
    When I run the "governance-companions" validator
    Then the exit code is 0
    And there are no violations

  Scenario: A v1 repository is refused rather than held to a contract it never adopted
    Given the repository declares a complete configuration
    When I run the "governance-companions" validator
    Then the exit code is 2
    And stderr contains "ose/repo-config/v2"

  Scenario: A repository with no governance tree has nothing to report
    Given the repository declares a v2 configuration
    And an empty repository
    When I run the "governance-companions" validator
    Then the exit code is 0
    And there are no violations
