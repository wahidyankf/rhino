Feature: Command contract

  Exit 0 means checked and clean, 1 means findings, 2 means the invocation,
  root, or configuration was unusable. A caller reads the code without knowing
  which subcommand produced it.

  Scenario: Every canonical nested command path succeeds
    Given a repository declaring every section with nothing to find
    When I run the "word-budget" validator
    Then the exit code is 0
    When I run the "directory-map" validator
    Then the exit code is 0
    When I run the "mermaid" validator
    Then the exit code is 0
    When I run the "internal-link" validator
    Then the exit code is 0
    When I run the "harness-parity" validator
    Then the exit code is 0

  Scenario: Word-budget validation is isolated
    Given a repository declaring every section with nothing to find
    And file "root-instructions.md" exceeds its declared word budget
    When I run the "word-budget" validator
    Then the exit code is 1
    When I run the "directory-map" validator
    Then the exit code is 0
    When I run the "mermaid" validator
    Then the exit code is 0

  Scenario: Directory-map validation is isolated
    Given a repository declaring every section with nothing to find
    And the repository contains:
      | path            | content |
      | rules/README.md | # Rules |
    When I run the "word-budget" validator
    Then the exit code is 0
    When I run the "directory-map" validator
    Then the exit code is 1
    When I run the "mermaid" validator
    Then the exit code is 0

  Scenario: A complete selected tree passes directory-map validation
    Given the repository declares the mapped trees "rules" and "guides"
    And the repository contains:
      | path                    | content                                                       |
      | guides/README.md        | # Guides\n\n## Directory Map\n\n- [Nested](nested/README.md)   |
      | guides/nested/README.md | # Nested\n\n## Directory Map\n\nNo other entries.              |
    When I run the directory-map validator for "guides"
    Then the exit code is 0

  Scenario: A selected directory without a README fails
    Given the repository declares the mapped trees "rules" and "guides"
    And the repository contains:
      | path                  | content                                                     |
      | guides/README.md      | # Guides\n\n## Directory Map\n\n- [Nested](nested/README.md) |
      | guides/nested/page.md | # Page                                                      |
    When I run the directory-map validator for "guides"
    Then the exit code is 1

  Scenario: An omitted selected sibling fails
    Given the repository declares the mapped trees "rules" and "guides"
    And file "guides/README.md" has title "Guides" and an empty directory map
    And the repository contains:
      | path           | content |
      | guides/page.md | # Page  |
    When I run the directory-map validator for "guides"
    Then the exit code is 1

  Scenario: A selected tree requires recursive README directory maps
    Given the repository declares the mapped tree "records"
    And the repository contains:
      | path                            | content                                                        |
      | records/README.md               | # Records\n\n## Directory Map\n\n- [Open](open/README.md)       |
      | records/open/README.md          | # Open\n\n## Directory Map\n\n- [Item](item/README.md)          |
      | records/open/item/README.md     | # Item                                                         |
      | records/open/item/detail.md     | # Detail                                                       |
    When I run the directory-map validator for "records"
    Then the exit code is 1

  Scenario: Mermaid accessibility validation is isolated
    Given a repository declaring every section with nothing to find
    And an unsafe "flowchart LR" Mermaid diagram exists at "guides/diagram.md" using backtick fences
    When I run the "word-budget" validator
    Then the exit code is 0
    When I run the "directory-map" validator
    Then the exit code is 0
    When I run the "mermaid" validator
    Then the exit code is 1

  Scenario: Root defaults to the current directory
    Given a repository declaring every section with nothing to find
    When I invoke the CLI from the repository directory with "governance|word-budget|validate"
    Then the exit code is 0

  Scenario: Root is accepted before and after nested commands
    Given a repository declaring every section with nothing to find
    When I invoke the CLI with "--root|{root}|governance|word-budget|validate"
    Then the exit code is 0
    When I invoke the CLI with "governance|word-budget|validate|--root|{root}"
    Then the exit code is 0

  Scenario: Leaf output has an atomic command-category prefix
    Given a repository declaring every section with nothing to find
    When I run the "word-budget" validator
    Then the exit code is 0
    And stdout lines start with "[word-budget] "
    When I run the "directory-map" validator
    Then the exit code is 0
    And stdout lines start with "[directory-map] "
    When I run the "mermaid" validator
    Then the exit code is 0
    And stdout lines start with "[mermaid] "
    Given file "root-instructions.md" exceeds its declared word budget
    When I run the "word-budget" validator
    Then the exit code is 1
    And stderr lines start with "[word-budget] "

  Scenario Outline: Inspection errors use command-specific diagnostics
    Given a repository declaring every section with nothing to find
    And the governed files are exclusively locked
    When I run the "<validator>" validator
    Then the exit code is 2
    And stdout is empty
    And stderr lines start with "[<validator>] "

    Examples:
      | validator     |
      | word-budget   |
      | directory-map |
      | mermaid       |
      | internal-link |
      | harness-parity |

  Scenario Outline: Help requests succeed
    Given a repository declaring every section with nothing to find
    When I invoke the CLI with "<arguments>"
    Then the exit code is 0

    Examples:
      | arguments                                    |
      | --help                                       |
      | governance\|--help                           |
      | governance\|word-budget\|--help              |
      | governance\|word-budget\|validate\|--help    |
      | governance\|directory-map\|validate\|--help  |
      | harness\|--help                              |
      | harness\|parity\|--help                      |
      | harness\|parity\|validate\|--help            |
      | md\|--help                                   |
      | md\|internal-link\|--help                    |
      | md\|internal-link\|validate\|--help          |
      | md\|mermaid\|--help                          |
      | md\|mermaid\|validate\|--help                |
      | md\|word-count\|inspect\|--help              |
      | repo-config\|validate\|--help                |
      | version\|--help                              |

  Scenario: Version reports the embedded release identity
    Given a repository declaring every section with nothing to find
    When I invoke the CLI with "version|--json"
    Then the exit code is 0
    And stdout JSON property "schemaVersion" is 1
    And stdout JSON has a "version" and a 40-character "commit"

  Scenario Outline: Invalid invocations return usage failure
    Given a repository declaring every section with nothing to find
    When I invoke the CLI with "<arguments>"
    Then the exit code is 2

    Examples:
      | arguments                                                |
      | {root}                                                   |
      | governance                                               |
      | governance\|word-budget                                  |
      | harness                                                  |
      | harness\|parity                                          |
      | md\|mermaid                                              |
      | md\|internal-link                                        |
      | unknown                                                  |
      | governance\|word-budget\|validate\|extra                 |
      | governance\|word-budget\|validate\|--root\|{missing-root} |

  Scenario: A governed file above its declared limit returns validation failure
    Given a repository declaring every section with nothing to find
    And file "root-instructions.md" exceeds its declared word budget
    When I run the "word-budget" validator
    Then the exit code is 1

  Scenario: An incomplete directory map returns validation failure
    Given a repository declaring every section with nothing to find
    And the repository contains:
      | path            | content |
      | rules/README.md | # Rules |
    When I run the "directory-map" validator
    Then the exit code is 1

  Scenario Outline: Output format is recursive for every validator
    Given a repository declaring every section with nothing to find
    When I invoke the CLI with "<arguments>|--output|json"
    Then the exit code is 0
    And stdout lines start with "{"

    Examples:
      | arguments                              |
      | governance\|word-budget\|validate      |
      | governance\|directory-map\|validate    |
      | harness\|parity\|validate              |
      | md\|internal-link\|validate            |
      | md\|mermaid\|validate                  |
      | repo-config\|validate                  |

  Scenario: A file word count is observable as JSON
    Given Markdown text containing a heading marker, Hello, can't-stop, naïve, and 42
    When I invoke the CLI with "md|word-count|inspect|--file|subject.md|--output|json"
    Then the exit code is 0
    And stdout JSON property "wordCount" is 4

  Scenario: Word-count inspection never reports findings
    Given a repository declaring every section with nothing to find
    And file "root-instructions.md" exceeds its declared word budget
    When I invoke the CLI with "md|word-count|inspect|--file|root-instructions.md"
    Then the exit code is 0

  Scenario: JSON validation failures retain the validation exit code
    Given a repository declaring every section with nothing to find
    And file "root-instructions.md" exceeds its declared word budget
    When I invoke the CLI with "governance|word-budget|validate|--output|json|--root|{root}"
    Then the exit code is 1
    And the first stdout JSON violation kind is "word-limit-exceeded"

  Scenario Outline: Unsupported output formats return usage failure
    Given a repository declaring every section with nothing to find
    When I invoke the CLI with "<arguments>|--output|xml"
    Then the exit code is 2

    Examples:
      | arguments                                    |
      | governance\|word-budget\|validate            |
      | governance\|directory-map\|validate          |
      | md\|internal-link\|validate                  |
      | md\|word-count\|inspect\|--file\|missing.md  |

  Scenario Outline: Global presentation flags are accepted by every leaf
    Given a repository declaring every section with nothing to find
    When I invoke the CLI with "governance|word-budget|validate|<flag>"
    Then the exit code is 0

    Examples:
      | flag       |
      | --quiet    |
      | --verbose  |
      | --no-color |

  Scenario: Repeated file selection and stdin are accepted by the Mermaid leaf
    Given the repository declares the accessible palette
    And the repository contains Mermaid sample "accessible colored class" at "guides/one.md"
    And the repository contains Mermaid sample "accessible colored class" at "guides/two.md"
    When I invoke the CLI with "md|mermaid|validate|--file|guides/one.md|--file|guides/two.md"
    Then the exit code is 0
    And 2 Mermaid diagrams were inspected
