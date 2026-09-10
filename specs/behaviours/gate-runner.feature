Feature: Gate dispatch

  A repository declaring ose/repo-config/v2 owns an ordered list of gates. RHINO
  does not decide what a gate does; it decides which gates a surface selects,
  what order they run in, what each child is told, and when the sequence stops.
  Everything a child is handed is stated here rather than assembled from the
  environment RHINO happens to be started in, because a hook that behaved one
  way under Git and another under CI would be a gate nobody could trust.

  The children in these scenarios are recorders: each one writes down the
  argument vector it was given, the surface it was told, and the standard input
  it read, then exits with the code the scenario declared for it.

  Scenario: A surface selects the gates that name it, in declaration order
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: first
          kind: check
          run:
            - ./gates/first.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
        - id: second
          kind: check
          run:
            - ./gates/second.sh
          surfaces:
            - pre-commit
        - id: third
          kind: check
          run:
            - ./gates/third.sh
          surfaces:
            - pre-commit
            - pre-push
      """
    And the gate "first" exits 0
    And the gate "second" exits 0
    And the gate "third" exits 0
    When I run the gates for the "pre-commit" surface
    Then the exit code is 0
    And the gates that ran are "first|second|third"

  Scenario: A gate that does not name the surface is not run
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: first
          kind: check
          run:
            - ./gates/first.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
        - id: second
          kind: check
          run:
            - ./gates/second.sh
          surfaces:
            - pre-commit
      """
    And the gate "first" exits 0
    And the gate "second" exits 0
    When I run the gates for the "pre-push" surface
    Then the exit code is 0
    And the gates that ran are "first"

  Scenario: Declaration order is the run order even when it is not alphabetical
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: zebra
          kind: check
          run:
            - ./gates/zebra.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
        - id: alpaca
          kind: check
          run:
            - ./gates/alpaca.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    And the gate "zebra" exits 0
    And the gate "alpaca" exits 0
    When I run the gates for the "commit-msg" surface
    Then the exit code is 0
    And the gates that ran are "zebra|alpaca"

  Scenario: The sequence stops at the first gate that fails
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: first
          kind: check
          run:
            - ./gates/first.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
        - id: second
          kind: check
          run:
            - ./gates/second.sh
          surfaces:
            - pre-commit
        - id: third
          kind: check
          run:
            - ./gates/third.sh
          surfaces:
            - pre-commit
      """
    And the gate "first" exits 0
    And the gate "second" exits 7
    And the gate "third" exits 0
    When I run the gates for the "pre-commit" surface
    Then the exit code is 1
    And the gates that ran are "first|second"

  Scenario: A mutation does not run after an earlier check has failed
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
        - id: formatter
          kind: mutation
          run:
            - ./gates/format.sh
          surfaces:
            - pre-commit
      """
    And the gate "hygiene" exits 3
    And the gate "formatter" exits 0
    When I run the gates for the "pre-commit" surface
    Then the exit code is 1
    And the gates that ran are "hygiene"

  Scenario: A public repository runs public-safety before anything else
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: public
      gates:
        - id: public-safety
          kind: check
          run:
            - ./gates/public-safety.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - pre-commit
      """
    And the gate "public-safety" exits 1
    And the gate "hygiene" exits 0
    When I run the gates for the "pre-commit" surface
    Then the exit code is 1
    And the gates that ran are "public-safety"

  Scenario: Each child is told exactly the surface that selected it
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    And the gate "hygiene" exits 0
    When I run the gates for the "pre-push" surface
    Then the exit code is 0
    And the gate "hygiene" was told the surface "pre-push"

  Scenario: The declared vector reaches the child unsplit and uninterpolated
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
            - --title
            - two words
            - $HOME;rm -rf .
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    And the gate "hygiene" exits 0
    When I run the gates for the "pre-commit" surface
    Then the exit code is 0
    And the gate "hygiene" received the argument vector "--title|two words|$HOME;rm -rf ."

  Scenario: The hook's own arguments are forwarded after the declared vector
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: message
          kind: check
          run:
            - ./gates/message.sh
            - --strict
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    And the gate "message" exits 0
    When I run the gates for the "commit-msg" surface with the arguments ".git/COMMIT_EDITMSG"
    Then the exit code is 0
    And the gate "message" received the argument vector "--strict|.git/COMMIT_EDITMSG"

  Scenario: Every selected child reads the same standard input
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: first
          kind: check
          run:
            - ./gates/first.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
        - id: second
          kind: check
          run:
            - ./gates/second.sh
          surfaces:
            - pre-push
      """
    And the gate "first" exits 0
    And the gate "second" exits 0
    When I run the gates for the "pre-push" surface with this standard input:
      """
      refs/heads/main 0000 refs/heads/main 1111
      """
    Then the exit code is 0
    And the gate "first" read the standard input "refs/heads/main 0000 refs/heads/main 1111"
    And the gate "second" read the standard input "refs/heads/main 0000 refs/heads/main 1111"

  Scenario: A child runs in the repository root rather than wherever RHINO started
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    And the gate "hygiene" exits 0
    When I run the gates for the "pre-commit" surface
    Then the exit code is 0
    And the gate "hygiene" ran in the repository root

  Scenario: Children run one at a time
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: first
          kind: check
          run:
            - ./gates/first.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
        - id: second
          kind: check
          run:
            - ./gates/second.sh
          surfaces:
            - pre-commit
      """
    And the gate "first" exits 0
    And the gate "second" exits 0
    When I run the gates for the "pre-commit" surface
    Then the exit code is 0
    And no two gates overlapped

  Scenario: A gate that cannot be launched is a protocol failure, not a finding
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    And the gate "hygiene" cannot be launched
    When I run the gates for the "pre-commit" surface
    Then the exit code is 3
    And stderr contains "hygiene"

  Scenario: A configuration the runner cannot use is refused before any child
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: hygiene
          kind: mutation
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    And the gate "hygiene" exits 0
    When I run the gates for the "pre-commit" surface
    Then the exit code is 2
    And no gate ran
    And stderr contains "a mutation may run only at pre-commit"

  Scenario: A surface outside the closed set is an invalid invocation
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    And the gate "hygiene" exits 0
    When I run the gates for the "post-merge" surface
    Then the exit code is 2
    And no gate ran
    And stderr contains "post-merge"

  Scenario: A v1 repository has no gates to dispatch
    Given the repository declares a complete configuration
    When I run the gates for the "pre-commit" surface
    Then the exit code is 2
    And no gate ran
    And stderr contains "gates"

  Scenario: A surface no gate names runs nothing and refuses nothing
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    And the gate "hygiene" exits 0
    When I run the gates for the "ci" surface
    Then the exit code is 0
    And no gate ran

  Scenario: The report names each gate it ran and how that gate ended
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: first
          kind: check
          run:
            - ./gates/first.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
        - id: second
          kind: check
          run:
            - ./gates/second.sh
          surfaces:
            - pre-commit
      """
    And the gate "first" exits 0
    And the gate "second" exits 9
    When I run the gates for the "pre-commit" surface
    Then the exit code is 1
    And stdout names the gate "first" as "passed"
    And stdout names the gate "second" as "failed"

  Scenario: The report repeats nothing a child wrote
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: hygiene
          kind: check
          run:
            - ./gates/hygiene.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
      """
    And the gate "hygiene" exits 1
    When I run the gates for the "pre-commit" surface
    Then the exit code is 1
    And no output repeats what a child wrote

  Scenario: The same dispatch twice reports the same thing
    Given the configuration file is this text:
      """
      schema: ose/repo-config/v2
      visibility: private
      gates:
        - id: first
          kind: check
          run:
            - ./gates/first.sh
          surfaces:
            - commit-msg
            - pre-commit
            - pre-push
        - id: second
          kind: check
          run:
            - ./gates/second.sh
          surfaces:
            - pre-commit
      """
    And the gate "first" exits 0
    And the gate "second" exits 4
    When I run the gates for the "pre-commit" surface
    And I run the gates for the "pre-commit" surface
    Then the exit code is 1
    And both runs reported the same thing
