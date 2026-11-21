#!/bin/sh
set -eu

# The integration gate: the tagged suites (-tags integration) over every
# authored package, with -race, as the test:integration target. The suites may
# use the real host (git, symlinks, processes) but never the network (C21).
#
# Network denial: the run goes into a fresh network namespace when the host
# allows one. A host that refuses the namespace prints the refusal and the
# suites run without it; the denial is never silently skipped.

root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"

unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_OBJECT_DIRECTORY \
	GIT_ALTERNATE_OBJECT_DIRECTORIES GIT_COMMON_DIR GIT_NAMESPACE GIT_PREFIX

if probe=$(unshare -rn true 2>&1); then
	deny="unshare -rn"
	echo "network denial: integration suites run under: unshare -rn"
else
	deny=""
	echo "network denial: unavailable here, unshare -rn refused: ${probe}" >&2
	echo "network denial: integration suites run without it" >&2
fi

echo "+ ${deny:+$deny }go test -race -count=1 -tags integration ./cmd/... ./internal/... ./scripts/... ./tests/bdd/..."
${deny:+$deny} go test -race -count=1 -tags integration ./cmd/... ./internal/... ./scripts/... ./tests/bdd/...
