Feature: Markdown heading-hierarchy validation

  A repository declares which files carry a governed heading structure, whether
  each holds exactly one level-1 heading, and how far a heading may drop below
  its predecessor. Both rules are the repository's numbers: the section is
  optional, and neither value has a default in the binary.

  Scenario: A hierarchy that descends one level at a time passes
    Given the repository declares the heading surface "rules/**/*.md" with one H1 and a maximum jump of 1
    And file "rules/policy.md" contains this Markdown:
      """
      # Policy

      ## Scope

      ### Exceptions

      ## Review
      """
    When I run the "heading-hierarchy" validator
    Then the exit code is 0
    And 1 files were inspected

  Scenario: A governed file with no level-1 heading is a finding
    Given the repository declares the heading surface "rules/**/*.md" with one H1 and a maximum jump of 1
    And file "rules/policy.md" contains this Markdown:
      """
      ## Scope

      ## Review
      """
    When I run the "heading-hierarchy" validator
    Then the only violation starts with "rules/policy.md:1: declares no level-1 heading"

  Scenario: A second level-1 heading is a finding at the line it appears on
    Given the repository declares the heading surface "rules/**/*.md" with one H1 and a maximum jump of 1
    And file "rules/policy.md" contains this Markdown:
      """
      # Policy

      # Second Policy
      """
    When I run the "heading-hierarchy" validator
    Then the only violation starts with "rules/policy.md:3: declares a second level-1 heading"

  Scenario: A heading dropping further than the declared jump is a finding
    Given the repository declares the heading surface "rules/**/*.md" with one H1 and a maximum jump of 1
    And file "rules/policy.md" contains this Markdown:
      """
      # Policy

      ### Exceptions
      """
    When I run the "heading-hierarchy" validator
    Then the only violation starts with "rules/policy.md:3: drops from level 1 to level 3"

  Scenario: A wider declared jump permits what a narrower one refuses
    Given the repository declares the heading surface "rules/**/*.md" with one H1 and a maximum jump of 2
    And file "rules/policy.md" contains this Markdown:
      """
      # Policy

      ### Exceptions
      """
    When I run the "heading-hierarchy" validator
    Then the exit code is 0

  Scenario: A hash inside a fenced block is not a heading
    Given the repository declares the heading surface "rules/**/*.md" with one H1 and a maximum jump of 1
    And file "rules/policy.md" contains this Markdown:
      """
      # Policy

      ## Scope

      ```sh
      #!/usr/bin/env bash
      # A comment, and not a second level-1 heading
      ```
      """
    When I run the "heading-hierarchy" validator
    Then the exit code is 0

  Scenario: A hash with no space after it is not a heading
    Given the repository declares the heading surface "rules/**/*.md" with one H1 and a maximum jump of 1
    And file "rules/policy.md" contains this Markdown:
      """
      # Policy

      #hashtag
      """
    When I run the "heading-hierarchy" validator
    Then the exit code is 0

  Scenario: A repository permitting any number of level-1 headings says so
    Given the repository declares the heading surface "rules/**/*.md" with any number of H1s and a maximum jump of 1
    And file "rules/policy.md" contains this Markdown:
      """
      # Policy

      # Second Policy
      """
    When I run the "heading-hierarchy" validator
    Then the exit code is 0

  Scenario: The first heading establishes the level the rest are measured from
    Given the repository declares the heading surface "rules/**/*.md" with any number of H1s and a maximum jump of 1
    And file "rules/policy.md" contains this Markdown:
      """
      ### Exceptions

      #### Detail
      """
    When I run the "heading-hierarchy" validator
    Then the exit code is 0

  Scenario: Files outside every declared surface carry no heading rules
    Given the repository declares the heading surface "rules/**/*.md" with one H1 and a maximum jump of 1
    And the repository contains:
      | path               | content    |
      | guides/overview.md | ## Scope   |
    When I run the "heading-hierarchy" validator
    Then the exit code is 0
    And 0 files were inspected

  Scenario: Omitting the section refuses rather than assuming a structure
    Given the repository declares a complete configuration
    When I run the "heading-hierarchy" validator
    Then the exit code is 2
    And stderr contains "md-heading-hierarchy"

  Scenario: An unusable surface glob is a configuration fault
    Given the repository declares the heading surface "rules/[" with one H1 and a maximum jump of 1
    When I run the "heading-hierarchy" validator
    Then the exit code is 2
    And stderr contains "md-heading-hierarchy.surfaces"

  Scenario: A source that cannot be read is refused
    Given the repository declares the heading surface "rules/**/*.md" with one H1 and a maximum jump of 1
    And file "rules/policy.md" contains this Markdown:
      """
      # Policy
      """
    And the governed files are exclusively locked
    When I run the "heading-hierarchy" validator
    Then the exit code is 2
    And stderr names a file it could not read
