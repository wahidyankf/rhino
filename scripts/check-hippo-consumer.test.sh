#!/usr/bin/env bash
# Tests for check-hippo-consumer.sh. Markdown prose is wrapped at 120 columns, and a wrapped list item puts
# the line break plus its indent between two words of a required phrase. The check must match such a phrase
# in both directions: accept it when it is present across a wrap, and refuse the document when it is absent.
# A check verified only on the wrapped-accept case would pass while accepting everything.
set -uo pipefail
git_local_env_vars=$(git rev-parse --local-env-vars) || exit 1
while IFS= read -r git_local_env_var; do
	unset "$git_local_env_var"
done <<<"$git_local_env_vars"

SCRIPT="$(cd "$(dirname "$0")" && pwd)/check-hippo-consumer.sh"
pass=0
fail=0

scratch="$(mktemp -d)"
trap 'rm -rf "$scratch"' EXIT

# Build a minimal consuming repository whose resource rule is the given text.
make_fixture() {
	local root="$1" rule="$2"
	mkdir -p "$root/scripts" "$root/repo-governance/development" || exit 1
	cp "$SCRIPT" "$root/scripts/check-hippo-consumer.sh" || exit 1
	printf '{"schemaVersion":1,"source":"rhino"}\n' >"$root/hippo.identity.json"
	printf '{"schemaVersion":3,"coordination":{"tiers":{"heavy":{},"light":{},"standard":{}}}}\n' \
		>"$root/hippo.local.json.example"
	printf 'version=v0.13.4\ncommit=%s\n' "$(printf '0%.0s' {1..40})" >"$root/hippo.lock"
	printf '#!/bin/sh\n# HIPPO_DEFAULT_IDENTITY\n# --path-format=absolute --git-common-dir\n' >"$root/hippo"
	printf '%s\n' '{repository location}/worktrees/<name>/' >"$root/AGENTS.md"
	printf '%s\n' "$rule" >"$root/repo-governance/development/resource-aware-development.md"
	GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null git init -q "$root" 2>/dev/null || exit 1
}

# Every required phrase on one line, then the same phrases wrapped the way a 120-column reflow wraps them.
flat_rule=$(
	cat <<'RULE'
`124` `125` never-started
`hippo.coordination.protocol-mismatch` is never retried: drain first.
Hippo repository at the commit in `hippo.lock`; kept 30 days.
RULE
)
wrapped_rule=$(
	cat <<'RULE'
`124` `125` never-started
- **`125`** `hippo.coordination.protocol-mismatch` is never
  retried: drain first. The Hippo repository at the commit in
  `hippo.lock`; kept
    30
  days.
RULE
)
missing_rule=$(
	cat <<'RULE'
`124` `125` never-started
`hippo.coordination.protocol-mismatch` is retried once: drain first.
Hippo repository at the commit in `hippo.lock`; kept 30 days.
RULE
)

expect() {
	local label="$1" rule="$2" want="$3" root code
	root="$scratch/case-$((pass + fail))"
	make_fixture "$root" "$rule"
	"$root/scripts/check-hippo-consumer.sh" >/dev/null 2>&1
	code=$?
	if { [ "$want" = "accepts" ] && [ "$code" -eq 0 ]; } || { [ "$want" = "refuses" ] && [ "$code" -ne 0 ]; }; then
		pass=$((pass + 1))
	else
		echo "FAIL: $label — expected the check to ${want%s}, exit code was $code"
		fail=$((fail + 1))
	fi
}

expect "phrases on single lines" "$flat_rule" accepts
expect "phrases wrapped across lines with indentation" "$wrapped_rule" accepts
expect "a required phrase absent" "$missing_rule" refuses

echo "passed=$pass failed=$fail"
[ "$fail" -eq 0 ]
