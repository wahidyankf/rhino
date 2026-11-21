Feature: Behaviour coverage

  A scenario in a declared corpus must be bound to a unit test, and must be bound to an integration and an end-to-end
  test or carry a valid exemption for each. RHINO reads the text of the corpus and the test files and checks these
  bindings. It runs no test and starts no process, so a binding it accepts is still proven only by the real test runs.

  Background:
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      policies:
        behaviour:
          corpus:
            - specs/behaviours
          fixtures:
            - test/fixtures
          levels:
            unit:
              - internal/**/*_test.go
            integration:
              - tests/integration/**
            e2e:
              - tests/e2e/**
      """
    And file "specs/behaviours/status.feature" contains this Markdown:
      """
      Feature: Status

        # Exemption(integration): the status view has no integration boundary; alternative-proof: internal/application/status_test.go
        @integration-exempt
        # Exemption(e2e): the status view has no end-to-end boundary; alternative-proof: internal/application/status_test.go
        @e2e-exempt
        Scenario: Status shows queued work
          Given a queued job
          When status runs
          Then the queue is shown
      """
    And file "internal/application/status_test.go" contains this Markdown:
      """
      package application

      // @scenario specs/behaviours/status.feature: Status shows queued work
      func TestStatusShowsQueuedWork() {}
      """
    And file "tests/integration/smoke_test.go" contains this Markdown:
      """
      package integration
      """
    And file "tests/e2e/smoke_test.go" contains this Markdown:
      """
      package e2e
      """

  Scenario: A corpus whose scenarios are all bound or exempt passes
    When I invoke the CLI with "behaviour|coverage|validate"
    Then the exit code is 0
    And stdout contains "no findings"

  # Behaviour: RB-04
  # Behaviour: RB-83
  Scenario: A scenario with no unit reference is reported (B01)
    Given file "internal/application/status_test.go" contains this Markdown:
      """
      package application

      func TestStatusShowsQueuedWork() {}
      """
    When I invoke the CLI with "behaviour|coverage|validate|--output|json"
    Then the exit code is 1
    And the first stdout JSON violation kind is "B01"

  Scenario: A scenario with an integration reference needs no integration exemption (B02 passes)
    Given file "specs/behaviours/status.feature" contains this Markdown:
      """
      Feature: Status

        # Exemption(e2e): the status view has no end-to-end boundary; alternative-proof: internal/application/status_test.go
        @e2e-exempt
        Scenario: Status shows queued work
          Given a queued job
          When status runs
          Then the queue is shown
      """
    And file "tests/integration/status_test.go" contains this Markdown:
      """
      package integration

      // @scenario specs/behaviours/status.feature: Status shows queued work
      func TestStatusIntegration() {}
      """
    When I invoke the CLI with "behaviour|coverage|validate"
    Then the exit code is 0

  Scenario: A scenario with neither an integration reference nor an exemption is reported (B02)
    Given file "specs/behaviours/status.feature" contains this Markdown:
      """
      Feature: Status

        # Exemption(e2e): the status view has no end-to-end boundary; alternative-proof: internal/application/status_test.go
        @e2e-exempt
        Scenario: Status shows queued work
          Given a queued job
          When status runs
          Then the queue is shown
      """
    When I invoke the CLI with "behaviour|coverage|validate|--output|json"
    Then the exit code is 1
    And the first stdout JSON violation kind is "B02"

  Scenario: A scenario with an end-to-end reference needs no end-to-end exemption (B03 passes)
    Given file "specs/behaviours/status.feature" contains this Markdown:
      """
      Feature: Status

        # Exemption(integration): the status view has no integration boundary; alternative-proof: internal/application/status_test.go
        @integration-exempt
        Scenario: Status shows queued work
          Given a queued job
          When status runs
          Then the queue is shown
      """
    And file "tests/e2e/status_test.go" contains this Markdown:
      """
      package e2e

      // @scenario specs/behaviours/status.feature: Status shows queued work
      func TestStatusEndToEnd() {}
      """
    When I invoke the CLI with "behaviour|coverage|validate"
    Then the exit code is 0

  Scenario: A scenario with neither an end-to-end reference nor an exemption is reported (B03)
    Given file "specs/behaviours/status.feature" contains this Markdown:
      """
      Feature: Status

        # Exemption(integration): the status view has no integration boundary; alternative-proof: internal/application/status_test.go
        @integration-exempt
        Scenario: Status shows queued work
          Given a queued job
          When status runs
          Then the queue is shown
      """
    When I invoke the CLI with "behaviour|coverage|validate|--output|json"
    Then the exit code is 1
    And the first stdout JSON violation kind is "B03"

  Scenario: An exemption tag without the comment directly above it is reported (B04)
    Given file "specs/behaviours/status.feature" contains this Markdown:
      """
      Feature: Status

        @integration-exempt
        # Exemption(e2e): the status view has no end-to-end boundary; alternative-proof: internal/application/status_test.go
        @e2e-exempt
        Scenario: Status shows queued work
          Given a queued job
          When status runs
          Then the queue is shown
      """
    When I invoke the CLI with "behaviour|coverage|validate|--output|json"
    Then the exit code is 1
    And the first stdout JSON violation kind is "B04"

  Scenario: A unit exemption tag is an unknown exemption and is reported (B04)
    Given file "specs/behaviours/status.feature" contains this Markdown:
      """
      Feature: Status

        # Exemption(unit): the status view has no unit boundary; alternative-proof: internal/application/status_test.go
        @unit-exempt
        # Exemption(integration): the status view has no integration boundary; alternative-proof: internal/application/status_test.go
        @integration-exempt
        # Exemption(e2e): the status view has no end-to-end boundary; alternative-proof: internal/application/status_test.go
        @e2e-exempt
        Scenario: Status shows queued work
          Given a queued job
          When status runs
          Then the queue is shown
      """
    When I invoke the CLI with "behaviour|coverage|validate|--output|json"
    Then the exit code is 1
    And the first stdout JSON violation kind is "B04"

  Scenario: An exemption reason on the deny list is reported (B04)
    Given file "specs/behaviours/status.feature" contains this Markdown:
      """
      Feature: Status

        # Exemption(integration): the integration run is too slow for the runtime budget; alternative-proof: internal/application/status_test.go
        @integration-exempt
        # Exemption(e2e): the status view has no end-to-end boundary; alternative-proof: internal/application/status_test.go
        @e2e-exempt
        Scenario: Status shows queued work
          Given a queued job
          When status runs
          Then the queue is shown
      """
    When I invoke the CLI with "behaviour|coverage|validate|--output|json"
    Then the exit code is 1
    And the first stdout JSON violation kind is "B04"

  Scenario: A whole-file reference is accepted in a test file that a whole-file glob matches (B05 passes)
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      policies:
        behaviour:
          corpus:
            - specs/behaviours
          fixtures:
            - test/fixtures
          whole-file:
            - internal/**/*_test.go
          levels:
            unit:
              - internal/**/*_test.go
            integration:
              - tests/integration/**
            e2e:
              - tests/e2e/**
      """
    And file "internal/application/status_test.go" contains this Markdown:
      """
      package application

      // @scenario specs/behaviours/status.feature: *
      func TestStatusShowsQueuedWork() {}
      """
    When I invoke the CLI with "behaviour|coverage|validate"
    Then the exit code is 0

  Scenario: A whole-file reference in a test file no whole-file glob matches is reported (B05)
    Given file "internal/application/status_test.go" contains this Markdown:
      """
      package application

      // @scenario specs/behaviours/status.feature: *
      func TestStatusShowsQueuedWork() {}
      """
    When I invoke the CLI with "behaviour|coverage|validate|--output|json"
    Then the exit code is 1
    And the first stdout JSON violation kind is "B05"

  Scenario: A reference that names a scenario the feature does not hold is reported (B05)
    Given file "internal/application/status_test.go" contains this Markdown:
      """
      package application

      // @scenario specs/behaviours/status.feature: Status shows idle work
      func TestStatusShowsQueuedWork() {}
      """
    When I invoke the CLI with "behaviour|coverage|validate|--output|json"
    Then the exit code is 1
    And the first stdout JSON violation kind is "B05"

  Scenario: An exemption that a reference at the same level also covers is stale (B06)
    Given file "specs/behaviours/status.feature" contains this Markdown:
      """
      Feature: Status

        # Exemption(integration): the status view has no integration boundary; alternative-proof: internal/application/status_test.go
        @integration-exempt
        # Exemption(e2e): the status view has no end-to-end boundary; alternative-proof: internal/application/status_test.go
        @e2e-exempt
        Scenario: Status shows queued work
          Given a queued job
          When status runs
          Then the queue is shown
      """
    And file "tests/integration/status_test.go" contains this Markdown:
      """
      package integration

      // @scenario specs/behaviours/status.feature: Status shows queued work
      func TestStatusIntegration() {}
      """
    When I invoke the CLI with "behaviour|coverage|validate"
    Then the exit code is 1
    And stderr contains "stale exemption — remove the tag"

  Scenario: A feature file outside the corpus and the fixtures is reported (B07)
    Given file "specs/other/status.feature" contains this Markdown:
      """
      Feature: Other

        Scenario: Other holds nothing
          Given nothing
      """
    When I invoke the CLI with "behaviour|coverage|validate|--output|json"
    Then the exit code is 1
    And the first stdout JSON violation kind is "B07"

  Scenario: A feature file that is a fixture input passes (B07 passes)
    Given file "test/fixtures/sample.feature" contains this Markdown:
      """
      Feature: Sample

        Scenario: Sample holds nothing
          Given nothing
      """
    When I invoke the CLI with "behaviour|coverage|validate"
    Then the exit code is 0

  Scenario: Two scenarios with one name in one feature file are reported (B08)
    Given file "specs/behaviours/status.feature" contains this Markdown:
      """
      Feature: Status

        # Exemption(integration): the status view has no integration boundary; alternative-proof: internal/application/status_test.go
        @integration-exempt
        # Exemption(e2e): the status view has no end-to-end boundary; alternative-proof: internal/application/status_test.go
        @e2e-exempt
        Scenario: Status shows queued work
          Given a queued job
          When status runs
          Then the queue is shown

        # Exemption(integration): the status view has no integration boundary; alternative-proof: internal/application/status_test.go
        @integration-exempt
        # Exemption(e2e): the status view has no end-to-end boundary; alternative-proof: internal/application/status_test.go
        @e2e-exempt
        Scenario: Status shows queued work
          Given a queued job
          When status runs
          Then the queue is shown
      """
    When I invoke the CLI with "behaviour|coverage|validate|--output|json"
    Then the exit code is 1
    And the first stdout JSON violation kind is "B08"

  Scenario: A corpus root that holds no scenario is reported (B09)
    Given file "specs/behaviours/status.feature" contains this Markdown:
      """
      Feature: Status
      """
    And file "internal/application/status_test.go" contains this Markdown:
      """
      package application

      func TestStatusShowsQueuedWork() {}
      """
    When I invoke the CLI with "behaviour|coverage|validate|--output|json"
    Then the exit code is 1
    And the first stdout JSON violation kind is "B09"

  Scenario: A level glob that matches no test file is reported (B09)
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      policies:
        behaviour:
          corpus:
            - specs/behaviours
          fixtures:
            - test/fixtures
          levels:
            unit:
              - internal/**/*_test.go
            integration:
              - tests/integration/**
            e2e:
              - tests/e2e-missing/**
      """
    When I invoke the CLI with "behaviour|coverage|validate|--output|json"
    Then the exit code is 1
    And the first stdout JSON violation kind is "B09"

  Scenario: The rules named by --rules are the only rules that run (--rules passes)
    Given file "internal/application/status_test.go" contains this Markdown:
      """
      package application

      func TestStatusShowsQueuedWork() {}
      """
    When I invoke the CLI with "behaviour|coverage|validate|--rules|B04,B05,B06,B07,B08"
    Then the exit code is 0

  Scenario: A named rule that fails is reported under --rules (--rules fails)
    Given file "internal/application/status_test.go" contains this Markdown:
      """
      package application

      func TestStatusShowsQueuedWork() {}
      """
    When I invoke the CLI with "behaviour|coverage|validate|--rules|B01|--output|json"
    Then the exit code is 1
    And the first stdout JSON violation kind is "B01"

  Scenario: An unknown rule named by --rules refuses the run
    When I invoke the CLI with "behaviour|coverage|validate|--rules|B10"
    Then the exit code is 2
    And stderr contains "B10"

  # Behaviour: RB-74
  Scenario: A corpus without a declared policies.behaviour section is refused
    Given the configuration file is this text:
      """
      schema: rhino/repo-config/v2
      """
    When I invoke the CLI with "behaviour|coverage|validate|--output|json"
    Then the exit code is 2
    And stderr contains "rhino.config.undeclared"

  # Behaviour: RB-11
  Scenario: The inspection reads the corpus and changes no file
    When I invoke the CLI with "behaviour|coverage|validate"
    Then the repository is unchanged by the inspection
