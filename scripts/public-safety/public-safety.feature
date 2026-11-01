Feature: Public output is screened before it leaves the repository

  This contract lives beside the implementation rather than under
  `specs/behaviours/`. Everything in that directory is discovered by RHINO's
  Go behaviour harness and must be bound at the CLI adapter with no exemption.
  The subject here is a shell leaf with no Go port, and its cheapest honest
  proof runs a scanner against an inert runtime marker, which is not what a
  CLI-adapter binding is for. The executable binding is `tests/run.sh`: one
  shell case per scenario, named after it.

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

    Scenario: A scanner cache never exposes a scanner without its stamp
      Given a first-time run has extracted the pinned scanner but not yet stamped it
      When a second first-time run completes against the same scanner cache
      Then no scanner appears at the cache path without its stamp
      And both runs exit 0 and the cache holds exactly one published scanner
      And a run that loses the publish race keeps the winner's entry exactly as published
      And a run killed between extraction and stamping leaves no scanner at the cache path
      And a scanner with no stamp at the cache path is refused with exit 2

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
      Given an inert, unissued token-shaped marker assembled at runtime outside the repository
      When the wrapper runs its canary
      Then the scanner reports a finding
      And the canary value appears in no stream and in no retained file
      And the canary directory is removed
      And a canary that detects nothing blocks the repository scan
      And no private key is created or read

    Scenario: Canary fails closed
      Given a canary scan with missing detection, disclosure, or scanner failure
      When the wrapper runs its canary
      Then it exits 2 before scanning outbound material
      And sanitized output contains none of the inert marker
      And temporary canary data is removed, including when the wrapper is interrupted

  Rule: A finding says enough to act on and nothing more

    Scenario: A blocked run does not reproduce what blocked it
      Given outbound content carrying a credential
      When the wrapper runs
      Then it exits 1
      And each diagnostic carries a detector, a screened path, a line, and a status
      And the credential value, the raw JSON, and every scanner stream are absent

    Scenario: A generic private shape blocks
      Given outbound content carrying an absolute maintainer home path, Unix or Windows, or a private address
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

  Rule: History is outbound, not only the final tree

    Scenario: A pushed range is screened commit by commit
      Given a range one of whose commits adds a maintainer home path that a later commit deletes
      When the range is screened at pre-push or on a pull request
      Then it exits 1 and names the commit, the file, and the line
      And each commit message in the range is screened
      And content the range did not add is not screened again
      And a merge contributes only what it resolved beyond the automatic merge
      And a range missing its base or its head is a scan error
