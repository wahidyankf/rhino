Feature: Regular file gate inputs

  Scenario: A regular-files input accepts a Git index binding
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates:
        entries:
          - id: format-files
            type: check
            inputs:
              files: { kind: regular-files }
            command:
              executable: formatter
              args:
                - { input: files.paths, expand: repeat }
            run-on:
              pre-commit:
                bind:
                  files: { source: git-index }
              pull-request:
                bind:
                  files: { source: explicit-range, range: explicit }
        composition:
          pull-request:
            relation: exact
      """
    When I invoke the CLI with "gate|validate"
    Then the exit code is 0

  Scenario: A regular-files input accepts an explicit-range binding
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates:
        entries:
          - id: format-files
            type: check
            inputs:
              files: { kind: regular-files }
            command:
              executable: formatter
              args:
                - { input: files.paths, expand: repeat }
            run-on:
              pre-commit:
                bind:
                  files: { source: git-index }
              pull-request:
                bind:
                  files: { source: explicit-range, range: explicit }
        composition:
          pull-request:
            relation: exact
      """
    When I invoke the CLI with "gate|validate"
    Then the exit code is 0

  Scenario Outline: A regular-files input refuses an unsupported source
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates:
        entries:
          - id: format-files
            type: check
            inputs:
              files: { kind: regular-files }
            command:
              executable: formatter
              args:
                - { input: files.paths, expand: repeat }
            run-on:
              manual:
                bind:
                  files: { source: <source> }
      """
    When I invoke the CLI with "gate|validate"
    Then the exit code is 2
    And stderr contains "not a compatible source"

    Examples:
      | source            |
      | checkout          |
      | hook-message-file |
      | push-updates      |

  Scenario: Regular-files index projection uses the selected Git snapshot
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates:
        entries:
          - id: format-files
            type: check
            inputs:
              files: { kind: regular-files }
            command:
              executable: gates/format-files.sh
              args:
                - { input: files.paths, expand: repeat }
            run-on:
              pre-commit:
                bind:
                  files: { source: git-index }
              pull-request:
                bind:
                  files: { source: explicit-range, range: explicit }
        composition:
          pull-request:
            relation: exact
      """
    And the repository contains:
      | path              | content             |
      | staged/source.md  | index base          |
      | range/source.md   | range working bytes |
    And the explicit range selects the regular file "range/source.md"
    And the Git index selects the regular file "staged/source.md"
    And the working tree path "staged/source.md" is a symbolic link to "../range/source.md"
    And the gate "format-files" exits 0
    When I invoke the CLI with "gate|run|--surface|pre-commit"
    Then the exit code is 0
    And the gate "format-files" received the argument vector "staged/source.md"

  Scenario: Regular-files range projection uses the immutable Git snapshot
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates:
        entries:
          - id: format-files
            type: check
            inputs:
              files: { kind: regular-files }
            command:
              executable: gates/format-files.sh
              args:
                - { input: files.paths, expand: repeat }
            run-on:
              pull-request:
                bind:
                  files: { source: explicit-range, range: explicit }
              pre-commit:
                bind:
                  files: { source: git-index }
        composition:
          pull-request:
            relation: exact
      """
    And the repository contains:
      | path              | content             |
      | staged/source.md  | index working bytes |
      | range/source.md   | range base          |
    And the explicit range selects the regular file "range/source.md"
    And the Git index selects the regular file "staged/source.md"
    And the working tree path "range/source.md" is a symbolic link to "../staged/source.md"
    And the gate "format-files" exits 0
    When I invoke the CLI with "gate|run|--surface|pull-request|--base|{base}|--head|{head}"
    Then the exit code is 0
    And the gate "format-files" received the argument vector "range/source.md"

  Scenario: Regular-files projections preserve argv and environment values
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates:
        entries:
          - id: format-files
            type: check
            inputs:
              files: { kind: regular-files }
            command:
              executable: gates/format-files.sh
              args:
                - { input: files.paths, expand: repeat }
              environment:
                RHINO_FILE: { input: files.paths }
            run-on:
              pre-commit:
                bind:
                  files: { source: git-index }
              pull-request:
                bind:
                  files: { source: explicit-range, range: explicit }
        composition:
          pull-request:
            relation: exact
      """
    And the repository contains:
      | path                          | content     |
      | projected/name with spaces.md | index base  |
      | range/project.md              | range base  |
    And the Git index selects the regular file "projected/name with spaces.md"
    And the explicit range selects the regular file "range/project.md"
    And the gate "format-files" exits 0
    When I invoke the CLI with "gate|run|--surface|pre-commit"
    Then the exit code is 0
    And the gate "format-files" received the argument vector "projected/name with spaces.md"
    And the gate "format-files" received the environment projection "RHINO_FILE=projected/name with spaces.md"

  Scenario: Skip an empty regular-files mutation on pre-commit
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates:
        entries:
          - id: format-files
            type: mutation
            inputs:
              files: { kind: regular-files }
            mutation:
              local: apply-index
              ci: verify-clean
            command:
              executable: ./must-not-run-child
              args:
                - { input: files.paths, expand: repeat }
            run-on:
              pre-commit:
                bind:
                  files: { source: git-index }
              pull-request:
                bind:
                  files: { source: explicit-range, range: explicit }
        composition:
          pull-request:
            relation: exact
      """
    And the Git index selects no regular files
    When I invoke the CLI with "gate|run|--surface|pre-commit"
    Then the exit code is 0

  Scenario: Skip an empty regular-files mutation on pull-request
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates:
        entries:
          - id: format-files
            type: mutation
            inputs:
              files: { kind: regular-files }
            mutation:
              local: apply-index
              ci: verify-clean
            command:
              executable: ./must-not-run-child
              args:
                - { input: files.paths, expand: repeat }
            run-on:
              pre-commit:
                bind:
                  files: { source: git-index }
              pull-request:
                bind:
                  files: { source: explicit-range, range: explicit }
        composition:
          pull-request:
            relation: exact
      """
    And the explicit range selects no regular files
    When I invoke the CLI with "gate|run|--surface|pull-request|--base|{base}|--head|{head}"
    Then the exit code is 0

  Scenario: Run a mixed-input mutation when a legacy file input selects a path
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates:
        entries:
          - id: format-files
            type: mutation
            inputs:
              regular: { kind: regular-files }
              legacy: { kind: files }
            mutation:
              local: apply-index
              ci: verify-clean
            command:
              executable: gates/format-files.sh
              args:
                - { input: regular.paths, expand: repeat }
                - { input: legacy.paths, expand: repeat }
            run-on:
              pre-commit:
                bind:
                  regular: { source: git-index }
                  legacy: { source: explicit-range, range: explicit }
              pull-request:
                bind:
                  regular: { source: git-index }
                  legacy: { source: explicit-range, range: explicit }
        composition:
          pull-request:
            relation: exact
      """
    And the repository contains:
      | path           | content     |
      | notes/range.md | range bytes |
    And the Git index selects no regular files
    And the explicit range selects the regular file "notes/range.md"
    And the declared mutation runs against its isolated snapshot
    And the gate "format-files" exits 0
    When I invoke the CLI with "gate|run|--surface|pull-request|--base|{base}|--head|{head}"
    Then the exit code is 0
    And the gate "format-files" received the argument vector "notes/range.md"

  Scenario: Run a regular-files check when its selection is empty
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      gates:
        entries:
          - id: format-files
            type: check
            inputs:
              files: { kind: regular-files }
            command:
              executable: gates/format-files.sh
              args:
                - { input: files.paths, expand: repeat }
            run-on:
              pre-commit:
                bind:
                  files: { source: git-index }
              pull-request:
                bind:
                  files: { source: explicit-range, range: explicit }
        composition:
          pull-request:
            relation: exact
      """
    And the Git index selects no regular files
    And the gate "format-files" exits 0
    When I invoke the CLI with "gate|run|--surface|pre-commit"
    Then the exit code is 0
    And the gate "format-files" received the argument vector ""
