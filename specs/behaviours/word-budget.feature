Feature: Word-budget validation

  A repository declares its own word surfaces and limits. RHINO holds no opinion
  about which files are governed or how long they may be; it enforces what the
  configuration says and reports what it counted.

  Scenario: Markdown punctuation does not create extra words
    Given Markdown text containing a heading marker, Hello, can't-stop, naïve, and 42
    When I count the words in "subject.md"
    Then the word count is 4

  Scenario: Only declared surfaces are scanned
    Given the repository declares word-budget surfaces:
      | glob                 | fail |
      | root-instructions.md | 40   |
      | rules/**/*.md        | 40   |
    And the repository contains:
      | path                     | content      |
      | root-instructions.md     | agents rules |
      | rules/nested/RULES.MD    | nested rules |
      | rules/notes.txt          | not Markdown |
      | guides/long-form.md      | outside      |
      | README.md                | outside root |
    When I scan the declared surfaces
    Then the scanned Markdown paths are:
      * root-instructions.md
      * rules/nested/RULES.MD

  Scenario: The declared limit is inclusive
    Given the repository declares a word-budget surface "**/*.md" failing above 40
    And file "root-instructions.md" contains 40 words
    And file "rules/too-long.md" contains 41 words
    When I find word-limit violations
    Then the only violation is a 41-word limit for "rules/too-long.md"

  Scenario: An empty repository scans nothing
    Given the repository declares a word-budget surface "**/*.md" failing above 40
    And an empty repository
    When I scan the declared surfaces
    Then no Markdown files are scanned

  Scenario: Files outside every declared surface have no limit
    Given the repository declares a word-budget surface "rules/**/*.md" failing above 40
    And file "designs/architecture.md" contains 41 words
    When I inspect the word budget
    Then no Markdown files are scanned
    And there are no violations

  Scenario: A later surface overrides an earlier one for the same file
    Given the repository declares word-budget surfaces:
      | glob         | fail |
      | rules/**/*.md | 40   |
      | **/README.md  | 80   |
    And file "rules/README.md" contains 60 words
    And file "rules/other.md" contains 60 words
    When I find word-limit violations
    Then the only violation is a 60-word limit for "rules/other.md"

  Scenario: Word-budget inspection ignores other validators' concerns
    Given the repository declares a word-budget surface "**/*.md" failing above 40
    And file "root-instructions.md" contains 41 words
    And the repository contains:
      | path            | content                                              |
      | rules/README.md | # Rules                                              |
      | guides/diagram.md | ```mermaid\nflowchart LR\nclassDef unsafe fill:red\n``` |
    When I inspect the word budget
    Then the scanned Markdown paths are:
      * guides/diagram.md
      * root-instructions.md
      * rules/README.md
    And the only violation is a 41-word limit for "root-instructions.md"
