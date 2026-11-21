# End-to-End Testing

End-to-end here means the process contract and nothing else: arguments in, exit code and streams out. The adapter spawns
the built binary and observes it from outside.

## What It May Observe

- The exit code, under the [public contract](public-contract.md).
- Standard output, including a documented machine-readable format.
- Standard error.
- The state of the temporary tree afterwards — unchanged for validators, and exactly the declared transaction result for
  an explicit mutator.

## What It May Not Do

- Reach inside the process. A test that calls a function is a unit test that happens to be slow.
- Depend on a tree it did not create, or leave one behind.
- Depend on another case's ordering or leftovers.
- Reach the network. The product opens no socket, and neither does its test.

## Where It Runs

Never in a Git hook and never in the quick gate. The integration target runs in the pull-request gate, because it stays
on the machine. The end-to-end target runs in a scheduled or `main` workflow, because a gate slow enough to be worked
around is a gate that will be (C17 of the
[contract](quality/checks/test-and-gate-contract/004-test-levels-and-targets.md)). Keep local E2E runs selective to the
touched flow. The daily `Scheduled` workflow runs the scheduled surface: the quick gate and this end-to-end target.

That placement is itself asserted: the static check verifies that no gate file invokes a slow adapter, and it reads that
list by filename — so renaming a gate file without telling the check is caught rather than silently ignored.

## Fixtures

Build a temporary tree per case and remove it. There is no shared fixture directory. A shared fixture is a test that
passes because of another test.

## Related

- [Quality gates](quality-gates.md)
- [Behaviour-driven development](behaviour-driven-development.md)
