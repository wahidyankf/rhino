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
resource_rule="$repo_root/repo-governance/development/resource-aware-development.md"
grep -Fq '`75`' "$resource_rule"
grep -Fq 'never-started' "$resource_rule"
grep -Fq '`76`' "$resource_rule"
grep -Fq 'never retry' "$resource_rule"
grep -Fq 'legacy client without distinct exit `76`' "$resource_rule"
grep -Fq 'Hippo repository at the commit in `hippo.lock`' "$resource_rule"
grep -Fq '30 days' "$resource_rule"
grep -Fq '{repository location}/worktrees/<name>/' "$repo_root/AGENTS.md"

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
