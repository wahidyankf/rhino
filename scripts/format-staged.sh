#!/usr/bin/env bash
# Format only paths the gate selected from the index or immutable review range.
set -euo pipefail

for file in "$@"; do
	# A selected path can be a deletion. It has no working-tree bytes to format,
	# and asking either formatter to read it turns a valid removal into a false
	# pre-commit failure.
	[[ -e "$file" || -L "$file" ]] || continue
	case "$file" in
	*.rs)
		bash scripts/format-staged-rust.sh "$file"
		;;
	*.md | *.json | *.yml | *.yaml)
		prettier --write "$file"
		;;
	*) ;;
	esac
done
