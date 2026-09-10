Feature: Canonical metadata validation

  A repository declares where its governed artifacts live and which of the four
  canonical schemas each surface carries. The schemas themselves are the shared
  contract rather than a repository's preference: governance documents,
  workflows, skills, and agents are the same four everywhere, and a repository
  that could redefine them would have adopted nothing.

  What is a repository's own is the mapping from path to schema, because RHINO
  ships no path. Surfaces are ordered and the last matching glob wins, which is
  how a workflow subtree carries a different schema from the governance tree it
  sits inside.

  Diagnostics here name a rule and a field as well as a place. That is the
  format the standardization contract specifies for structural validation, and
  it is deliberately not the format the validators that shipped before it use:
  a consumer's stored output may not change because a new command arrived.

  Scenario: A governance document carrying both required keys passes
    Given the repository declares the metadata surface "repo-governance/**/*.md" using the "governance" schema
    And file "repo-governance/conventions/file-naming.md" contains this Markdown:
      """
      ---
      description: Defines the durable repository rule for naming one governed file.
      when_to_use: >-
        Use when creating or renaming a governed Markdown document.
      ---

      # File Naming
      """
    When I run the "metadata" validator
    Then the exit code is 0
    And 1 files were inspected

  Scenario: A workflow carries the name its own schema requires
    Given the repository declares the metadata surface "repo-governance/**/*.md" using the "governance" schema
    And the repository declares the metadata surface "repo-governance/workflows/**/*.md" using the "workflow" schema
    And file "repo-governance/workflows/adopt-artifact.md" contains this Markdown:
      """
      ---
      name: adopt-artifact
      description: Adopts explicitly requested catalog artifacts into a repository-owned local form.
      when_to_use: >-
        Use after the user names an artifact or a bounded family to adopt.
      ---

      # Adopt Artifact
      """
    When I run the "metadata" validator
    Then the exit code is 0
    And 1 files were inspected

  Scenario: The path selects the schema, so the same key is right in one tree and wrong in another
    Given the repository declares the metadata surface "repo-governance/**/*.md" using the "governance" schema
    And the repository declares the metadata surface "repo-governance/workflows/**/*.md" using the "workflow" schema
    And file "repo-governance/conventions/file-naming.md" contains this Markdown:
      """
      ---
      name: file-naming
      description: Defines the durable repository rule for naming one governed file.
      when_to_use: >-
        Use when creating or renaming a governed Markdown document.
      ---
      """
    When I run the "metadata" validator
    Then the only violation starts with "repo-governance/conventions/file-naming.md:2:1 metadata-unknown-key name"

  Scenario: A skill declares the one optional key its schema allows
    Given the repository declares the metadata surface ".agents/skills/*/SKILL.md" using the "skill" schema
    And file ".agents/skills/assess-alignment/SKILL.md" contains this Markdown:
      """
      ---
      name: assess-alignment
      description: Compares repository behaviour with the catalog when an alignment assessment is requested.
      when_to_use: >-
        Use for a bounded read-only comparison; do not use to adopt or modify artifacts.
      compatibility: Requires Git and read access to the target repository.
      ---
      """
    When I run the "metadata" validator
    Then the exit code is 0
    And 1 files were inspected

  Scenario: An agent declares both of its optional lists
    Given the repository declares the metadata surface ".agents/agents/*.md" using the "agent" schema
    And file ".agents/agents/plan-checker.md" contains this Markdown:
      """
      ---
      name: plan-checker
      description: Audits project plans for completeness, consistency, safety, and execution readiness.
      when_to_use: >-
        Use after a plan rewrite and before implementation begins.
      tier: plan
      capabilities:
        - repository-read
        - shell
      skills:
        - assess-alignment
      constraints:
        - read-only
      ---
      """
    When I run the "metadata" validator
    Then the exit code is 0
    And 1 files were inspected

  Scenario: A required key that is absent is reported against the artifact
    Given the repository declares the metadata surface "repo-governance/**/*.md" using the "governance" schema
    And file "repo-governance/conventions/file-naming.md" contains this Markdown:
      """
      ---
      description: Defines the durable repository rule for naming one governed file.
      ---
      """
    When I run the "metadata" validator
    Then the only violation starts with "repo-governance/conventions/file-naming.md:1:1 metadata-required-key-missing when_to_use"

  Scenario: An artifact carrying no front matter is missing every required key
    Given the repository declares the metadata surface ".agents/agents/*.md" using the "agent" schema
    And file ".agents/agents/plan-checker.md" contains this Markdown:
      """
      # Plan Checker
      """
    When I run the "metadata" validator
    Then there are 5 violations
    And all violations are "metadata-required-key-missing"

  Scenario: Front matter that opens and never closes is unusable
    Given the repository declares the metadata surface "repo-governance/**/*.md" using the "governance" schema
    And file "repo-governance/conventions/file-naming.md" contains this Markdown:
      """
      ---
      description: Defines the durable repository rule for naming one governed file.

      # File Naming
      """
    When I run the "metadata" validator
    Then the only violation starts with "repo-governance/conventions/file-naming.md:1:1 metadata-frontmatter-unterminated"

  Scenario: A duplicate key fails before a decoder could discard one of them
    Given the repository declares the metadata surface "repo-governance/**/*.md" using the "governance" schema
    And file "repo-governance/conventions/file-naming.md" contains this Markdown:
      """
      ---
      description: Defines the durable repository rule for naming one governed file.
      description: Defines something else entirely for the same governed file.
      when_to_use: >-
        Use when creating or renaming a governed Markdown document.
      ---
      """
    When I run the "metadata" validator
    Then the only violation starts with "repo-governance/conventions/file-naming.md:3:1 metadata-duplicate-key description"

  Scenario: A key the path already supplies is refused
    Given the repository declares the metadata surface "repo-governance/**/*.md" using the "governance" schema
    And file "repo-governance/conventions/file-naming.md" contains this Markdown:
      """
      ---
      description: Defines the durable repository rule for naming one governed file.
      when_to_use: >-
        Use when creating or renaming a governed Markdown document.
      category: conventions
      last_updated: 2026-09-11
      ---
      """
    When I run the "metadata" validator
    Then there are 2 violations
    And all violations are "metadata-unknown-key"

  Scenario: An explicit null is not a value
    Given the repository declares the metadata surface "repo-governance/**/*.md" using the "governance" schema
    And file "repo-governance/conventions/file-naming.md" contains this Markdown:
      """
      ---
      description: null
      when_to_use: >-
        Use when creating or renaming a governed Markdown document.
      ---
      """
    When I run the "metadata" validator
    Then the only violation starts with "repo-governance/conventions/file-naming.md:2:1 metadata-null-value description"

  Scenario: An empty value is not a value either
    Given the repository declares the metadata surface "repo-governance/**/*.md" using the "governance" schema
    And file "repo-governance/conventions/file-naming.md" contains this Markdown:
      """
      ---
      description:
      when_to_use: >-
        Use when creating or renaming a governed Markdown document.
      ---
      """
    When I run the "metadata" validator
    Then the only violation starts with "repo-governance/conventions/file-naming.md:2:1 metadata-empty-value description"

  Scenario: Canonical key order is enforced rather than preferred
    Given the repository declares the metadata surface "repo-governance/workflows/**/*.md" using the "workflow" schema
    And file "repo-governance/workflows/adopt-artifact.md" contains this Markdown:
      """
      ---
      description: Adopts explicitly requested catalog artifacts into a repository-owned local form.
      name: adopt-artifact
      when_to_use: >-
        Use after the user names an artifact or a bounded family to adopt.
      ---
      """
    When I run the "metadata" validator
    Then the only violation starts with "repo-governance/workflows/adopt-artifact.md:3:1 metadata-key-order name"

  Scenario: A description shorter than the floor is a description that says nothing
    Given the repository declares the metadata surface "repo-governance/**/*.md" using the "governance" schema
    And file "repo-governance/conventions/file-naming.md" contains this Markdown:
      """
      ---
      description: Names files.
      when_to_use: >-
        Use when creating or renaming a governed Markdown document.
      ---
      """
    When I run the "metadata" validator
    Then the only violation starts with "repo-governance/conventions/file-naming.md:2:1 metadata-description-length description"

  Scenario: A routing trigger written as a plain scalar is refused
    Given the repository declares the metadata surface "repo-governance/**/*.md" using the "governance" schema
    And file "repo-governance/conventions/file-naming.md" contains this Markdown:
      """
      ---
      description: Defines the durable repository rule for naming one governed file.
      when_to_use: Use when creating or renaming a governed Markdown document.
      ---
      """
    When I run the "metadata" validator
    Then the only violation starts with "repo-governance/conventions/file-naming.md:3:1 metadata-folded-scalar-required when_to_use"

  Scenario: A trigger that repeats the description routes nothing
    Given the repository declares the metadata surface "repo-governance/**/*.md" using the "governance" schema
    And file "repo-governance/conventions/file-naming.md" contains this Markdown:
      """
      ---
      description: Defines the durable repository rule for naming one governed file.
      when_to_use: >-
        Defines the durable repository rule for naming one governed file.
      ---
      """
    When I run the "metadata" validator
    Then the only violation starts with "repo-governance/conventions/file-naming.md:3:1 metadata-routing-not-distinct when_to_use"

  Scenario: A declared name that is not the path identity fails
    Given the repository declares the metadata surface ".agents/skills/*/SKILL.md" using the "skill" schema
    And file ".agents/skills/assess-alignment/SKILL.md" contains this Markdown:
      """
      ---
      name: alignment-assessor
      description: Compares repository behaviour with the catalog when an alignment assessment is requested.
      when_to_use: >-
        Use for a bounded read-only comparison; do not use to adopt or modify artifacts.
      ---
      """
    When I run the "metadata" validator
    Then the only violation starts with ".agents/skills/assess-alignment/SKILL.md:2:1 metadata-name-path-mismatch name"

  Scenario: A workflow entrypoint README takes its name from its directory
    Given the repository declares the metadata surface "repo-governance/workflows/**/*.md" using the "workflow" schema
    And file "repo-governance/workflows/plan/README.md" contains this Markdown:
      """
      ---
      name: plan
      description: Indexes the planning workflows a repository runs from first draft to archived plan.
      when_to_use: >-
        Use when choosing which planning workflow a task needs.
      ---
      """
    When I run the "metadata" validator
    Then the exit code is 0
    And 1 files were inspected

  Scenario: A tier outside the closed set is not a tier
    Given the repository declares the metadata surface ".agents/agents/*.md" using the "agent" schema
    And file ".agents/agents/plan-checker.md" contains this Markdown:
      """
      ---
      name: plan-checker
      description: Audits project plans for completeness, consistency, safety, and execution readiness.
      when_to_use: >-
        Use after a plan rewrite and before implementation begins.
      tier: opus
      capabilities:
        - repository-read
      ---
      """
    When I run the "metadata" validator
    Then the only violation starts with ".agents/agents/plan-checker.md:6:1 metadata-tier-unknown tier"

  Scenario: A capability outside the portable vocabulary is refused
    Given the repository declares the metadata surface ".agents/agents/*.md" using the "agent" schema
    And file ".agents/agents/plan-checker.md" contains this Markdown:
      """
      ---
      name: plan-checker
      description: Audits project plans for completeness, consistency, safety, and execution readiness.
      when_to_use: >-
        Use after a plan rewrite and before implementation begins.
      tier: plan
      capabilities:
        - repository-read
        - websearch
      ---
      """
    When I run the "metadata" validator
    Then the only violation starts with ".agents/agents/plan-checker.md:9:1 metadata-capability-unknown capabilities"

  Scenario: Capabilities are listed in the canonical order rather than the author's
    Given the repository declares the metadata surface ".agents/agents/*.md" using the "agent" schema
    And file ".agents/agents/plan-checker.md" contains this Markdown:
      """
      ---
      name: plan-checker
      description: Audits project plans for completeness, consistency, safety, and execution readiness.
      when_to_use: >-
        Use after a plan rewrite and before implementation begins.
      tier: plan
      capabilities:
        - shell
        - repository-read
      ---
      """
    When I run the "metadata" validator
    Then the only violation starts with ".agents/agents/plan-checker.md:9:1 metadata-array-order capabilities"

  Scenario: An empty required array is an absent answer
    Given the repository declares the metadata surface ".agents/agents/*.md" using the "agent" schema
    And file ".agents/agents/plan-checker.md" contains this Markdown:
      """
      ---
      name: plan-checker
      description: Audits project plans for completeness, consistency, safety, and execution readiness.
      when_to_use: >-
        Use after a plan rewrite and before implementation begins.
      tier: plan
      capabilities: []
      ---
      """
    When I run the "metadata" validator
    Then the only violation starts with ".agents/agents/plan-checker.md:7:1 metadata-array-empty capabilities"

  Scenario: A repeated array member is refused
    Given the repository declares the metadata surface ".agents/agents/*.md" using the "agent" schema
    And file ".agents/agents/plan-checker.md" contains this Markdown:
      """
      ---
      name: plan-checker
      description: Audits project plans for completeness, consistency, safety, and execution readiness.
      when_to_use: >-
        Use after a plan rewrite and before implementation begins.
      tier: plan
      capabilities:
        - repository-read
        - repository-read
      ---
      """
    When I run the "metadata" validator
    Then the only violation starts with ".agents/agents/plan-checker.md:9:1 metadata-array-duplicate capabilities"

  Scenario: A file outside every declared surface is not inspected
    Given the repository declares the metadata surface "repo-governance/**/*.md" using the "governance" schema
    And file "plans/in-progress/some-plan/README.md" contains this Markdown:
      """
      # Some Plan
      """
    When I run the "metadata" validator
    Then the exit code is 0
    And no Markdown files are scanned

  Scenario: Diagnostics sort by path, then rule, then field
    Given the repository declares the metadata surface "repo-governance/**/*.md" using the "governance" schema
    And file "repo-governance/b-second.md" contains this Markdown:
      """
      ---
      when_to_use: >-
        Use when creating or renaming a governed Markdown document.
      ---
      """
    And file "repo-governance/a-first.md" contains this Markdown:
      """
      ---
      description: Defines the durable repository rule for naming one governed file.
      when_to_use: >-
        Use when creating or renaming a governed Markdown document.
      title: A First
      version: 3
      ---
      """
    When I run the "metadata" validator
    Then there are 3 violations
    And the violations are ordinally sorted

  Scenario: The metadata section is optional and its absence is refused rather than assumed
    Given the repository declares a complete configuration
    And the configuration omits "metadata"
    When I run the "metadata" validator
    Then the exit code is 2
    And stderr contains "the section is not declared"

  Scenario: A trigger of more than three sentences stops being a trigger
    Given the repository declares the metadata surface "repo-governance/**/*.md" using the "governance" schema
    And file "repo-governance/conventions/file-naming.md" contains this Markdown:
      """
      ---
      description: Defines the durable repository rule for naming one governed file.
      when_to_use: >-
        Use when creating a file. Use when renaming one. Use when splitting one. Use when deleting one.
      ---
      """
    When I run the "metadata" validator
    Then the only violation starts with "repo-governance/conventions/file-naming.md:3:1 metadata-when-to-use-sentences when_to_use"

  Scenario: A name that is not a portable identifier is refused before it is compared
    Given the repository declares the metadata surface "repo-governance/workflows/**/*.md" using the "workflow" schema
    And file "repo-governance/workflows/adopt-artifact.md" contains this Markdown:
      """
      ---
      name: Adopt_Artifact
      description: Adopts explicitly requested catalog artifacts into a repository-owned local form.
      when_to_use: >-
        Use after the user names an artifact or a bounded family to adopt.
      ---
      """
    When I run the "metadata" validator
    Then the only violation starts with "repo-governance/workflows/adopt-artifact.md:2:1 metadata-name-format name"

  Scenario: A list key written as one value is refused
    Given the repository declares the metadata surface ".agents/agents/*.md" using the "agent" schema
    And file ".agents/agents/plan-checker.md" contains this Markdown:
      """
      ---
      name: plan-checker
      description: Audits project plans for completeness, consistency, safety, and execution readiness.
      when_to_use: >-
        Use after a plan rewrite and before implementation begins.
      tier: plan
      capabilities: repository-read
      ---
      """
    When I run the "metadata" validator
    Then the only violation starts with ".agents/agents/plan-checker.md:7:1 metadata-array-required capabilities"

  Scenario: A single-value key written as a list is refused
    Given the repository declares the metadata surface "repo-governance/**/*.md" using the "governance" schema
    And file "repo-governance/conventions/file-naming.md" contains this Markdown:
      """
      ---
      description:
        - Defines the durable repository rule for naming one governed file.
      when_to_use: >-
        Use when creating or renaming a governed Markdown document.
      ---
      """
    When I run the "metadata" validator
    Then the only violation starts with "repo-governance/conventions/file-naming.md:2:1 metadata-scalar-required description"

  Scenario: A description written as a literal block is refused
    Given the repository declares the metadata surface "repo-governance/**/*.md" using the "governance" schema
    And file "repo-governance/conventions/file-naming.md" contains this Markdown:
      """
      ---
      description: |
        Defines the durable repository rule for naming one governed file.
      when_to_use: >-
        Use when creating or renaming a governed Markdown document.
      ---
      """
    When I run the "metadata" validator
    Then the only violation starts with "repo-governance/conventions/file-naming.md:2:1 metadata-description-form description"

  Scenario: A line inside the block that declares nothing is reported
    Given the repository declares the metadata surface "repo-governance/**/*.md" using the "governance" schema
    And file "repo-governance/conventions/file-naming.md" contains this Markdown:
      """
      ---
      description: Defines the durable repository rule for naming one governed file.
      this line declares nothing
      when_to_use: >-
        Use when creating or renaming a governed Markdown document.
      ---
      """
    When I run the "metadata" validator
    Then the only violation starts with "repo-governance/conventions/file-naming.md:3:1 metadata-frontmatter-malformed"

  Scenario: A horizontal rule further down the document is not front matter
    Given the repository declares the metadata surface "repo-governance/**/*.md" using the "governance" schema
    And file "repo-governance/conventions/file-naming.md" contains this Markdown:
      """
      # File Naming

      One rule about naming.

      ---

      A closing note.
      """
    When I run the "metadata" validator
    Then there are 2 violations
    And all violations are "file-naming.md:1:1 metadata-required-key-missing"

  Scenario: A blank line between declarations is not a declaration
    Given the repository declares the metadata surface "repo-governance/**/*.md" using the "governance" schema
    And file "repo-governance/conventions/file-naming.md" contains this Markdown:
      """
      ---
      description: Defines the durable repository rule for naming one governed file.

      when_to_use: >-
        Use when creating or renaming a governed Markdown document.
      ---
      """
    When I run the "metadata" validator
    Then the exit code is 0
    And 1 files were inspected

  Scenario: A quoted value is the value it quotes
    Given the repository declares the metadata surface ".agents/agents/*.md" using the "agent" schema
    And file ".agents/agents/plan-checker.md" contains this Markdown:
      """
      ---
      name: "plan-checker"
      description: Audits project plans for completeness, consistency, safety, and execution readiness.
      when_to_use: >-
        Use after a plan rewrite and before implementation begins.
      tier: 'plan'
      capabilities:
        - repository-read
      ---
      """
    When I run the "metadata" validator
    Then the exit code is 0
    And 1 files were inspected

  Scenario: A surface glob that cannot compile is a configuration fault
    Given the repository declares the metadata surface "repo-governance/**/[" using the "governance" schema
    When I run the "metadata" validator
    Then the exit code is 2
    And stderr contains "metadata.surfaces"

  Scenario: A file that cannot be read refuses the whole run
    Given the repository declares the metadata surface "repo-governance/**/*.md" using the "governance" schema
    And file "repo-governance/conventions/file-naming.md" cannot be opened
    When I run the "metadata" validator
    Then the exit code is 2
    And stderr names a file it could not read

  Scenario: The JSON form carries the column and the field a finding is about
    Given the repository declares the metadata surface "repo-governance/**/*.md" using the "governance" schema
    And file "repo-governance/conventions/file-naming.md" contains this Markdown:
      """
      ---
      description: Defines the durable repository rule for naming one governed file.
      when_to_use: >-
        Use when creating or renaming a governed Markdown document.
      title: File Naming
      ---
      """
    When I invoke the CLI with "metadata|validate|--output|json"
    Then the exit code is 1
    And the first stdout JSON violation kind is "metadata-unknown-key"
