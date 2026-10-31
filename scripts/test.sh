#!/bin/sh
set -eu

# The full gate: everything the quick gate runs, then the suites that are too
# wide for a Git hook, then a current vulnerability scan. CI runs this on every
# operating system in the release matrix, so it must stay portable: no
# GNU-only flags, no Linux-only paths, and no network use beyond the scan.

root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"

unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_OBJECT_DIRECTORY \
	GIT_ALTERNATE_OBJECT_DIRECTORIES GIT_COMMON_DIR GIT_NAMESPACE GIT_PREFIX

# Formatting, vet, the full unit suite under -race, deterministic-core coverage,
# and a binary smoke run. The -race run over every package is part of that gate.
./scripts/test-quick.sh

# The behaviour corpus is named here too: `go test ./tests/bdd` runs every
# scenario in specs/behaviours under -race and fails on any skipped scenario.
go test -race -count=1 ./tests/bdd/

# The public-safety wrapper's own suite: offline, with a stub scanner, so it
# checks the wrapper's contract on this operating system without a download.
bash scripts/public-safety/tests/run.sh

# A current vulnerability scan of the module and the standard library it links.
# The tool is pinned in go.mod, so the scan needs no install step.
go tool govulncheck ./...
