# How to wire RHINO into your gates

You want RHINO's answer before a change lands, not after. Three places are
worth wiring, and they are not interchangeable.

## Where each check belongs

| Place         | Runs            | Why                                                              |
| ------------- | --------------- | ---------------------------------------------------------------- |
| Local command | every validator | The one a maintainer runs on purpose while working.              |
| Push hook     | every validator | Fast enough to sit in front of a push, and catches it before CI. |
| CI            | every validator | The answer of record, on a machine nobody configured by hand.    |

RHINO reads files and writes nothing, spawns no process, and opens no socket,
so it is cheap enough to run in all three.

## One script, three callers

Put the loop in one place so the three cannot drift apart.

```sh
#!/bin/sh
# scripts/hygiene.sh
set -u

worst=0
for command in \
  "repo-config validate" \
  "governance word-budget validate" \
  "governance directory-map validate" \
  "md internal-link validate" \
  "md mermaid validate" \
  "harness parity validate"
do
  status=0
  # shellcheck disable=SC2086
  rhino $command || status=$?
  case "$status:$worst" in
    2:*) worst=2 ;;
    1:0) worst=1 ;;
  esac
done
exit "$worst"
```

`repo-config validate` goes first. If the configuration is unusable every other
command exits `2` for the same reason, and one clear message beats five copies.

## The push hook

```sh
#!/bin/sh
# .husky/pre-push  (or .git/hooks/pre-push)
./scripts/hygiene.sh
```

Do not add a bypass flag to the hook. A hook that can be skipped by a flag
someone remembers under pressure is a hook that is skipped exactly when it
matters.

## CI

```yaml
- name: Repository hygiene
  run: ./scripts/hygiene.sh
```

Pin the binary the same way you pin any other tool. See
[how to install a pinned release](./install-a-pinned-release.md).

## Prove the gate can fail

A gate nobody has seen fail is a gate nobody knows works. Break something on a
scratch branch and watch it go red:

```console
$ printf '\n[nowhere](does-not-exist.md)\n' >> README.md
$ rhino md internal-link validate
[internal-link] checked 5 links, 1 finding
[internal-link] README.md:5: `does-not-exist.md` does not exist
```

Then run the script itself and check what it returns, because that — not the
single command — is what your hook and your CI job actually see:

```console
$ ./scripts/hygiene.sh > /dev/null 2>&1
$ echo $?
1
```

Then revert it. Do this once when you wire the gate in, and again whenever you
change what the gate runs.

## What not to do

**Do not swallow exit `2`.** It means nothing was checked. A gate that treats
it as a pass is a gate that goes quiet exactly when the configuration breaks.

**Do not run only the validators that are currently clean.** A validator
excluded because it is noisy today is a rule the repository has stopped
declaring.

**Do not parse the text output.** Use `--output json` and filter on `kind`.

## Related

- [How to respond to an exit code](./respond-to-exit-codes.md)
- [How to consume the JSON output](./consume-the-json-output.md)
- [Exit codes](../reference/exit-codes.md)
