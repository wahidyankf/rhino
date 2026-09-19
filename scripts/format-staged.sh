#!/usr/bin/env bash
# Format only paths the gate selected from the index or immutable review range.
set -euo pipefail

for file in "$@"; do
	# A selected path can be a deletion. It has no working-tree bytes to format,
	# and asking either formatter to read it turns a valid removal into a false
	# pre-commit failure.
	[[ -e "$file" || -L "$file" ]] || continue
	case "$file" in
	.claude/agents/catalog.json | .claude/agents/provenance.json | .claude/skills/catalog.json | .claude/skills/provenance.json | .codex/agents/catalog.json | .codex/agents/provenance.json | .opencode/agents/catalog.json | .opencode/agents/provenance.json)
		# These manifests are byte-for-byte output of `harness adapters generate`.
		# Reformatting them would make a generated adapter stale immediately.
		continue
		;;
	*.rs)
		bash scripts/format-staged-rust.sh "$file"
		;;
	*.md | *.json | *.yml | *.yaml)
		prettier --write "$file"
		;;
	*) ;;
	esac
done
