Feature: Plan structure

  A formal plan is six documents and exactly one technical shape, kept under a
  lifecycle root whose name and slug form say where the plan is in its life.
  More than one implementation validates that shape, so the rules, their
  identifiers, their diagnostics, and their exit classes are frozen in a shared
  contract and exercised against a shared fixture corpus.

  What RHINO does not judge is the plan. Whether the writing is clear, whether
  the approach is sound, whether a checklist item is granular enough: none of
  those is visible in the shape, and a rule that guessed at them would be
  enforced in cases nobody considered.

  Diagnostics are `path:line:column rule field message`, sorted by path, line,
  column, rule, then field. A finding about a path rather than a position in a
  file is reported at `1:1`, and `field` is `-` where no element is named.

  These rules arrived with ose/repo-config/v2 and are checked only for a
  repository that declared it.

  Scenario: A conforming plan in the single-file shape passes
    Given the repository declares a v2 configuration
    And the repository holds a conforming plan at "plans/backlog/tidy-the-corpus"
    When I run the "plan" validator
    Then the exit code is 0
    And there are no violations

  Scenario: A conforming plan in the directory shape passes
    Given the repository declares a v2 configuration
    And the repository holds a conforming plan at "plans/in-progress/split-the-shape"
    And the plan at "plans/in-progress/split-the-shape" uses the directory technical shape
    When I run the "plan" validator
    Then the exit code is 0
    And there are no violations

  Scenario: A completed plan carrying a completion date passes
    Given the repository declares a v2 configuration
    And the repository holds a conforming plan at "plans/done/2026-01-15__finish-the-sample"
    When I run the "plan" validator
    Then the exit code is 0
    And there are no violations

  Scenario: An idea brief is not a formal plan
    Given the repository declares a v2 configuration
    And the repository contains:
      | path                                | content            |
      | plans/ideas/consider-a-shorter-run.md | # Consider it     |
    When I run the "plan" validator
    Then the exit code is 0
    And there are no violations

  Scenario: A lifecycle root outside the four canonical names is a finding
    Given the repository declares a v2 configuration
    And the repository holds a conforming plan at "plans/completed/tidy-the-corpus"
    When I run the "plan" validator
    Then the exit code is 1
    And the only violation starts with "plans/completed:1:1 PLAN-LIFECYCLE-001 completed"

  Scenario: A live plan slug carrying a date is a finding
    Given the repository declares a v2 configuration
    And the repository holds a conforming plan at "plans/backlog/2026-01-15__tidy-the-corpus"
    When I run the "plan" validator
    Then the only violation starts with "plans/backlog/2026-01-15__tidy-the-corpus:1:1 PLAN-LIFECYCLE-002 2026-01-15__tidy-the-corpus"

  Scenario: A completed plan slug with no completion date is a finding
    Given the repository declares a v2 configuration
    And the repository holds a conforming plan at "plans/done/finish-the-sample"
    When I run the "plan" validator
    Then the only violation starts with "plans/done/finish-the-sample:1:1 PLAN-LIFECYCLE-003 finish-the-sample"

  Scenario: A plan slug that is not lowercase hyphen-separated is a finding
    Given the repository declares a v2 configuration
    And the repository holds a conforming plan at "plans/backlog/Tidy_The_Corpus"
    When I run the "plan" validator
    Then the only violation starts with "plans/backlog/Tidy_The_Corpus:1:1 PLAN-LIFECYCLE-004 Tidy_The_Corpus"

  Scenario: One slug occupying two lifecycle roots is a finding
    Given the repository declares a v2 configuration
    And the repository holds a conforming plan at "plans/backlog/tidy-the-corpus"
    And the repository holds a conforming plan at "plans/in-progress/tidy-the-corpus"
    When I run the "plan" validator
    Then the only violation starts with "plans/in-progress/tidy-the-corpus:1:1 PLAN-LIFECYCLE-005 tidy-the-corpus"

  Scenario: A missing plan document is a finding naming the document
    Given the repository declares a v2 configuration
    And the repository holds a conforming plan at "plans/backlog/tidy-the-corpus"
    And the plan at "plans/backlog/tidy-the-corpus" has no "prd.md"
    When I run the "plan" validator
    Then the only violation starts with "plans/backlog/tidy-the-corpus:1:1 PLAN-DOCUMENT-001 prd.md"

  Scenario: Each missing document is named once
    Given the repository declares a v2 configuration
    And the repository holds a conforming plan at "plans/backlog/tidy-the-corpus"
    And the plan at "plans/backlog/tidy-the-corpus" has no "learnings.md"
    And the plan at "plans/backlog/tidy-the-corpus" has no "delivery.md"
    When I run the "plan" validator
    Then there are 2 violations
    And all violations are "PLAN-DOCUMENT-001"

  Scenario: A plan carrying both technical shapes is a finding
    Given the repository declares a v2 configuration
    And the repository holds a conforming plan at "plans/backlog/tidy-the-corpus"
    And the plan at "plans/backlog/tidy-the-corpus" also carries a directory technical shape
    When I run the "plan" validator
    Then the only violation starts with "plans/backlog/tidy-the-corpus:1:1 PLAN-DOCUMENT-002 tech-docs"

  Scenario: A plan carrying no technical shape is a finding
    Given the repository declares a v2 configuration
    And the repository holds a conforming plan at "plans/backlog/tidy-the-corpus"
    And the plan at "plans/backlog/tidy-the-corpus" has no "tech-docs.md"
    When I run the "plan" validator
    Then the only violation starts with "plans/backlog/tidy-the-corpus:1:1 PLAN-DOCUMENT-003 tech-docs"

  Scenario: An archived plan is held to its lifecycle form and not to its contents
    Given the repository declares a v2 configuration
    And the repository holds a conforming plan at "plans/done/2026-01-15__finish-the-sample"
    And the plan at "plans/done/2026-01-15__finish-the-sample" has no "learnings.md"
    When I run the "plan" validator
    Then the exit code is 0
    And there are no violations

  Scenario: A companion ordinal that is not three digits is a finding
    Given the repository declares a v2 configuration
    And the repository holds a conforming plan at "plans/backlog/tidy-the-corpus"
    And the plan at "plans/backlog/tidy-the-corpus" uses the directory technical shape
    And the companion "001-context.md" is renamed to "01-context.md"
    When I run the "plan" validator
    Then the only violation starts with "plans/backlog/tidy-the-corpus/tech-docs/01-context.md:1:1 PLAN-COMPANION-001 01"

  Scenario: Companion ordinals with a gap are a finding
    Given the repository declares a v2 configuration
    And the repository holds a conforming plan at "plans/backlog/tidy-the-corpus"
    And the plan at "plans/backlog/tidy-the-corpus" uses the directory technical shape
    And the companion "002-approach.md" is renamed to "003-approach.md"
    When I run the "plan" validator
    Then the only violation starts with "plans/backlog/tidy-the-corpus/tech-docs:1:1 PLAN-COMPANION-002 -"

  Scenario: One ordinal used twice is a finding
    Given the repository declares a v2 configuration
    And the repository holds a conforming plan at "plans/backlog/tidy-the-corpus"
    And the plan at "plans/backlog/tidy-the-corpus" uses the directory technical shape
    And the companion "002-approach.md" is renamed to "001-approach.md"
    When I run the "plan" validator
    Then the only violation starts with "plans/backlog/tidy-the-corpus/tech-docs:1:1 PLAN-COMPANION-003 001"

  Scenario: A companion name that is not kebab-case after its ordinal is a finding
    Given the repository declares a v2 configuration
    And the repository holds a conforming plan at "plans/backlog/tidy-the-corpus"
    And the plan at "plans/backlog/tidy-the-corpus" uses the directory technical shape
    And the companion "002-approach.md" is renamed to "002-Approach_Notes.md"
    When I run the "plan" validator
    Then the only violation starts with "plans/backlog/tidy-the-corpus/tech-docs/002-Approach_Notes.md:1:1 PLAN-COMPANION-004 Approach_Notes"

  Scenario: A companion the entrypoint does not list is a finding
    Given the repository declares a v2 configuration
    And the repository holds a conforming plan at "plans/backlog/tidy-the-corpus"
    And the plan at "plans/backlog/tidy-the-corpus" uses the directory technical shape
    And the companion set at "plans/backlog/tidy-the-corpus/tech-docs" holds "003-rollout.md"
    When I run the "plan" validator
    Then the only violation starts with "plans/backlog/tidy-the-corpus/tech-docs/003-rollout.md:1:1 PLAN-COMPANION-005 -"

  Scenario: An entrypoint listing a companion that does not exist is a finding
    Given the repository declares a v2 configuration
    And the repository holds a conforming plan at "plans/backlog/tidy-the-corpus"
    And the plan at "plans/backlog/tidy-the-corpus" uses the directory technical shape
    And the companion index at "plans/backlog/tidy-the-corpus/tech-docs" lists "003-rollout.md"
    When I run the "plan" validator
    Then the only violation starts with "plans/backlog/tidy-the-corpus/tech-docs/README.md:7:1 PLAN-COMPANION-006 003-rollout.md"

  Scenario: Contiguity is not reported for a set whose ordinals are already malformed
    Given the repository declares a v2 configuration
    And the repository holds a conforming plan at "plans/backlog/tidy-the-corpus"
    And the plan at "plans/backlog/tidy-the-corpus" uses the directory technical shape
    And the companion "001-context.md" is renamed to "01-context.md"
    When I run the "plan" validator
    Then there are 1 violations
    And all violations are "PLAN-COMPANION-001"

  Scenario: An acceptance identifier defined twice is a finding
    Given the repository declares a v2 configuration
    And the repository holds a conforming plan at "plans/backlog/tidy-the-corpus"
    And file "plans/backlog/tidy-the-corpus/prd.md" contains this Markdown:
      """
      # Product Requirements

      ## Acceptance Criteria

      ```gherkin
      Feature: Fixture criteria

        Scenario: [AC-01] Condition 1
          Given a fixture repository
          When the validator runs
          Then it reads this scenario

        Scenario: [AC-01] Condition 2
          Given a fixture repository
          When the validator runs
          Then it reads this scenario
      ```
      """
    When I run the "plan" validator
    Then the only violation starts with "plans/backlog/tidy-the-corpus/prd.md:13:1 PLAN-CRITERION-001 AC-01"

  Scenario: A delivery item citing an undefined acceptance identifier is a finding
    Given the repository declares a v2 configuration
    And the repository holds a conforming plan at "plans/backlog/tidy-the-corpus"
    And file "plans/backlog/tidy-the-corpus/delivery.md" contains this Markdown:
      """
      # Delivery

      ## Phase 1: Build

      - [AI] Do the thing this phase names. `[AC-09]`

      ## Plan Archival

      - [AI] Move the plan folder to the done root.
      """
    When I run the "plan" validator
    Then the only violation starts with "plans/backlog/tidy-the-corpus/delivery.md:5:1 PLAN-CRITERION-002 AC-09"

  Scenario: A checklist item carrying no executor label is a finding
    Given the repository declares a v2 configuration
    And the repository holds a conforming plan at "plans/backlog/tidy-the-corpus"
    And file "plans/backlog/tidy-the-corpus/delivery.md" contains this Markdown:
      """
      # Delivery

      ## Phase 1: Build

      - Do the thing this phase names. `[AC-01]`

      ## Plan Archival

      - [AI] Move the plan folder to the done root.
      """
    When I run the "plan" validator
    Then the only violation starts with "plans/backlog/tidy-the-corpus/delivery.md:5:1 PLAN-DELIVERY-001 -"

  Scenario: An executor label outside the two permitted values is a finding
    Given the repository declares a v2 configuration
    And the repository holds a conforming plan at "plans/backlog/tidy-the-corpus"
    And file "plans/backlog/tidy-the-corpus/delivery.md" contains this Markdown:
      """
      # Delivery

      ## Phase 1: Build

      - [ROBOT] Do the thing this phase names. `[AC-01]`

      ## Plan Archival

      - [AI] Move the plan folder to the done root.
      """
    When I run the "plan" validator
    Then the only violation starts with "plans/backlog/tidy-the-corpus/delivery.md:5:1 PLAN-DELIVERY-002 ROBOT"

  Scenario: A delivery phase heading with no phase number is a finding
    Given the repository declares a v2 configuration
    And the repository holds a conforming plan at "plans/backlog/tidy-the-corpus"
    And file "plans/backlog/tidy-the-corpus/delivery.md" contains this Markdown:
      """
      # Delivery

      ## Build

      - [AI] Do the thing this phase names. `[AC-01]`

      ## Plan Archival

      - [AI] Move the plan folder to the done root.
      """
    When I run the "plan" validator
    Then the only violation starts with "plans/backlog/tidy-the-corpus/delivery.md:3:1 PLAN-DELIVERY-003 Build"

  Scenario: Archival items before a substantive phase are a finding
    Given the repository declares a v2 configuration
    And the repository holds a conforming plan at "plans/backlog/tidy-the-corpus"
    And file "plans/backlog/tidy-the-corpus/delivery.md" contains this Markdown:
      """
      # Delivery

      ## Plan Archival

      - [AI] Move the plan folder to the done root.

      ## Phase 1: Build

      - [AI] Do the thing this phase names. `[AC-01]`
      """
    When I run the "plan" validator
    Then the only violation starts with "plans/backlog/tidy-the-corpus/delivery.md:3:1 PLAN-DELIVERY-004 -"

  Scenario: A human executor label is one of the two permitted values
    Given the repository declares a v2 configuration
    And the repository holds a conforming plan at "plans/backlog/tidy-the-corpus"
    And file "plans/backlog/tidy-the-corpus/delivery.md" contains this Markdown:
      """
      # Delivery

      ## Phase 1: Build

      - [HUMAN] Decide the thing this phase names. `[AC-01]`

      ## Plan Archival

      - [AI] Move the plan folder to the done root.
      """
    When I run the "plan" validator
    Then the exit code is 0
    And there are no violations

  Scenario: Suppression never crosses families
    Given the repository declares a v2 configuration
    And the repository holds a conforming plan at "plans/backlog/Tidy_The_Corpus"
    And file "plans/backlog/Tidy_The_Corpus/delivery.md" contains this Markdown:
      """
      # Delivery

      ## Build

      - [AI] Do the thing this phase names. `[AC-01]`

      ## Plan Archival

      - [AI] Move the plan folder to the done root.
      """
    When I run the "plan" validator
    Then there are 2 violations
    And the violations are ordinally sorted

  Scenario: Findings are reported in the contract's order
    Given the repository declares a v2 configuration
    And the repository holds a conforming plan at "plans/backlog/tidy-the-corpus"
    And the plan at "plans/backlog/tidy-the-corpus" has no "learnings.md"
    And the plan at "plans/backlog/tidy-the-corpus" has no "brd.md"
    And the repository holds a conforming plan at "plans/backlog/Another_One"
    When I run the "plan" validator
    Then the violations are ordinally sorted

  Scenario: Two runs over unchanged input agree exactly
    Given the repository declares a v2 configuration
    And the repository holds a conforming plan at "plans/backlog/tidy-the-corpus"
    And the plan at "plans/backlog/tidy-the-corpus" has no "learnings.md"
    And the repository holds a conforming plan at "plans/backlog/split-the-shape"
    And the plan at "plans/backlog/split-the-shape" has no "brd.md"
    And the repository holds a conforming plan at "plans/in-progress/retire-the-alias"
    And the plan at "plans/in-progress/retire-the-alias" has no "README.md"
    When I run the "plan" validator twice
    Then the two runs are byte-identical

  Scenario: A repository with no plans tree has nothing to report
    Given the repository declares a v2 configuration
    And an empty repository
    When I run the "plan" validator
    Then the exit code is 0
    And there are no violations

  Scenario: A malformed document outside the plans tree is not this command's
    Given the repository declares a v2 configuration
    And the repository contains:
      | path                  | content     |
      | docs/Tidy_The_Docs.md | # Not a plan |
    When I run the "plan" validator
    Then the exit code is 0
    And there are no violations

  Scenario: A plan document that cannot be read refuses the run
    Given the repository declares a v2 configuration
    And the repository holds a conforming plan at "plans/backlog/tidy-the-corpus"
    And the file "plans/backlog/tidy-the-corpus/delivery.md" cannot be read
    When I run the "plan" validator
    Then the exit code is 2
    And stderr names a file it could not read

  Scenario: A v1 repository is refused rather than held to a contract it never adopted
    Given the repository declares a complete configuration
    When I run the "plan" validator
    Then the exit code is 2
    And stderr contains "ose/repo-config/v2"

  Scenario: A requirements document that cannot be read refuses the run
    Given the repository declares a v2 configuration
    And the repository holds a conforming plan at "plans/backlog/tidy-the-corpus"
    And the file "plans/backlog/tidy-the-corpus/prd.md" cannot be read
    When I run the "plan" validator
    Then the exit code is 2
    And stderr names a file it could not read

  Scenario: A checklist that holds no text refuses the run
    Given the repository declares a v2 configuration
    And the repository holds a conforming plan at "plans/backlog/tidy-the-corpus"
    And file "plans/backlog/tidy-the-corpus/delivery.md" holds bytes that are not text
    When I run the "plan" validator
    Then the exit code is 2
    And stderr contains "holds no text"

  Scenario: A companion index that cannot be read refuses the run
    Given the repository declares a v2 configuration
    And the repository holds a conforming plan at "plans/backlog/tidy-the-corpus"
    And the plan at "plans/backlog/tidy-the-corpus" uses the directory technical shape
    And the file "plans/backlog/tidy-the-corpus/tech-docs/README.md" cannot be read
    When I run the "plan" validator
    Then the exit code is 2
    And stderr names a file it could not read

  Scenario: A companion set with no index reports its modules rather than the index
    Given the repository declares a v2 configuration
    And the repository holds a conforming plan at "plans/backlog/tidy-the-corpus"
    And the plan at "plans/backlog/tidy-the-corpus" uses the directory technical shape
    And the plan at "plans/backlog/tidy-the-corpus" has no "tech-docs/README.md"
    When I run the "plan" validator
    Then there are 2 violations
    And all violations are "PLAN-COMPANION-005"
