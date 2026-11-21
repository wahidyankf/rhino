#!/bin/sh
set -eu

# The quick gate for the Go port: formatting, compilation, strict vet, unit
# tests with -race, unit coverage, and a binary smoke run.
# Documentation hygiene is dispatched elsewhere, as in the hippo template.
#
# Coverage floor: the repository floor is 99% over every authored production
# package: cmd, internal, and scripts/release-archive. No package is excluded;
# only generated and test-support code sits outside the measured set. The gate
# below pins that floor, so any regression below 99% fails the quick gate.

root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"

unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_OBJECT_DIRECTORY \
	GIT_ALTERNATE_OBJECT_DIRECTORIES GIT_COMMON_DIR GIT_NAMESPACE GIT_PREFIX

if [ -n "$(gofmt -l cmd internal)" ]; then
	echo "unformatted Go files:" >&2
	gofmt -l cmd internal >&2
	exit 1
fi

go vet ./...

# Lint is a gate. The pinned golangci-lint (go.mod tool directive) fails on
# any finding, so warning severity and above fail this gate. Its forbidigo and
# depguard rules keep the unit tests hermetic (C21); integration files carry the
# integration build tag, which this default run omits.
go tool golangci-lint run ./...

# Network denial (C21). The unit suite runs in a fresh network namespace when
# the host allows one. A host that refuses the namespace prints why, and the
# suite runs without it; the static network ban in the lint rules still applies.
if probe=$(unshare -rn true 2>&1); then
	deny="unshare -rn"
	echo "network denial: unit suite runs under: unshare -rn"
else
	deny=""
	echo "network denial: unavailable here, unshare -rn refused: ${probe}" >&2
	echo "network denial: unit suite runs without it; the lint rules still ban network packages" >&2
fi

# The unit suite runs with -race: a data race is a finding in this tool, not a
# flake to retry. The integration suite (tests/bdd) and the end-to-end suite
# (tests/e2e) are not part of this gate; they are the test:integration and
# test:e2e targets, so this gate means quick and nothing slower.
echo "+ ${deny:+$deny }go test -race -count=1 ./cmd/... ./internal/... ./scripts/release-archive"
${deny:+$deny} go test -race -count=1 ./cmd/... ./internal/... ./scripts/release-archive

mkdir -p coverage
${deny:+$deny} go test -count=1 \
	-coverpkg=./cmd/...,./internal/...,./scripts/release-archive \
	-coverprofile=coverage/unit.raw.out \
	./cmd/... ./internal/... ./scripts/release-archive >/dev/null

# Every test binary records each measured block, so the raw profile repeats a
# block once per binary. Keep one line per block, with the highest count any
# binary recorded, so a block counts once and is covered if any run covered it.
awk 'NR == 1 { print; next }
{
	key = $1 " " $2
	if (!(key in count) || $3 + 0 > count[key] + 0) count[key] = $3
	if (!(key in order)) { order[key] = ++n; keys[n] = key }
}
END {
	for (i = 1; i <= n; i++) print keys[i] " " count[keys[i]]
}' coverage/unit.raw.out >coverage/unit.out

# The floor is compared on the exact percentage. go tool cover rounds its total
# to one decimal, so a 98.99% total would print as 99.0% and pass a whole-number
# comparison; the floor is therefore computed here from the statement counts.
total=$(awk 'NR > 1 { statements += $2; if ($3 > 0) covered += $2 }
END { printf "%.4f", 100 * covered / statements }' coverage/unit.out)
minimum=99
if awk -v total="$total" -v minimum="$minimum" 'BEGIN { exit !(total + 0 < minimum + 0) }'; then
	echo "unit coverage ${total}% is below the pinned ${minimum}%" >&2
	exit 1
fi
echo "unit coverage ${total}% (pinned floor ${minimum}%, repository floor 99%)"

# Binary smoke: the stamped binary answers the public contract on a fixture.
smoke=$(mktemp -d "${TMPDIR:-/tmp}/rhino-smoke.XXXXXX")
trap 'rm -rf -- "$smoke"' EXIT HUP INT TERM
go build -o "$smoke/rhino" ./cmd/rhino
mkdir -p "$smoke/repo/config"
printf 'schema: rhino/repo-config/v2\npolicies:\n  conventions:\n    emoji:\n      prohibited:\n        - glob: "**/*.json"\n' >"$smoke/repo/repo-config.yml"
printf '{"emoji": "\360\237\246\217"}\n' >"$smoke/repo/config/a.json"
if (cd "$smoke/repo" && "$smoke/rhino" version --json | grep -q '"schemaVersion":1'); then
	:
else
	echo "version --json shape broken" >&2
	exit 1
fi
if (cd "$smoke/repo" && "$smoke/rhino" convention emoji validate 2>/dev/null); then
	echo "emoji finding was not reported" >&2
	exit 1
fi
trap - EXIT HUP INT TERM
rm -rf -- "$smoke"

# The static behaviour check runs last, as test:quick requires (C14). It reads
# the corpus and its bindings, and runs no test.
go run ./cmd/rhino behaviour coverage validate
