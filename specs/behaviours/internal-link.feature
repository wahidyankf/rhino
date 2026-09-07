Feature: Markdown internal-link validation

  RHINO resolves local document targets. It never fetches an external URL and
  never reports on one, which is what the command name promises.

  Background:
    Given the repository declares a complete configuration

  Scenario: Existing local links and non-local links pass
    Given file "README.md" contains this Markdown:
      """
      # Root
      """
    And file "guides/target.md" contains this Markdown:
      """
      # Target
      """
    And file "guides/entry/entry.txt" contains this Markdown:
      """
      Guide entry
      """
    And file "guides/source.md" contains this Markdown:
      """
      [Target](target.md?raw=1#section), [root](../README.md), and [entry](entry/).
      [External](https://example.com/docs), [email](mailto:docs@example.com), and [section](#local) remain valid.
      [Reference target][target-ref].

      [target-ref]: target.md
      """
    When I run the "internal-link" validator
    Then the exit code is 0

  Scenario Outline: Missing or out-of-repository local targets fail
    Given the repository contains:
      | path             | content             |
      | guides/source.md | [Missing](<target>) |
    When I run the "internal-link" validator
    Then the exit code is 1

    Examples:
      | target           |
      | missing.md       |
      | ../../outside.md |
      | /outside.md      |

  Scenario: A reference-style definition with a missing target fails
    Given file "guides/source.md" contains this Markdown:
      """
      [Missing reference][missing-ref]

      [missing-ref]: missing.md
      """
    When I run the "internal-link" validator
    Then the exit code is 1

  Scenario: Fragment links and fenced Markdown examples are ignored
    Given file "guides/source.md" contains this Markdown:
      """
      [Current section](#details)

      ```markdown
      [Example only](missing.md)
      ```
      """
    When I run the "internal-link" validator
    Then the exit code is 0

  Scenario: A malformed local target fails without stopping inspection
    Given file "guides/a-malformed.md" contains this Markdown:
      """
      [Malformed](bad{nul}path.md)
      """
    And file "guides/z-missing.md" contains this Markdown:
      """
      [Missing](missing.md)
      """
    When I inspect internal links
    Then there are 2 violations

  Scenario: A declared excluded source is not a link source
    Given the repository declares the internal-link excluded source "archive/**"
    And file "archive/historical.md" contains this Markdown:
      """
      [Retired document](missing.md)
      """
    When I run the "internal-link" validator
    Then the exit code is 0

  Scenario: An excluded source remains a valid link target
    Given the repository declares the internal-link excluded source "archive/**"
    And file "archive/historical.md" contains this Markdown:
      """
      # Historical
      """
    And file "guides/source.md" contains this Markdown:
      """
      [Historical](../archive/historical.md)
      """
    When I run the "internal-link" validator
    Then the exit code is 0
