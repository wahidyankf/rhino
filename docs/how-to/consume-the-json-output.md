# How to consume the JSON output

You want RHINO's result in a program — a dashboard, an annotation on a pull
request, a report that says which rule breaks most often. Parse the JSON, not
the text.

## One line, one object

```console
$ rhino governance word-budget validate --output json
{"schemaVersion":1,"command":"word-budget","exitCode":0,"subject":"file","inspected":2,"scanned":["AGENTS.md","README.md"],"notes":[],"violations":[]}
```

Line-delimited on purpose: concatenating several runs gives a stream `jq` can
read one object at a time.

## Filter on `kind`, never on `message`

`kind` is a stable identifier for the rule. `message` is prose, and prose gets
improved.

```sh
rhino md mermaid validate --output json \
  | jq -r '.violations[] | select(.kind == "mermaid-legibility") | "\(.path):\(.line) \(.measured)/\(.limit)"'
```

## Collect every validator into one report

```sh
for command in \
  "governance word-budget validate" \
  "governance directory-map validate" \
  "md internal-link validate" \
  "md mermaid validate" \
  "harness parity validate"
do
  # shellcheck disable=SC2086
  rhino $command --output json
done > findings.jsonl
```

Then ask questions of it:

```sh
# Which rules fire most often?
jq -r '.violations[].kind' findings.jsonl | sort | uniq -c | sort -rn

# Which files carry the most findings?
jq -r '.violations[].path' findings.jsonl | sort | uniq -c | sort -rn

# Everything, flattened, as TSV.
jq -r '.command as $c | .violations[] | [$c, .kind, .path, (.line // ""), .message] | @tsv' findings.jsonl
```

## Check that something was actually inspected

This is the check most reporting forgets. A validator whose surface matched
nothing reports zero findings and exits `0`, which looks identical to a clean
repository:

```sh
rhino governance word-budget validate --output json \
  | jq -e '.inspected > 0' > /dev/null \
  || echo "the word-budget surface matched nothing" >&2
```

`inspected` and `scanned` exist for exactly this. A surface that matched
nothing and a surface that matched everything both pass, and only the listing
tells them apart.

## Handle exit 2 before parsing

On exit `2`, **stdout is empty** and the diagnostic is on stderr. Check the
code first:

```sh
output=$(rhino md mermaid validate --output json) || status=$?
if [ "${status:-0}" = 2 ]; then
  echo "RHINO could not run" >&2
  exit 2
fi
printf '%s\n' "$output" | jq .
```

Because stdout is empty rather than holding a half-result, a parser never sees
a summary of a run that did not happen.

## Related

- [JSON output](../reference/json-output.md) for every field
- [Findings](../reference/findings.md) for every kind
- [Exit codes](../reference/exit-codes.md)
