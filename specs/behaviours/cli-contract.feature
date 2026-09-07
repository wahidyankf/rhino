Feature: Command contract

  Exit 0 means checked and clean, 1 means findings, 2 means the invocation,
  root, or configuration was unusable. A caller reads the code without knowing
  which subcommand produced it.

  Scenario: Every canonical nested command path succeeds
    Given a repository declaring every section with nothing to find
    When I run the "word-budget" validator
    Then the exit code is 0
    And 2 files were inspected
    When I run the "directory-map" validator
    Then the exit code is 0
    And 1 directories were inspected
    When I run the "mermaid" validator
    Then the exit code is 0
    And 1 Mermaid diagrams were inspected
    When I run the "internal-link" validator
    Then the exit code is 0
    And 2 links were inspected
    When I run the "harness-parity" validator
    Then the exit code is 0
    And 3 capability declarations were reconciled

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
      | path                    | content                                                      |
      | guides/README.md        | # Guides\n\n## Directory Map\n\n- [Nested](nested/README.md) |
      | guides/nested/README.md | # Nested\n\n## Directory Map\n\nNo other entries.            |
    When I run the directory-map validator for "guides"
    Then the exit code is 0
    And 2 directories were inspected

  Scenario: A selected directory without a README fails
    Given the repository declares the mapped trees "rules" and "guides"
    And the repository contains:
      | path                  | content                                        |
      | guides/README.md      | # Guides\n\n## Directory Map\n\n- [Nested](nested) |
      | guides/nested/page.md | # Page                                         |
    When I run the directory-map validator for "guides"
    Then the exit code is 1
    And the only violation is a missing README at "guides/nested"

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
      | path                        | content                                                   |
      | records/README.md           | # Records\n\n## Directory Map\n\n- [Open](open/README.md) |
      | records/open/README.md      | # Open\n\n## Directory Map\n\n- [Item](item/README.md)    |
      | records/open/item/README.md | # Item                                                    |
      | records/open/item/detail.md | # Detail                                                  |
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
    And the scanned Markdown paths are:
      * rules/README.md
      * rules/diagram.md

  Scenario: Root is accepted before and after nested commands
    Given a repository declaring every section with nothing to find
    When I invoke the CLI with "--root|rules|governance|word-budget|validate"
    Then the exit code is 2
    When I invoke the CLI with "governance|word-budget|validate|--root|rules"
    Then the exit code is 2

  Scenario: A selected root is the repository that gets inspected
    Given a repository declaring every section with nothing to find
    When I invoke the CLI with "governance|word-budget|validate|--root|rules"
    Then the exit code is 2
    And stderr lines start with "[word-budget] "

  Scenario: A selected root brings its own scan exclusions
    Given a repository declaring every section with nothing to find
    And a nested repository under "vendored" scans a directory the outer repository excludes
    When I invoke the CLI with "md|mermaid|validate|--root|vendored"
    Then the exit code is 1
    And 1 Mermaid diagrams were inspected

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
    And stderr names a file it could not read

    Examples:
      | validator      |
      | word-budget    |
      | directory-map  |
      | mermaid        |
      | internal-link  |
      | harness-parity |

  Scenario Outline: Help requests succeed
    Given a repository declaring every section with nothing to find
    When I invoke the CLI with "<arguments>"
    Then the exit code is 0
    And stdout names the command "<lists>"
    And stdout names every exit code

    Examples:
      | arguments                                   | lists                                   |
      | --help                                      | rhino version                           |
      | governance\|--help                          | rhino governance directory-map validate |
      | governance\|word-budget\|--help             | rhino governance word-budget validate   |
      | governance\|word-budget\|validate\|--help   | rhino governance word-budget validate   |
      | governance\|directory-map\|validate\|--help | rhino governance directory-map validate |
      | harness\|--help                             | rhino harness parity validate           |
      | harness\|parity\|--help                     | rhino harness parity validate           |
      | harness\|parity\|validate\|--help           | rhino harness parity validate           |
      | md\|--help                                  | rhino md mermaid validate               |
      | md\|internal-link\|--help                   | rhino md internal-link validate         |
      | md\|internal-link\|validate\|--help         | rhino md internal-link validate         |
      | md\|mermaid\|--help                         | rhino md mermaid validate               |
      | md\|mermaid\|validate\|--help               | rhino md mermaid validate               |
      | md\|word-count\|inspect\|--help             | rhino md word-count inspect             |
      | repo-config\|validate\|--help               | rhino repo-config validate              |
      | version\|--help                             | rhino version                           |

  Scenario: Help is scoped to the command path that asked for it
    Given a repository declaring every section with nothing to find
    When I invoke the CLI with "md|--help"
    Then the exit code is 0
    And stdout names the command "rhino md mermaid validate"
    And stdout does not name the command "rhino harness parity validate"
    When I invoke the CLI with "--help"
    Then the exit code is 0
    And stdout names the command "rhino md mermaid validate"
    And stdout names the command "rhino harness parity validate"

  Scenario: Version reports the embedded release identity
    Given a repository declaring every section with nothing to find
    When I invoke the CLI with "version|--json"
    Then the exit code is 0
    And stdout JSON property "schemaVersion" is 1
    And stdout JSON has a "version" and a 40-character hexadecimal "commit"

  Scenario Outline: Invalid invocations return usage failure
    Given a repository declaring every section with nothing to find
    When I invoke the CLI with "<arguments>"
    Then the exit code is 2

    Examples:
      | arguments                                                 |
      | {root}                                                    |
      | governance                                                |
      | governance\|word-budget                                   |
      | harness                                                   |
      | harness\|parity                                           |
      | md\|mermaid                                               |
      | md\|internal-link                                         |
      | unknown                                                   |
      | governance\|word-budget\|validate\|extra                  |
      | governance\|word-budget\|validate\|--harness\|alpha       |
      | md\|mermaid\|validate\|--directory\|guides                |
      | md\|mermaid\|validate\|--file\|/rules/README.md           |
      | unknown\|--help                                           |
      | version\|--json\|--output\|text                           |
      | governance\|word-budget\|validate\|--root                  |
      | governance\|word-budget\|validate\|--nope                  |
      | governance\|word-budget\|validate\|--root\|{missing-root} |

  Scenario: A governed file above its declared limit returns validation failure
    Given a repository declaring every section with nothing to find
    And file "root-instructions.md" exceeds its declared word budget
    When I run the "word-budget" validator
    Then the exit code is 1
    And the only violation is a 6-word limit for "root-instructions.md"

  Scenario: An incomplete directory map returns validation failure
    Given a repository declaring every section with nothing to find
    And the repository contains:
      | path            | content |
      | rules/README.md | # Rules |
    When I run the "directory-map" validator
    Then the exit code is 1
    And the only violation is a missing directory map at "rules/README.md"

  Scenario Outline: Output format is recursive for every validator
    Given a repository declaring every section with nothing to find
    When I invoke the CLI with "<arguments>|--output|json"
    Then the exit code is 0
    And stdout lines start with "{"

    Examples:
      | arguments                           |
      | governance\|word-budget\|validate   |
      | governance\|directory-map\|validate |
      | harness\|parity\|validate           |
      | md\|internal-link\|validate         |
      | md\|mermaid\|validate               |
      | repo-config\|validate               |

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
      | arguments                                   |
      | governance\|word-budget\|validate           |
      | governance\|directory-map\|validate         |
      | md\|internal-link\|validate                 |
      | md\|word-count\|inspect\|--file\|rules/README.md |

  Scenario Outline: Global presentation flags are accepted by every reporting leaf
    Given a repository declaring every section with nothing to find
    When I invoke the CLI with "<command>|<flag>"
    Then the exit code is 0
    And stdout lines start with "[<category>] "

    Examples:
      | command                             | flag       | category       |
      | governance\|word-budget\|validate   | --quiet    | word-budget    |
      | governance\|directory-map\|validate | --verbose  | directory-map  |
      | harness\|parity\|validate           | --no-color | harness-parity |
      | md\|internal-link\|validate         | --quiet    | internal-link  |
      | md\|mermaid\|validate               | --verbose  | mermaid        |
      | repo-config\|validate               | --no-color | repo-config    |

  Scenario: Repeated file selection is accepted by the Mermaid leaf
    Given the repository declares the accessible palette
    And the repository contains Mermaid sample "accessible colored class" at "guides/one.md"
    And the repository contains Mermaid sample "accessible colored class" at "guides/two.md"
    When I invoke the CLI with "md|mermaid|validate|--file|guides/one.md|--file|guides/two.md"
    Then the exit code is 0
    And 2 Mermaid diagrams were inspected

  Scenario: The Mermaid leaf reads a diagram from standard input
    Given the repository declares the accessible palette
    And an unsafe "flowchart" Mermaid diagram is supplied on standard input
    When I invoke the CLI with "md|mermaid|validate|--file|-"
    Then the exit code is 1
    And 1 Mermaid diagrams were inspected

  Scenario: Version reports the release identity as text
    Given a repository declaring every section with nothing to find
    When I invoke the CLI with "version|--json"
    Then the exit code is 0
    When I invoke the CLI with "version"
    Then the exit code is 0
    And stdout is one non-empty line
    And stdout is the version the JSON form reported

  Scenario: An empty command line asks for a command
    Given a repository declaring every section with nothing to find
    When I invoke the CLI with no arguments
    Then the exit code is 2
    And stderr contains "no command given"
