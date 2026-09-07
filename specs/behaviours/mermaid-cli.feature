Feature: Mermaid accessibility command behaviour

  A repository declares the colour sets its diagrams may use. Which diagram
  syntaxes RHINO can parse is a property of the tool; which colours are
  acceptable is a property of the repository.

  Background:
    Given the repository declares the accessible palette

  Scenario: File scope excludes unrelated diagrams
    Given the repository contains Mermaid sample "accessible colored class" at "guides/selected.md"
    And an unsafe "flowchart LR" Mermaid diagram exists at "guides/unselected.md" using backtick fences
    When I invoke the CLI with "md|mermaid|validate|--file|guides/selected.md"
    Then the exit code is 0

  Scenario: A repository with no diagrams reports a clean explicit zero
    Given an empty repository
    When I run the "mermaid" validator
    Then the exit code is 0
    And 0 Mermaid diagrams were inspected
    And stdout reports that zero diagrams were checked

  Scenario Outline: Parseable diagram types enforce class colors
    Given an unsafe "<header>" Mermaid diagram exists at "guides/diagram.md" using backtick fences
    When I run the "mermaid" validator
    Then the exit code is 1

    Examples:
      | header             |
      | flowchart LR       |
      | graph TD           |
      | classDiagram       |
      | stateDiagram       |
      | stateDiagram-v2    |
      | erDiagram          |
      | requirementDiagram |
      | block              |

  Scenario Outline: Unparseable diagram types are skipped
    Given an unsafe "<header>" Mermaid diagram exists at "guides/diagram.md" using backtick fences
    When I run the "mermaid" validator
    Then the exit code is 0

    Examples:
      | header            |
      | sequenceDiagram   |
      | mindmap           |
      | timeline          |
      | kanban            |
      | architecture-beta |
      | treeView          |
      | gantt             |
      | pie               |
      | quadrantChart     |
      | treemap-beta      |
      | swimlane-beta     |
      | futureDiagram     |

  Scenario: Tilde-fenced Mermaid diagrams are extracted
    Given an unsafe "flowchart LR" Mermaid diagram exists at "guides/diagram.md" using tilde fences
    When I run the "mermaid" validator
    Then the exit code is 1

  Scenario: Diagram type is found after YAML front matter
    Given the repository contains Mermaid sample "YAML front matter before an unsafe flowchart" at "guides/diagram.md"
    When I run the "mermaid" validator
    Then the exit code is 1

  Scenario: A Mermaid block without a diagram type is skipped
    Given the repository contains Mermaid sample "no diagram declaration" at "guides/diagram.md"
    When I run the "mermaid" validator
    Then the exit code is 0

  Scenario: Declared excluded directories are not scanned
    Given the repository declares excluded scan directories:
      * .git
      * build-output
      * dependencies
    And each declared excluded directory contains an unsafe Mermaid diagram
    When I run the "mermaid" validator
    Then the exit code is 0

  Scenario: An accessible colored class passes
    Given the repository contains Mermaid sample "accessible colored class" at "guides/diagram.md"
    When I run the "mermaid" validator
    Then the exit code is 0

  Scenario Outline: Colors outside classDef are rejected
    Given the repository contains Mermaid sample "<sample>" at "guides/diagram.md"
    When I run the "mermaid" validator
    Then the exit code is 1

    Examples:
      | sample                         |
      | style declaration color        |
      | linkStyle declaration color    |
      | initialization directive color |

  Scenario Outline: Unsupported color formats are rejected
    Given the repository contains Mermaid sample "<sample>" at "guides/diagram.md"
    When I run the "mermaid" validator
    Then the exit code is 1

    Examples:
      | sample                |
      | named color           |
      | three-digit hex color |
      | eight-digit hex color |
      | RGB function color    |
      | HSL function color    |

  Scenario Outline: Palette comments are neither required nor inspected
    Given the repository contains Mermaid sample "<sample>" at "guides/diagram.md"
    When I run the "mermaid" validator
    Then the exit code is 0

    Examples:
      | sample                     |
      | duplicate palette comments |
      | inaccurate palette comment |

  Scenario Outline: Node color roles and normal-text contrast are enforced
    Given the repository contains Mermaid sample "<sample>" at "guides/diagram.md"
    When I run the "mermaid" validator
    Then the exit code is 1

    Examples:
      | sample                     |
      | identical fill and stroke  |
      | missing stroke             |
      | undeclared node stroke     |
      | missing text color         |
      | undeclared text color      |
      | insufficient text contrast |

  Scenario: A fill color the repository declared passes
    Given the repository declares a palette whose only fill color is "#123456"
    And the repository contains a Mermaid class filled "#123456" with a declared stroke and text color
    When I run the "mermaid" validator
    Then the exit code is 0

  Scenario: An accessible stroke-only class passes
    Given the repository contains Mermaid sample "accessible stroke-only class" at "guides/diagram.md"
    When I run the "mermaid" validator
    Then the exit code is 0

  Scenario Outline: Text-only and inaccessible stroke-only classes fail
    Given the repository contains Mermaid sample "<sample>" at "guides/diagram.md"
    When I run the "mermaid" validator
    Then the exit code is 1

    Examples:
      | sample                         |
      | text-only class                |
      | inaccessible stroke-only class |
