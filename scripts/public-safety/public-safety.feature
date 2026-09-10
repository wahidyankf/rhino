Feature: Public output is screened before it leaves the repository

  This contract lives beside the implementation rather than under
  `specs/behaviours/`. Everything in that directory is discovered by RHINO's
  Gherkin harness and must be bound in Rust at the unit, integration, and end
  to end adapters with no exemption at unit. The subject here is a shell leaf
  with no Rust port, and its cheapest honest proof runs a scanner and generates
  a credential, which is not what the unit adapter is for. The executable
  binding is `tests/run.sh`: one case per scenario, named after it.

  Background:
    Given a repository whose repo-config declares public visibility
    And the wrapper is the first gate on every publication surface

  Rule: A scan that cannot be trusted is not a scan that passed

    Scenario: The pinned scanner is the only scanner
      Given the approved digest table
      When the wrapper reports its pin
      Then it names TruffleHog 3.97.1 and this platform's approved digest
      And a platform outside the supported matrix is refused
      And a cached scanner that no longer matches what was extracted is refused

    Scenario: A malformed shape set makes publication ineligible
      Given a shape set that is absent, unreadable, empty, or malformed
      When the wrapper runs
      Then it exits 2 and names what could not be trusted
      And it never prints a line of the shape set

    Scenario: A shape set carrying a literal private term is malformed
      Given a shape set row whose kind is not a regular expression
      When the wrapper runs
      Then it exits 2
      And the reason is that a public repository's shape set states shapes only

    Scenario: A scanner error blocks
      Given a scanner that fails or emits a record shape nobody declared
      When the wrapper runs
      Then it exits 2
      And no scanner stream reaches the output

  Rule: Detection and non-disclosure are proved before real material is read

    Scenario: A synthetic canary proves the scanner detects
      Given a credential generated at runtime outside the repository
      When the wrapper runs its canary
      Then the scanner reports a finding
      And the canary value appears in no stream and in no retained file
      And the canary directory is removed
      And a canary that detects nothing blocks the repository scan

  Rule: A finding says enough to act on and nothing more

    Scenario: A blocked run does not reproduce what blocked it
      Given outbound content carrying a credential
      When the wrapper runs
      Then it exits 1
      And each diagnostic carries a detector, a screened path, a line, and a status
      And the credential value, the raw JSON, and every scanner stream are absent

    Scenario: A generic private shape blocks
      Given outbound content carrying an absolute maintainer home path or a private address
      When the wrapper runs
      Then it exits 1 for the shape class that matched

    Scenario: Clean content passes
      Given outbound content carrying nothing prohibited
      When the wrapper runs
      Then it exits 0 and says which surface was screened

  Rule: Every publication surface is named

    Scenario: The surface set is closed
      Given each declared surface in turn
      When the wrapper runs with clean content
      Then it exits 0
      And a surface outside the set is a scan error rather than a default
