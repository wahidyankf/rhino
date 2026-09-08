# End-to-End Testing

End-to-end here means the process contract and nothing else: arguments in, exit code and streams out. The adapter spawns the built binary and observes it from outside.

## What It May Observe

- The exit code, under the [public contract](public-contract.md).
- Standard output, including a documented machine-readable format.
- Standard error.
- The state of the temporary tree afterwards — which, for a read-only product, means proving it is unchanged.

## What It May Not Do

- Reach inside the process. A test that calls a function is a unit test that happens to be slow.
- Depend on a tree it did not create, or leave one behind.
- Depend on another case's ordering or leftovers.
- Reach the network. The product opens no socket, and neither does its test.

## Where It Runs

Never in a Git hook and never in the quick gate. The end-to-end and integration adapters run on a schedule, because a gate slow enough to be worked around is a gate that will be.

That placement is itself asserted: the static check verifies that no gate file invokes a slow adapter, and it reads that list by filename — so renaming a gate file without telling the check is caught rather than silently ignored.

## Fixtures

Build a temporary tree per case and remove it. There is no shared fixture directory. A shared fixture is a test that passes because of another test.

## Related

- [Quality gates](quality-gates.md)
- [Behaviour-driven development](behaviour-driven-development.md)
