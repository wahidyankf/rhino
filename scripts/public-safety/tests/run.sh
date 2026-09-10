#!/usr/bin/env bash
# ==============================================================================
# run.sh — the generic public-safety wrapper's own test suite
# ==============================================================================
# Usage: bash scripts/public-safety/tests/run.sh [case-name-fragment ...]
#
# Each case is a file in cases/ defining a `run` function, and each is named
# after a scenario in ../public-safety.feature. The harness gives every case a
# private temporary directory outside the repository and a stub scanner, so the
# suite is offline, deterministic, and fast enough to sit on a Git hook.
#
# The stub is not a bypass. The wrapper proves detection and non-disclosure with
# a credential it generates at runtime before it reads any repository material,
# so a scanner that finds nothing fails the canary and blocks. Substituting a
# scanner can make the wrapper refuse; it cannot make the wrapper pass.
#
# Set PUBLIC_SAFETY_ONLINE=1 to additionally bootstrap and canary the real
# pinned scanner. That case is reported as skipped otherwise, never as passed.
# ==============================================================================

# The assertion and fixture helpers below are called from the case files this
# harness sources, which ShellCheck cannot see from here.
# shellcheck disable=SC2329

set -uo pipefail

# `cd` must not consult CDPATH: a colon-separated entry in a contributor's
# environment would silently resolve these to some other directory.
unset CDPATH

here=$(cd -- "$(dirname -- "$0")" && pwd)
root=$(cd -- "$here/../.." && pwd)
export PUBLIC_SAFETY_ROOT="$root"
export WRAPPER="$root/scripts/public-safety/public-safety.sh"

pass=0
fail=0
skip=0
failed_names=()
skipped_names=()

# Assertions print the claim, never the data. A failing suite must not become
# the disclosure the suite exists to prevent.
assert_exit() {
	local want=$1 got=$2 what=${3:-exit code}
	if [[ "$want" != "$got" ]]; then
		echo "    expected $what $want, got $got" >&2
		return 1
	fi
}

assert_contains() {
	local needle=$1 hay=$2 what=${3:-output}
	if [[ "$hay" != *"$needle"* ]]; then
		echo "    expected $what to contain: $needle" >&2
		return 1
	fi
}

assert_absent() {
	# Deliberately does not print the needle: this is the non-disclosure
	# assertion, and naming the value on failure would disclose it.
	local needle=$1 hay=$2 what=${3:-output}
	if [[ "$hay" == *"$needle"* ]]; then
		echo "    $what disclosed a value it must never contain (${#needle} chars, not shown)" >&2
		return 1
	fi
}

skip_case() {
	printf '%s' "$1" >"$CASE_TMP/.skipped"
	return 0
}

# A stub scanner implementing the part of TruffleHog's contract the wrapper
# depends on: newline-delimited JSON records on stdout, 183 when it found
# something, 0 when it did not.
write_stub_scanner() {
	local path=$1
	cat >"$path" <<-'STUB'
		#!/usr/bin/env bash
		set -uo pipefail
		mode="${STUB_MODE:-normal}"
		dir=""
		for arg in "$@"; do
			[[ -d "$arg" ]] && dir="$arg"
		done
		case "$mode" in
		error)
			echo "stub: the scan did not complete" >&2
			exit 7
			;;
		garbage)
			printf '{"Unexpected":"shape"}\n'
			exit 183
			;;
		silent)
			exit 0
			;;
		esac
		found=0
		while IFS= read -r file; do
			line=$(grep -n -- '-----BEGIN' "$file" 2>/dev/null | head -1 | cut -d: -f1)
			[[ -z "$line" ]] && continue
			found=1
			printf '{"DetectorName":"PrivateKey","Raw":"REDACTED-BY-NOBODY","SourceMetadata":{"Data":{"Filesystem":{"file":"%s","line":%s}}}}\n' "$file" "$line"
		done < <(find "$dir" -type f 2>/dev/null | sort)
		[[ "$found" -eq 1 ]] && exit 183
		exit 0
	STUB
	chmod +x "$path"
}

# A synthetic shape set. Regular expressions only, and none of them names
# anything real: the wrapper's own set is generic by contract, and a test that
# wrote a private term into this repository would be the leak it screens for.
#
# Rows are written with printf rather than a heredoc because the format is
# whitespace-separated and a formatter that reindented a heredoc would rewrite
# the data.
write_synthetic_shapes() {
	{
		printf '# synthetic shape set, generated per test run\n'
		printf 'maintainer-path regex /Users/%s(/|$)\n' "$SYNTHETIC_NAME"
		printf 'internal-host regex synthetic-host-[0-9]{4}[.]invalid\n'
	} >"$1"
}

shopt -s nullglob
cases=("$here"/cases/*.sh)
shopt -u nullglob

if [[ ${#cases[@]} -eq 0 ]]; then
	echo "run.sh: no cases found under $here/cases" >&2
	exit 2
fi

for case_file in "${cases[@]}"; do
	name=$(basename "$case_file" .sh)
	if [[ $# -gt 0 ]]; then
		matched=0
		for want in "$@"; do
			[[ "$name" == *"$want"* ]] && matched=1
		done
		[[ $matched -eq 1 ]] || continue
	fi

	# Fresh state per case: a temporary directory outside the repository, a
	# fresh synthetic name, and a scanner cache nothing else shares.
	CASE_TMP=$(mktemp -d "${TMPDIR:-/tmp}/rhino-public-safety-test.XXXXXX")
	SYNTHETIC_NAME="zz-synthetic-$RANDOM$RANDOM"
	STUB="$CASE_TMP/stub-scanner"
	write_stub_scanner "$STUB"
	export CASE_TMP SYNTHETIC_NAME STUB
	export PUBLIC_SAFETY_CACHE="$CASE_TMP/cache"
	export PUBLIC_SAFETY_SCANNER="$STUB"

	echo "  $name"
	if (
		set -uo pipefail
		# shellcheck source=/dev/null
		source "$case_file"
		run
	); then
		if [[ -s "$CASE_TMP/.skipped" ]]; then
			skip=$((skip + 1))
			skipped_names+=("$name ($(cat "$CASE_TMP/.skipped"))")
		else
			pass=$((pass + 1))
		fi
	else
		fail=$((fail + 1))
		failed_names+=("$name")
	fi
	rm -rf "$CASE_TMP"
	unset PUBLIC_SAFETY_CACHE PUBLIC_SAFETY_SCANNER
done

echo
echo "public-safety tests: $pass passed, $fail failed, $skip skipped"
if [[ $skip -gt 0 ]]; then
	# Named rather than counted. A skip that is only a number reads as coverage
	# the suite does not have.
	printf 'skipped: %s\n' "${skipped_names[*]}"
fi
if [[ $fail -gt 0 ]]; then
	printf 'failed: %s\n' "${failed_names[*]}" >&2
	exit 1
fi
exit 0
