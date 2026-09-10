Feature: Mermaid legibility inspection

  Label length limits are declared by the repository and counted in Unicode
  grapheme clusters after markup removal and entity decoding.

  A repository may also declare one authoring rule for conceptual diagrams.
  Under `rendered` a Mermaid diagram carries an accessible title and an
  accessible description; under `plain-text` no Mermaid diagram may appear at
  all. Declaring neither leaves both unchecked, which is what a repository that
  never adopted the rule already had.

  The other half of the plain-text rule stays outside RHINO. A fenced block of
  ASCII is a conceptual drawing, a quoted file tree, or command output, and the
  contract excludes the last two from the rule -- so a validator requiring prose
  beside every one of them would be reporting exactly the cases the rule
  excludes. Whether a drawing is described is a review.

  Background:
    Given the repository declares the accessible palette
    And the repository declares node labels at 32 graphemes and edge labels at 24

  Scenario: Mermaid inspection ignores other validators' concerns
    Given the repository declares a word-budget surface "**/*.md" failing above 40
    And file "root-instructions.md" contains 41 words
    And the repository contains:
      | path            | content |
      | rules/README.md | # Rules |
    And an unsafe "flowchart LR" Mermaid diagram exists at "guides/diagram.md" using backtick fences
    When I inspect Mermaid accessibility
    Then 1 Mermaid diagrams were inspected
    And the only violation is a Mermaid accessibility issue at "guides/diagram.md"

  Scenario: Mermaid diagnostics identify the Markdown source line
    Given file "guides/diagram.md" contains this Markdown:
      """
      # Diagram

      ```mermaid
      flowchart LR
          A[Node]
          classDef unsafe fill:#FF0000,stroke:#000000,color:#FFFFFF
      ```
      """
    When I inspect Mermaid accessibility
    Then the formatted violation starts with "guides/diagram.md:6:"

  Scenario Outline: Every parseable diagram type rejects an overlong visible node label
    Given the repository contains Mermaid sample "<sample>" at "guides/diagram.md"
    When I invoke the CLI with "md|mermaid|validate|--output|json"
    Then the exit code is 1
    And the first stdout JSON violation kind is "mermaid-legibility"

    Examples:
      | sample                    |
      | overlong flowchart node   |
      | overlong graph node       |
      | overlong class node       |
      | overlong state node       |
      | overlong state-v2 node    |
      | overlong ER entity        |
      | overlong requirement node |
      | overlong block node       |

  Scenario Outline: Label segments use deterministic grapheme boundaries
    Given the repository contains Mermaid sample "<sample>" at "guides/diagram.md"
    When I run the "mermaid" validator
    Then the exit code is <exit>

    Examples:
      | sample                              | exit |
      | node label at the declared limit    | 0    |
      | node label one past the limit       | 1    |
      | edge label at the declared limit    | 0    |
      | edge label one past the limit       | 1    |
      | text edge one past the limit        | 1    |
      | split node labels at the boundary   | 0    |
      | split node labels with br           | 0    |
      | split node labels with newline      | 0    |
      | combining grapheme node at boundary | 0    |
      | encoded node at boundary            | 0    |

  Scenario: A different declared limit moves the boundary
    Given the repository declares node labels at 8 graphemes and edge labels at 24
    And the repository contains a Mermaid node label of 9 graphemes
    When I run the "mermaid" validator
    Then the exit code is 1

  Scenario: State transition semicolons are rejected
    Given the repository contains Mermaid sample "state transition semicolon" at "guides/diagram.md"
    When I invoke the CLI with "md|mermaid|validate|--output|json"
    Then the exit code is 1
    And the first stdout JSON violation kind is "mermaid-legibility"

  Scenario: Legibility JSON reports deterministic measurement fields
    Given the repository contains Mermaid sample "node label one past the limit" at "guides/diagram.md"
    When I invoke the CLI with "md|mermaid|validate|--output|json"
    Then the first stdout JSON legibility fields are "node", 33, and 32

  Scenario: Non-label Mermaid declarations are excluded
    Given the repository contains Mermaid sample "legibility exclusions" at "guides/diagram.md"
    When I run the "mermaid" validator
    Then the exit code is 0

  Scenario Outline: A label is measured as it is seen, not as it is written
    Given the repository declares node labels at 8 graphemes and edge labels at 24
    And the repository contains a Mermaid node label written as "<written>"
    When I run the "mermaid" validator
    Then the exit code is <exit>

    Examples:
      | written                                          | exit |
      | aaaaaaaa                                         | 0    |
      | aaaaaaaaa                                        | 1    |
      | <b>aaaaaaaa</b>                                  | 0    |
      | &amp;&amp;&amp;&amp;&amp;&amp;&amp;&amp;         | 0    |
      | &#65;&#65;&#65;&#65;&#65;&#65;&#65;&#65;         | 0    |
      | &#x41;&#x41;&#x41;&#x41;&#x41;&#x41;&#x41;&#x41; | 0    |
      | &lt;&gt;&quot;&apos;&nbsp;aaa                    | 0    |
      | &frob;aaa                                        | 1    |
      | a&b aaaaa                                        | 1    |

  Scenario: A rendered repository requires an accessible title and description
    Given the configuration sets "md-mermaid.authoring-rule" to "rendered"
    And file "guides/diagram.md" contains this Markdown:
      """
      # Guide

      ```mermaid
      flowchart LR
        accTitle: Delivery path
        accDescr: A change moves from a worktree to a pull request to main.
        A[Worktree] --> B[Pull request]
      ```
      """
    When I inspect Mermaid accessibility
    Then the exit code is 0
    And there are no violations

  Scenario: A rendered diagram with no accessible title is a finding
    Given the configuration sets "md-mermaid.authoring-rule" to "rendered"
    And file "guides/diagram.md" contains this Markdown:
      """
      # Guide

      ```mermaid
      flowchart LR
        accDescr: A change moves from a worktree to a pull request to main.
        A[Worktree] --> B[Pull request]
      ```
      """
    When I inspect Mermaid accessibility
    Then the only violation starts with "guides/diagram.md: declares no accessible title"

  Scenario: A rendered diagram with no accessible description is a finding
    Given the configuration sets "md-mermaid.authoring-rule" to "rendered"
    And file "guides/diagram.md" contains this Markdown:
      """
      # Guide

      ```mermaid
      flowchart LR
        accTitle: Delivery path
        A[Worktree] --> B[Pull request]
      ```
      """
    When I inspect Mermaid accessibility
    Then the only violation starts with "guides/diagram.md: declares no accessible description"

  Scenario: The braced description form is a description
    Given the configuration sets "md-mermaid.authoring-rule" to "rendered"
    And file "guides/diagram.md" contains this Markdown:
      """
      # Guide

      ```mermaid
      flowchart LR
        accTitle: Delivery path
        accDescr {
          A change moves from a worktree
          to a pull request to main.
        }
        A[Worktree] --> B[Pull request]
      ```
      """
    When I inspect Mermaid accessibility
    Then the exit code is 0
    And there are no violations

  Scenario: A plain-text repository refuses a Mermaid diagram
    Given the configuration sets "md-mermaid.authoring-rule" to "plain-text"
    And file "guides/diagram.md" contains this Markdown:
      """
      # Guide

      ```mermaid
      flowchart LR
        accTitle: Delivery path
        accDescr: A change moves from a worktree to a pull request to main.
        A[Worktree] --> B[Pull request]
      ```
      """
    When I inspect Mermaid accessibility
    Then the only violation starts with "guides/diagram.md: carries a Mermaid diagram"

  Scenario: A plain-text repository accepts an ASCII drawing with prose beside it
    Given the configuration sets "md-mermaid.authoring-rule" to "plain-text"
    And file "guides/diagram.md" contains this Markdown:
      """
      # Guide

      A change moves from a worktree to a pull request to main.

      ```text
      worktree --> pull request --> main
      ```
      """
    When I inspect Mermaid accessibility
    Then the exit code is 0
    And there are no violations

  Scenario: An ASCII drawing with no prose beside it is a review, not a finding
    Given the configuration sets "md-mermaid.authoring-rule" to "plain-text"
    And file "guides/diagram.md" contains this Markdown:
      """
      # Guide

      ```text
      worktree --> pull request --> main
      ```
      """
    When I inspect Mermaid accessibility
    Then the exit code is 0
    And there are no violations

  Scenario: A repository declaring no authoring rule is held to neither
    Given file "guides/diagram.md" contains this Markdown:
      """
      # Guide

      ```mermaid
      flowchart LR
        A[Worktree] --> B[Pull request]
      ```
      """
    When I inspect Mermaid accessibility
    Then the exit code is 0
    And there are no violations
