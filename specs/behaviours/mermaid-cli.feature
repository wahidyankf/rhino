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

  Scenario: A selected file that cannot be read is refused
    Given the repository contains Mermaid sample "accessible colored class" at "guides/diagram.md"
    And the governed files are exclusively locked
    When I invoke the CLI with "md|mermaid|validate|--file|guides/diagram.md"
    Then the exit code is 2
    And stderr names a file it could not read

  Scenario: A fenced Mermaid block with no diagram in it is not counted
    Given file "guides/diagram.md" contains this Markdown:
      """
      # Notes

      ```mermaid
      ```
      """
    When I run the "mermaid" validator
    Then the exit code is 0
    And 0 Mermaid diagrams were inspected

  Scenario: A class that sets no color role declares no color
    Given file "guides/diagram.md" contains this Markdown:
      """
      # Diagram

      ```mermaid
      flowchart LR
          Alpha[Alpha]
          classDef thick stroke-width:2px
          class Alpha thick
      ```
      """
    When I run the "mermaid" validator
    Then the exit code is 0
    And there are no violations
    And 1 Mermaid diagrams were inspected

  Scenario: A class with a stroke and a text color but no fill is still checked
    Given file "guides/diagram.md" contains this Markdown:
      """
      # Diagram

      ```mermaid
      flowchart LR
          Alpha[Alpha]
          classDef edged stroke:#DE8F05,color:#FFFFFF
          class Alpha edged
      ```
      """
    When I run the "mermaid" validator
    Then the exit code is 1
    And the only violation starts with "guides/diagram.md:6: stroke `#DE8F05` is not a declared edge color"

  Scenario Outline: A color set outside a class declaration is refused whatever its notation
    Given file "guides/diagram.md" contains this Markdown:
      """
      # Diagram

      ```mermaid
      flowchart LR
          Alpha[Alpha]
          style Alpha <declaration>
      ```
      """
    When I run the "mermaid" validator
    Then the exit code is 1
    And the only violation starts with "guides/diagram.md:6: color is declared outside a classDef"

    Examples:
      | declaration                        |
      | fill:#0173B2                       |
      | fill:blue                          |
      | fill-opacity:rgb(1, 115, 178)      |
      | fill-opacity:rgba(1, 115, 178, 1)  |
      | fill-opacity:hsl(202, 99%, 35%)    |

  Scenario: An unpaired edge-label delimiter ends the edge scan
    Given file "guides/diagram.md" contains this Markdown:
      """
      # Diagram

      ```mermaid
      flowchart LR
          Alpha -->|aaaaaaaaaaaaaaaaaaaaaaaaa
      ```
      """
    When I run the "mermaid" validator
    Then the exit code is 0
    And 1 Mermaid diagrams were inspected

  Scenario: A fenced block in another language is not a diagram
    Given file "guides/diagram.md" contains this Markdown:
      """
      # Notes

      ```text
      flowchart LR
          Alpha[Alpha]
      ```
      """
    When I run the "mermaid" validator
    Then the exit code is 0
    And 0 Mermaid diagrams were inspected
