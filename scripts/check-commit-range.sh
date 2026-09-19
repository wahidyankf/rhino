#!/usr/bin/env bash
# Commitlint receives each commit in the immutable typed range, preserving the
# per-commit contract that a concatenated message stream cannot represent.
set -euo pipefail

[[ $# -eq 2 ]] || {
	printf '%s\n' 'check-commit-range.sh requires a base and head revision' >&2
	exit 2
}

exec npm exec -- commitlint --from "$1" --to "$2" --verbose
