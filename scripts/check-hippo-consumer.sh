#!/usr/bin/env bash
# Enforce Rhino's Hippo consumer, tier, and worktree contracts.
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

jq -e '.schemaVersion == 1 and .source == "rhino"' "$repo_root/hippo.identity.json" >/dev/null
jq -e '.schemaVersion == 3 and (.coordination.tiers | keys) == ["heavy", "light", "standard"]' \
  "$repo_root/hippo.local.json.example" >/dev/null
grep -Eq '^version=v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$' "$repo_root/hippo.lock"
grep -Eq '^commit=[0-9a-f]{40}$' "$repo_root/hippo.lock"
grep -Fq 'HIPPO_DEFAULT_IDENTITY' "$repo_root/hippo"
grep -Fq -- '--path-format=absolute --git-common-dir' "$repo_root/hippo"
# Markdown prose is wrapped at 120 columns, so a required phrase may span a
# line break; match it against the file with every newline read as a space.
has_phrase() {
  tr '\n' ' ' <"$1" | grep -Fq -- "$2"
}

resource_rule="$repo_root/repo-governance/development/resource-aware-development.md"
has_phrase "$resource_rule" '`124`'
has_phrase "$resource_rule" 'never-started'
has_phrase "$resource_rule" '`125`'
has_phrase "$resource_rule" 'never retried'
has_phrase "$resource_rule" 'protocol-mismatch'
has_phrase "$resource_rule" 'Hippo repository at the commit in `hippo.lock`'
has_phrase "$resource_rule" '30 days'
has_phrase "$repo_root/AGENTS.md" '{repository location}/worktrees/<name>/'

tier_findings=$(git -C "$repo_root" grep -n -E \
  '\./hippo run --class (ephemeral|service|transactional)' -- \
  . ':(exclude)plans/done/**' ':(exclude)scripts/check-hippo-consumer.sh' | \
  grep -v -- '--resource-tier' || true)
if [ -n "$tier_findings" ]; then
  printf 'FAIL: HIPPO commands missing --resource-tier:\n%s\n' "$tier_findings" >&2
  exit 1
fi

nested_findings=$(git -C "$repo_root" grep -n -E \
  '\./hippo run .*-- (rtk )?npm (run|test)( |$)' -- \
  . ':(exclude)plans/done/**' ':(exclude)scripts/check-hippo-consumer.sh' || true)
if [ -n "$nested_findings" ]; then
  printf 'FAIL: HIPPO commands double-guard package scripts:\n%s\n' "$nested_findings" >&2
  exit 1
fi

printf '%s\n' 'PASS: Rhino Hippo consumer policy is aligned'
