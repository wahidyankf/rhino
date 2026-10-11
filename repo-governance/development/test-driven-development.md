# Test-Driven Development

New or changed behaviour and every bug fix follow RED, GREEN, REFACTOR, with named evidence at each step.

## RED

Write the failing test first, and run it. Record:

- the test path and the command that ran it;
- the failure message; and
- the **stated reason** it fails — the behaviour is absent.

A test that fails because the binding is missing, the fixture is wrong, or the module does not compile is not RED. It is
a broken test, and proceeding from it proves nothing about the change.

For a bug fix, RED reproduces the reported behaviour. A fix without a reproducing test is a change of unknown effect.

## GREEN

Write the smallest production change that makes it pass. Record the passing output. Do not improve anything else while
red-to-green is in flight; a refactor mixed into GREEN hides which change did the work.

## REFACTOR

Improve the code with the tests staying green throughout. Record the green result after refactoring. Behaviour does not
change here — if it does, that was a new RED you skipped.

## Pure Refactors

A refactor with no behaviour change starts from a green baseline, recorded before touching anything. When the existing
tests do not pin the behaviour being moved, write characterization tests first, prove they pass against the current
code, and only then refactor. Moving code with no test watching it is not a refactor; it is a rewrite with an optimistic
name.

## Evidence

Each increment is a separate item in the [task list](../conventions/task-tracking.md), with its evidence attached.
"Tests pass" is not evidence; the command and its output are.

## Focused Runs and Regression Gates

For each local RED and GREEN item, run the named case or smallest relevant test selection that demonstrates the
behaviour; do not rerun the whole suite for every item. Run the named affected behaviours and shared boundaries after
each REFACTOR step, expanding the selection when the change affects more code.

Record the exact command, test path, selected case identities, positive executed case count, exit status, and observed
result for each step. An unmatched filter, zero executed cases, or cached output that executes nothing is not proof.

Use the raw runner's selection when an aggregate enforces coverage over code deliberately omitted from a focused run. Do
not lower coverage floors or change their exclusions or full-coverage targets. Keep required build, code generation,
resource admission, fixture isolation, and cleanup around every selected run.

At each phase end and before creating or updating the pull request, run the complete affected mandatory regression,
coverage, and quick gates, plus applicable Integration and E2E gates. One recorded complete run may satisfy both
boundaries when the tested code, tests, configuration, and affected scope are unchanged; otherwise rerun the affected
gates. Focused runs replace no hook, CI, or delivery gate.

Followed evidence names relevant executed cases with positive counts and the complete boundary runs. A per-item whole
suite, a selection omitting an affected shared boundary, nonexecuting evidence, or a missing required complete gate
violates this rule. Existing TDD scope exceptions remain unchanged.

**Enforcement: unenforced by decision.** Selecting the meaningful affected scope and proving its recorded evidence
require judgement; static text checks do not establish semantic test execution.

## Related

- [Red, green, refactor](../workflows/quality/red-green-refactor.md)
- [Specification maintenance](specification-maintenance.md)
