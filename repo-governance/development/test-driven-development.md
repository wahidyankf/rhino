# Test-Driven Development

New or changed behaviour and every bug fix follow RED, GREEN, REFACTOR, with named evidence at each step.

## RED

Write the failing test first, and run it. Record:

- the test path and the command that ran it;
- the failure message; and
- the **stated reason** it fails — the behaviour is absent.

A test that fails because the binding is missing, the fixture is wrong, or the module does not compile is not RED. It is a broken test, and proceeding from it proves nothing about the change.

For a bug fix, RED reproduces the reported behaviour. A fix without a reproducing test is a change of unknown effect.

## GREEN

Write the smallest production change that makes it pass. Record the passing output. Do not improve anything else while red-to-green is in flight; a refactor mixed into GREEN hides which change did the work.

## REFACTOR

Improve the code with the tests staying green throughout. Record the green result after refactoring. Behaviour does not change here — if it does, that was a new RED you skipped.

## Pure Refactors

A refactor with no behaviour change starts from a green baseline, recorded before touching anything. When the existing tests do not pin the behaviour being moved, write characterization tests first, prove they pass against the current code, and only then refactor. Moving code with no test watching it is not a refactor; it is a rewrite with an optimistic name.

## Evidence

Each increment is a separate item in the [task list](../conventions/task-tracking.md), with its evidence attached. "Tests pass" is not evidence; the command and its output are.

## Related

- [Red, green, refactor](../workflows/red-green-refactor.md)
- [Specification maintenance](specification-maintenance.md)
