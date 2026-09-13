#!/usr/bin/env bash
# Formats exactly the Rust files lint-staged hands it, and nothing else.
#
# `rustfmt <path>` also rewrites every out-of-line module that file declares,
# staged or not, which is the unstaged rewrite a pre-commit formatter must not
# perform. Formatting through standard input sees only the file itself, so each
# staged file goes through stdin. `rustfmt.toml` is read from the working
# directory, which lint-staged sets to the repository root.
set -euo pipefail

for file in "$@"; do
	formatted=$(mktemp)
	if ! rustfmt --emit stdout <"$file" >"$formatted"; then
		rm -f "$formatted"
		exit 1
	fi
	# Rewrite only on a difference, so an already formatted file keeps its mtime.
	cmp -s "$formatted" "$file" || cat "$formatted" >"$file"
	rm -f "$formatted"
done
