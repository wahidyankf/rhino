# How to respond to a RHINO exit code

You are wiring RHINO into a script, a gate, or CI, and you need each code to
cause the right thing.

## The rule

**Exit `1` is the repository's problem. Exit `2` is yours.**

Exit `1` means RHINO ran, understood the policy, and found the repository in
breach of it. Exit `2` means RHINO never formed an opinion — it could not parse
the command, find the root, read the configuration, or open a selected file.

A gate that collapses the two will tell a maintainer to shorten a document when
the real problem is a typo in a flag.

## Handle them separately

```sh
#!/bin/sh
set -eu

rhino governance word-budget validate
status=$?

case $status in
  0) ;;
  1)
    echo "policy violation: see the findings above" >&2
    exit 1
    ;;
  2)
    echo "RHINO could not run: check the invocation and repo-config.yml" >&2
    exit 2
    ;;
esac
```

With `set -e`, capture the status in the same statement so the shell does not
exit before you read it:

```sh
status=0
rhino md mermaid validate || status=$?
```

## Run every validator and keep the worst code

A gate usually wants all the findings, not just the first validator's. Run them
all, then report the most serious outcome:

```sh
#!/bin/sh
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
  # 2 outranks 1: a run that could not happen is worse news than one that found something.
  case "$status:$worst" in
    2:*) worst=2 ;;
    1:0) worst=1 ;;
  esac
done
exit "$worst"
```

`repo-config validate` runs first on purpose. If the configuration is unusable,
every other command will exit `2` for the same reason, and one clear message
beats five copies of it.

## Tell a finding from a broken invocation without reading the code

On exit `2`, **stdout is empty**. On `0` and `1` it carries the summary. So a
caller that only has stdout can still tell them apart:

```sh
summary=$(rhino md internal-link validate 2>/dev/null)
if [ -z "$summary" ]; then
  echo "nothing ran" >&2
fi
```

## Related

- [Exit codes](../reference/exit-codes.md)
- [How to wire RHINO into your gates](./wire-rhino-into-your-gates.md)
- [What a finding means](../explanation/what-a-finding-means.md)
