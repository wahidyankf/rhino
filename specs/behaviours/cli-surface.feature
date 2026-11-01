Feature: CLI build identity, repository selection, process boundaries, and exclusion scope

  Scenario: The version command prints the build identity
    When I invoke the CLI with "version"
    Then the exit code is 0
    And stdout contains "v"

  Scenario: The version command prints one schema-versioned JSON object with the flag
    When I invoke the CLI with "version|--json"
    Then the exit code is 0
    And stdout JSON property "schemaVersion" is 1
    And stdout contains "commit"

  Scenario: The version command refuses a flag it does not accept
    When I invoke the CLI with "version|--xml"
    Then the exit code is 2

  Scenario: A root naming a directory outside the working directory is accepted
    Given the directory outside the repository contains:
      | path            | content                      |
      | repo-config.yml | schema: rhino/repo-config/v2 |
    When I invoke the CLI with "--root|../outside|repo-config|validate"
    Then the exit code is 0
    And stdout contains "checked 1 configuration file"

  Scenario: A root naming a missing directory is refused as an unusable repository
    When I invoke the CLI with "--root|../missing|repo-config|validate|--output|json"
    Then the exit code is 2
    And stderr contains "rhino.repository.unusable"
    And stderr contains "names nothing usable"

  Scenario: A selection that climbs out of the chosen root is still refused
    Given the directory outside the repository contains:
      | path            | content                      |
      | repo-config.yml | schema: rhino/repo-config/v2 |
    When I invoke the CLI with "--root|../outside|md|mermaid|validate|--file|../held.md|--output|json"
    Then the exit code is 2
    And stderr contains "rhino.path.escapes-root"
    And stderr contains "escapes the repository root"

  Scenario: A gate child that cannot be found exits 127 and says so
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates:
        entries:
          - id: absent-check
            type: check
            command:
              executable: ./absent-check
            run-on:
              pre-commit: {}
              pull-request:
                reason: replay check
        composition:
          pull-request:
            relation: exact
      """
    When I invoke the CLI with "gate|run|--surface|pre-commit"
    Then the exit code is 127
    And stderr contains "executable was not found"

  Scenario: A gate child that is not executable exits 126
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates:
        entries:
          - id: unrunnable-check
            type: check
            command:
              executable: ./unrunnable-check
            run-on:
              pre-commit: {}
              pull-request:
                reason: replay check
        composition:
          pull-request:
            relation: exact
      """
    And the repository contains:
      | path              | content              |
      | unrunnable-check  | echo not a program   |
    When I invoke the CLI with "gate|run|--surface|pre-commit"
    Then the exit code is 126

  Scenario: A gate child that interrupts its CLI makes the run exit 130
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates:
        entries:
          - id: interrupted-check
            type: check
            command:
              executable: ./interrupted-check
            run-on:
              pre-commit: {}
              pull-request:
                reason: replay check
        composition:
          pull-request:
            relation: exact
      """
    And the gate "interrupted-check" interrupts the CLI with signal "INT"
    When I invoke the CLI with "gate|run|--surface|pre-commit"
    Then the exit code is 130

  Scenario: A termination signal that reaches the CLI makes the run exit 143
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates:
        entries:
          - id: terminated-check
            type: check
            command:
              executable: ./terminated-check
            run-on:
              pre-commit: {}
              pull-request:
                reason: replay check
        composition:
          pull-request:
            relation: exact
      """
    And the gate "terminated-check" interrupts the CLI with signal "TERM"
    When I invoke the CLI with "gate|run|--surface|pre-commit"
    Then the exit code is 143

  Scenario Outline: A scan exclusion keeps a Markdown validator from inspecting an excluded file
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      scan:
        exclude-directories: [vendor]
      <group>
      """
    And the repository contains:
      | path               | content                                      |
      | docs/good-name.md  | ---\ntitle: Good name\n---\n[self](good-name.md)\n |
      | vendor/Bad_Name.md | # Bad name\n                                 |
    When I invoke the CLI with "<command>"
    Then the exit code is 0
    And stdout JSON property "inspected" is 1

    Examples:
      | group                                                                                 | command                                         |
      | policies: {markdown: {naming: {surfaces: [{glob: "**/*.md", style: kebab-case}], exempt: []}}} | md\|naming\|validate\|--output\|json |
      | policies: {markdown: {frontmatter: {surfaces: [{glob: "**/*.md", require: [title]}]}}}        | md\|frontmatter\|validate\|--output\|json |
      | policies: {markdown: {internal-link: {}}}                                                                   | md\|internal-link\|validate\|--output\|json |

  Scenario: A toolchain probe that writes past the output bound yields no output bytes
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      toolchains:
        entries:
          - id: chatty
            executable: ./chatty-tool
            probe: [--version]
            required: true
      """
    And the repository contains an executable "chatty-tool" with the body:
      """
      #!/bin/sh
      yes x | head -c 70000
      """
    When I invoke the CLI with "toolchain|validate"
    Then the exit code is 1
    And stderr contains "chatty: unavailable"

  Scenario: A toolchain probe that outlives its timeout is cut short
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      toolchains:
        entries:
          - id: slow
            executable: ./slow-tool
            probe: [--version]
            timeout-seconds: 1
            required: true
      """
    And the repository contains an executable "slow-tool" with the body:
      """
      #!/bin/sh
      sleep 30
      """
    When I invoke the CLI with "toolchain|validate"
    Then the exit code is 1
    And stderr contains "slow: unavailable"
