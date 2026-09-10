Feature: The instruction spine

  Every repository keeps one canonical instruction body in AGENTS.md, opening
  with five top-level sections in a fixed order. Repository-specific sections
  may follow; none may be inserted between spine sections, because a reader
  looking for the same fact in two repositories has to find it in the same
  place.

  CLAUDE.md becomes exactly an import of that body. RHINO checks the terminal
  state and nothing before it: whether every pre-existing statement received a
  disposition is a fact about a file that no longer exists, and a validator
  claiming to have checked it would be claiming to have read history.

  These rules arrived with ose/repo-config/v2 and are checked only for a
  repository that declared it.

  Scenario: A canonical instruction with the five sections in order passes
    Given the repository declares a v2 configuration
    And the repository declares the canonical instruction spine
    When I run the "governance-instructions" validator
    Then the exit code is 0
    And there are no violations

  Scenario: Repository-specific sections may follow the spine
    Given the repository declares a v2 configuration
    And the repository declares the canonical instruction spine
    And the instruction adds the section "Homelab" after the spine
    When I run the "governance-instructions" validator
    Then the exit code is 0
    And there are no violations

  Scenario: A missing spine section is a finding
    Given the repository declares a v2 configuration
    And the repository declares the canonical instruction spine
    And the instruction omits the section "Ownership"
    When I run the "governance-instructions" validator
    Then the only violation starts with "AGENTS.md: declares no `Ownership` section"

  Scenario: Spine sections out of order are a finding
    Given the repository declares a v2 configuration
    And the repository declares the canonical instruction spine
    And the instruction swaps "Ownership" with "Required Workflow"
    When I run the "governance-instructions" validator
    Then the only violation starts with "AGENTS.md: `Required Workflow` is out of spine order"

  Scenario: A section inserted between two spine sections is a finding
    Given the repository declares a v2 configuration
    And the repository declares the canonical instruction spine
    And the instruction inserts the section "Homelab" after "Ownership"
    When I run the "governance-instructions" validator
    Then the only violation starts with "AGENTS.md: `Homelab` is written between spine sections"

  Scenario: A repository with no canonical instruction is a finding
    Given the repository declares a v2 configuration
    And an empty repository
    When I run the "governance-instructions" validator
    Then the only violation starts with "AGENTS.md: is not there"

  Scenario: A CLAUDE.md that is exactly the import passes
    Given the repository declares a v2 configuration
    And the repository declares the canonical instruction spine
    And the repository contains:
      | path      | content   |
      | CLAUDE.md | @AGENTS.md |
    When I run the "governance-instructions" validator
    Then the exit code is 0
    And there are no violations

  Scenario: A CLAUDE.md carrying its own instruction is a finding
    Given the repository declares a v2 configuration
    And the repository declares the canonical instruction spine
    And file "CLAUDE.md" contains this Markdown:
      """
      @AGENTS.md

      Also never run the formatter on Fridays.
      """
    When I run the "governance-instructions" validator
    Then the only violation starts with "CLAUDE.md: is not exactly `@AGENTS.md`"

  Scenario: A CLAUDE.md importing something else is a finding
    Given the repository declares a v2 configuration
    And the repository declares the canonical instruction spine
    And the repository contains:
      | path      | content        |
      | CLAUDE.md | @CONTRIBUTING.md |
    When I run the "governance-instructions" validator
    Then the only violation starts with "CLAUDE.md: is not exactly `@AGENTS.md`"

  Scenario: A repository with no Claude surface needs no CLAUDE.md
    Given the repository declares a v2 configuration
    And the repository declares the canonical instruction spine
    When I run the "governance-instructions" validator
    Then the exit code is 0
    And there are no violations

  Scenario: Surrounding whitespace in the import is not a difference
    Given the repository declares a v2 configuration
    And the repository declares the canonical instruction spine
    And file "CLAUDE.md" contains this Markdown:
      """

      @AGENTS.md

      """
    When I run the "governance-instructions" validator
    Then the exit code is 0
    And there are no violations

  Scenario: A spine heading written at the wrong level is not a spine section
    Given the repository declares a v2 configuration
    And the repository declares the canonical instruction spine
    And the instruction demotes the section "Harnesses"
    When I run the "governance-instructions" validator
    Then the only violation starts with "AGENTS.md: declares no `Harnesses` section"

  Scenario: A spine section named inside a fenced example is not a section
    Given the repository declares a v2 configuration
    And the repository declares the canonical instruction spine
    And the instruction omits the section "Harnesses"
    And the instruction shows "## Harnesses" inside a fenced example
    When I run the "governance-instructions" validator
    Then the only violation starts with "AGENTS.md: declares no `Harnesses` section"

  Scenario: An unreadable canonical instruction refuses the run
    Given the repository declares a v2 configuration
    And the repository declares the canonical instruction spine
    And the file "AGENTS.md" cannot be read
    When I run the "governance-instructions" validator
    Then the exit code is 2
    And stderr names a file it could not read

  Scenario: A v1 repository is refused rather than held to a contract it never adopted
    Given the repository declares a complete configuration
    When I run the "governance-instructions" validator
    Then the exit code is 2
    And stderr contains "ose/repo-config/v2"

  Scenario: A canonical instruction that holds no text refuses the run
    Given the repository declares a v2 configuration
    And file "AGENTS.md" holds bytes that are not text
    When I run the "governance-instructions" validator
    Then the exit code is 2
    And stderr contains "holds no text"

  Scenario: An import that cannot be read refuses the run
    Given the repository declares a v2 configuration
    And the repository declares the canonical instruction spine
    And the repository contains:
      | path      | content    |
      | CLAUDE.md | @AGENTS.md |
    And the file "CLAUDE.md" cannot be read
    When I run the "governance-instructions" validator
    Then the exit code is 2
    And stderr names a file it could not read
