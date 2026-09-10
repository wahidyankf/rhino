Feature: Markdown front-matter validation

  A repository declares which files carry front matter, which keys they must
  hold, which values are closed sets, which are dates, and which keys may not
  appear at all. RHINO holds none of those lists: the section is optional, and a
  repository that declares nothing gets a refusal rather than a schema borrowed
  from whoever wrote the validator.

  Scenario: A surface's requirements are met
    Given the repository declares the front-matter surface "notes/**/*.md" requiring "title, tags"
    And file "notes/entry.md" contains this Markdown:
      """
      ---
      title: An Entry
      tags:
        - naming
      ---

      # An Entry
      """
    When I run the "frontmatter" validator
    Then the exit code is 0
    And 1 files were inspected

  Scenario: Front matter that opens and never closes is unusable
    Given the repository declares the front-matter surface "notes/**/*.md" requiring "title"
    And file "notes/entry.md" contains this Markdown:
      """
      ---
      title: An Entry

      # An Entry
      """
    When I run the "frontmatter" validator
    Then the only violation starts with "notes/entry.md:1: front matter opens and never closes"

  Scenario: A required key that is absent is a finding
    Given the repository declares the front-matter surface "notes/**/*.md" requiring "title, tags"
    And file "notes/entry.md" contains this Markdown:
      """
      ---
      title: An Entry
      ---
      """
    When I run the "frontmatter" validator
    Then the only violation starts with "notes/entry.md:1: front matter declares no `tags`"

  Scenario: A file carrying no front matter is missing every required key
    Given the repository declares the front-matter surface "notes/**/*.md" requiring "title, tags"
    And file "notes/entry.md" contains this Markdown:
      """
      # An Entry
      """
    When I run the "frontmatter" validator
    Then there are 2 violations
    And all violations are "front matter declares no"

  Scenario: A declared value set refuses anything outside it
    Given the repository declares the front-matter surface "notes/**/*.md" with the "category" values "tutorial, how-to"
    And file "notes/entry.md" contains this Markdown:
      """
      ---
      category: essay
      ---
      """
    When I run the "frontmatter" validator
    Then the only violation starts with "notes/entry.md:2: `category` holds a value outside its declared set"

  Scenario: A declared value set accepts a value inside it
    Given the repository declares the front-matter surface "notes/**/*.md" with the "category" values "tutorial, how-to"
    And file "notes/entry.md" contains this Markdown:
      """
      ---
      category: "how-to"
      ---
      """
    When I run the "frontmatter" validator
    Then the exit code is 0

  Scenario Outline: A declared date key holds an ISO calendar date and nothing else
    Given the repository declares the front-matter surface "notes/**/*.md" with the ISO date key "last_updated"
    And file "notes/entry.md" contains this Markdown:
      """
      ---
      last_updated: <written>
      ---
      """
    When I run the "frontmatter" validator
    Then the exit code is <code>

    Examples:
      | written    | code |
      | 2026-01-31 | 0    |
      | 2026-02-28 | 0    |
      | 2024-02-29 | 0    |
      | 2026-02-30 | 1    |
      | 2026-04-31 | 1    |
      | 2026-+2-01 | 1    |
      | 28-02-2026 | 1    |
      | yesterday  | 1    |

  Scenario: A forbidden key is a finding wherever it appears
    Given the repository declares the front-matter surface "notes/**/*.md" forbidding "updated"
    And file "notes/entry.md" contains this Markdown:
      """
      ---
      title: An Entry
      updated: 2026-02-28
      ---
      """
    When I run the "frontmatter" validator
    Then the only violation starts with "notes/entry.md:3: `updated` is a key this surface forbids"

  Scenario: A surface requiring nothing accepts a file with no front matter
    Given the repository declares the front-matter surface "notes/**/*.md" requiring nothing
    And file "notes/entry.md" contains this Markdown:
      """
      # An Entry
      """
    When I run the "frontmatter" validator
    Then the exit code is 0
    And 1 files were inspected

  Scenario: A later surface overrides an earlier one for the same file
    Given the repository declares the front-matter surface "**/*.md" requiring "title"
    And the repository declares the front-matter surface "notes/**/*.md" requiring nothing
    And the repository contains:
      | path                | content    |
      | notes/entry.md      | # An Entry |
      | guides/overview.md  | # Overview |
    When I run the "frontmatter" validator
    Then the only violation starts with "guides/overview.md"
    And 2 files were inspected

  Scenario: Files outside every declared surface carry no schema
    Given the repository declares the front-matter surface "notes/**/*.md" requiring "title"
    And the repository contains:
      | path               | content    |
      | guides/overview.md | # Overview |
    When I run the "frontmatter" validator
    Then the exit code is 0
    And 0 files were inspected

  Scenario: Omitting the section refuses rather than assuming a schema
    Given the repository declares a complete configuration
    When I run the "frontmatter" validator
    Then the exit code is 2
    And stderr contains "md-frontmatter"

  Scenario: An unusable surface glob is a configuration fault
    Given the repository declares the front-matter surface "notes/[" requiring "title"
    When I run the "frontmatter" validator
    Then the exit code is 2
    And stderr contains "md-frontmatter.surfaces"

  Scenario: A source that cannot be read is refused
    Given the repository declares the front-matter surface "notes/**/*.md" requiring "title"
    And file "notes/entry.md" contains this Markdown:
      """
      # An Entry
      """
    And the governed files are exclusively locked
    When I run the "frontmatter" validator
    Then the exit code is 2
    And stderr names a file it could not read
