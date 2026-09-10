#!/usr/bin/env bash
# ==============================================================================
# public-safety.sh — the generic gate every public repository runs first
# ==============================================================================
# Usage:
#   public-safety.sh --surface <surface> [--text <string>]... [--file <path>]...
#                    [--shapes <path>]
#   public-safety.sh --print-pin
#   public-safety.sh --print-platform
#
#   --surface  baseline | diff | commit | ref | pull-request | release | logs
#   --text     an outbound string: a commit message, a branch name, a PR body
#   --file     a file whose contents are outbound
#   --shapes   shape set to screen against (default: shapes.txt beside this)
#
# Exit codes are the whole interface:
#
#   0  clean       nothing prohibited was found; publication may proceed
#   1  blocked     something prohibited was found; publication must not proceed
#   2  scan error  the scan could not be trusted; publication must not proceed
#
# There is no allowlist, suppression, or bypass. 1 and 2 are both refusals; they
# differ only in whether we know what is wrong. A scan that cannot run is not a
# scan that passed.
#
# This is the generic layer. It screens credential patterns and generic private
# metadata shapes, and it holds no workspace-private term: it ships in public
# repositories, so a literal here would be the disclosure it exists to stop. The
# confidential layer that does hold identifiers lives in the command center and
# is never copied into a public checkout.
#
# Nothing this script prints ever contains a matched value, a row of the shape
# set, or raw scanner output. A diagnostic carries a detector, a screened path,
# a line number, and a status -- that is the entire vocabulary.
# ==============================================================================

# No `-e`. This script's whole interface is its exit code, and it depends on
# commands that return non-zero as part of normal operation: `grep -q` finding
# nothing, and TruffleHog returning 183 when it finds something. Under `-e` the
# first of those would abort with status 1 -- which this script's own contract
# reads as "blocked" -- turning a clean scan into a false refusal. Every fallible
# command is checked explicitly instead.
set -uo pipefail

unset CDPATH

readonly TRUFFLEHOG_VERSION="3.97.1"

# The GitHub release-asset digests for v3.97.1. No other source is approved, and
# a mismatch is a scan error rather than a warning.
trufflehog_digest() {
	case "$1" in
	darwin_amd64) echo "1515710bb16be5653ca9986c27ecd1a0e7536fc6e53ad46f7100992692f6a05f" ;;
	darwin_arm64) echo "1af86cf30c1cc5c1735ec6af9292b399ec9bed3ff1b30be13fcbfd4a30ab449a" ;;
	linux_amd64) echo "f863ea3a8d786f7d097870496c977944cce7372a2fe1e56707d965016e543ece" ;;
	linux_arm64) echo "57bfcc0988aae3f2ef97e74abe1138cf37a8fbd84dd26299062c77a6a6b125dd" ;;
	*) return 1 ;;
	esac
}

readonly PLATFORMS="darwin_amd64 darwin_arm64 linux_amd64 linux_arm64"
readonly SURFACES=" baseline diff commit ref pull-request release logs "
readonly SHAPE_CLASSES=" maintainer-path internal-host private-address private-topology credential-assignment "

here=$(cd -- "$(dirname -- "$0")" && pwd)

surface=""
shapes_file="$here/shapes.txt"
declare -a input_labels=()
declare -a input_files=()

# ------------------------------------------------------------------------------
# Output. Two channels, both sanitized by construction.
# ------------------------------------------------------------------------------

emit_finding() {
	# emit_finding <detector> <screened-path> <line>
	#
	# Findings land in a file rather than a counter. Some producers run inside a
	# command substitution, and a subshell's increment would be lost on return.
	printf '[public-safety] finding %s %s:%s\n' "$1" "$2" "$3" >>"$FINDINGS_FILE"
}

scan_error() {
	# A scan error blocks. It names what could not be trusted, never the value.
	printf '[public-safety] blocked scan-error %s\n' "$*" >&2
	exit 2
}

digest_of() {
	if command -v shasum >/dev/null 2>&1; then
		shasum -a 256 "$1" | cut -d' ' -f1
	elif command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | cut -d' ' -f1
	else
		return 1
	fi
}

platform() {
	# An override exists so the closed matrix can be exercised without a second
	# machine. It can only ever narrow what runs: a platform outside the matrix
	# is refused, and one inside it still has to match its approved digest.
	if [[ -n "${PUBLIC_SAFETY_PLATFORM:-}" ]]; then
		printf '%s' "$PUBLIC_SAFETY_PLATFORM"
		return 0
	fi
	local os arch
	case "$(uname -s)" in
	Darwin) os=darwin ;;
	Linux) os=linux ;;
	*) return 1 ;;
	esac
	case "$(uname -m)" in
	x86_64 | amd64) arch=amd64 ;;
	arm64 | aarch64) arch=arm64 ;;
	*) return 1 ;;
	esac
	printf '%s_%s' "$os" "$arch"
}

# ------------------------------------------------------------------------------
# Reports that answer without scanning. Both are read by this wrapper's own case
# suite, which is how the pin is checked without a download.
# ------------------------------------------------------------------------------

if [[ "${1:-}" == "--print-pin" ]]; then
	printf 'trufflehog %s\n' "$TRUFFLEHOG_VERSION"
	for plat in $PLATFORMS; do
		printf '%s %s\n' "$plat" "$(trufflehog_digest "$plat")"
	done
	exit 0
fi

if [[ "${1:-}" == "--print-platform" ]]; then
	plat=$(platform) || {
		printf 'unsupported platform\n' >&2
		exit 2
	}
	printf '%s\n' "$plat"
	exit 0
fi

# ------------------------------------------------------------------------------
# Workspace. Outside the repository, removed on every exit path.
# ------------------------------------------------------------------------------

WORK=$(mktemp -d "${TMPDIR:-/tmp}/rhino-public-safety.XXXXXX") || scan_error "cannot create a working directory"
# Invoked by the trap below, which ShellCheck does not read as a call.
# shellcheck disable=SC2329
cleanup() { rm -rf "$WORK"; }
trap cleanup EXIT INT TERM

FINDINGS_FILE="$WORK/findings.txt"
: >"$FINDINGS_FILE"

# ------------------------------------------------------------------------------
# Arguments
# ------------------------------------------------------------------------------

text_count=0
while [[ $# -gt 0 ]]; do
	case "$1" in
	--surface)
		[[ $# -ge 2 ]] || scan_error "--surface needs a value"
		surface="$2"
		shift 2
		;;
	--shapes)
		[[ $# -ge 2 ]] || scan_error "--shapes needs a value"
		shapes_file="$2"
		shift 2
		;;
	--text)
		[[ $# -ge 2 ]] || scan_error "--text needs a value"
		text_count=$((text_count + 1))
		printf '%s\n' "$2" >"$WORK/text-$text_count"
		input_labels+=("<${surface:-input}-text-$text_count>")
		input_files+=("$WORK/text-$text_count")
		shift 2
		;;
	--file)
		[[ $# -ge 2 ]] || scan_error "--file needs a value"
		[[ -r "$2" ]] || scan_error "input file is not readable"
		input_labels+=("$2")
		input_files+=("$2")
		shift 2
		;;
	*) scan_error "unrecognized argument" ;;
	esac
done

[[ -n "$surface" ]] || scan_error "--surface is required"
[[ "$SURFACES" == *" $surface "* ]] || scan_error "unsupported surface"

# Two surfaces know how to find their own inputs, so a hook can name the surface
# and nothing else. Explicit inputs win: a caller that said what to screen is
# not asking this script to guess.
derived=0
if [[ ${#input_files[@]} -eq 0 ]]; then
	derived=1
	case "$surface" in
	baseline)
		while IFS= read -r tracked; do
			[[ -r "$tracked" ]] || continue
			input_labels+=("$tracked")
			input_files+=("$tracked")
		done < <(git ls-files 2>/dev/null)
		;;
	diff)
		while IFS= read -r staged; do
			[[ -r "$staged" ]] || continue
			input_labels+=("$staged")
			input_files+=("$staged")
		done < <(git diff --cached --name-only --diff-filter=ACMR 2>/dev/null)
		;;
	esac
fi

# A caller that named no inputs on a surface that does not derive them has
# asked for a scan of nothing, and answering "clean" would pass on every surface
# that forgot to hand anything over. A derived surface with an empty set is
# different: it looked, and there is nothing outbound to look at.
if [[ ${#input_files[@]} -eq 0 ]]; then
	[[ "$derived" -eq 1 && ("$surface" == baseline || "$surface" == diff) ]] ||
		scan_error "no outbound input was given"
	printf '[public-safety] %s: clean, nothing outbound\n' "$surface"
	exit 0
fi

# ------------------------------------------------------------------------------
# Shape set. Validated before it is trusted; every failure is exit 2, and no
# refusal ever quotes a row.
# ------------------------------------------------------------------------------

# A shape set carrying a credential would publish it to anyone who clones the
# repository, so a credential-shaped pattern is malformed rather than unwise.
reject_credential_shaped_pattern() {
	local value=$1 lineno=$2

	if printf '%s' "$value" |
		grep -qE '(ghp_|gho_|ghu_|ghs_|ghr_|github_pat_|xox[baprs]-|AKIA|ASIA|AIza|glpat-|dop_v1_|-----BEGIN)'; then
		scan_error "malformed shape set at line $lineno: the pattern carries a known credential prefix"
	fi
	if printf '%s' "$value" | grep -qE '^[a-zA-Z][a-zA-Z0-9+.-]*://[^[:space:]/@]+:[^[:space:]/@]+@'; then
		scan_error "malformed shape set at line $lineno: the pattern looks like a connection string"
	fi
}

[[ -e "$shapes_file" ]] || scan_error "shape set is absent: publication is ineligible"
[[ -r "$shapes_file" ]] || scan_error "shape set is unreadable: publication is ineligible"

normalized="$WORK/shapes.tsv"
: >"$normalized"

lineno=0
while IFS= read -r raw || [[ -n "$raw" ]]; do
	lineno=$((lineno + 1))
	[[ -z "${raw//[[:space:]]/}" ]] && continue
	[[ "${raw#"${raw%%[![:space:]]*}"}" == \#* ]] && continue

	# Class, kind, then the rest of the line: a pattern may contain spaces, and
	# splitting it on them would silently truncate it.
	read -r cls kind value <<<"$raw"
	[[ -n "${cls:-}" && -n "${kind:-}" && -n "${value:-}" ]] ||
		scan_error "malformed shape set at line $lineno: a class, a kind, and a pattern are required"

	# Kind before class, because a literal row is the more specific fault and
	# naming the class first would hide why the row cannot be here at all.
	[[ "$kind" == regex ]] ||
		scan_error "malformed shape set at line $lineno: a public repository's shape set states shapes only, and a literal row is the private term itself"
	[[ "$SHAPE_CLASSES" == *" $cls "* ]] || scan_error "malformed shape set at line $lineno: unknown class"

	printf '' | grep -qE -- "$value" 2>/dev/null
	[[ $? -le 1 ]] || scan_error "malformed shape set at line $lineno: the pattern is not a usable regular expression"

	reject_credential_shaped_pattern "$value" "$lineno"

	printf '%s\t%s\n' "$cls" "$value" >>"$normalized"
done <"$shapes_file"

[[ -s "$normalized" ]] ||
	scan_error "shape set is empty: an empty set cannot be distinguished from a set that found nothing, so publication is ineligible"

# ------------------------------------------------------------------------------
# Shape screening. Paths are screened too, and a prohibited path is reported as
# `<blocked-path>` rather than printed.
# ------------------------------------------------------------------------------

screen_path() {
	local candidate=$1 cls value
	while IFS=$'\t' read -r cls value; do
		if printf '%s' "$candidate" | grep -qE -- "$value"; then
			printf '<blocked-path>'
			return
		fi
	done <"$normalized"
	printf '%s' "$candidate"
}

screen_shapes() {
	# screen_shapes <label> <file>
	local label=$1 file=$2 cls value hits path
	path=$(screen_path "$label")
	while IFS=$'\t' read -r cls value; do
		hits=$(grep -nE -- "$value" "$file" 2>/dev/null | cut -d: -f1)
		[[ -z "$hits" ]] && continue
		while IFS= read -r n; do
			[[ -n "$n" ]] && emit_finding "$cls" "$path" "$n"
		done <<<"$hits"
	done <"$normalized"
}

# ------------------------------------------------------------------------------
# The scanner. Bootstrapped once into a cache outside the repository, verified
# by digest before extraction, stamped with what was extracted, and run offline.
# ------------------------------------------------------------------------------

resolve_scanner() {
	# An alternative scanner may be named. It is not a bypass: the canary below
	# proves detection and non-disclosure against a credential generated at
	# runtime before any repository material is read, so substituting a scanner
	# can make this wrapper refuse and cannot make it pass.
	if [[ -n "${PUBLIC_SAFETY_SCANNER:-}" ]]; then
		[[ -x "$PUBLIC_SAFETY_SCANNER" ]] || scan_error "the named scanner is not executable"
		TRUFFLEHOG_BIN="$PUBLIC_SAFETY_SCANNER"
		return 0
	fi

	local plat want cache dir tarball actual stamp
	plat=$(platform) || scan_error "unsupported platform: refusing to scan"
	want=$(trufflehog_digest "$plat") || scan_error "unsupported platform: no approved digest"

	cache="${PUBLIC_SAFETY_CACHE:-${XDG_CACHE_HOME:-$HOME/.cache}/ose-public-safety}"
	dir="$cache/trufflehog/$TRUFFLEHOG_VERSION/$plat"
	TRUFFLEHOG_BIN="$dir/trufflehog"
	stamp="$dir/trufflehog.sha256"

	# A cache hit is re-verified rather than trusted. The tarball digest is
	# checked once, at download; the stamp carries that verification forward, so
	# a binary that drifted from it drifted after it was approved.
	if [[ -x "$TRUFFLEHOG_BIN" ]]; then
		# Fail closed rather than re-download. A hook may run with no network,
		# and a wrapper that reached for one when its cache looked wrong would
		# be least available exactly when it is most needed. The path is not
		# printed: a cache under a home directory is itself a maintainer path.
		[[ -r "$stamp" ]] ||
			scan_error "the cached scanner carries no record of what was extracted: remove the scanner cache directory and run again"
		actual=$(digest_of "$TRUFFLEHOG_BIN") || scan_error "cannot digest the cached scanner"
		[[ "$actual" == "$(cat "$stamp")" ]] ||
			scan_error "the cached scanner does not match what was extracted: refusing to run it"
		return 0
	fi

	mkdir -p "$dir" || scan_error "cannot create the scanner cache"
	tarball="$WORK/trufflehog.tar.gz"

	# The only network access in this script, and only when the exact pinned
	# version is absent from the cache.
	curl -sSfL --max-time 180 -o "$tarball" \
		"https://github.com/trufflesecurity/trufflehog/releases/download/v${TRUFFLEHOG_VERSION}/trufflehog_${TRUFFLEHOG_VERSION}_${plat}.tar.gz" ||
		scan_error "cannot download the pinned scanner"

	actual=$(digest_of "$tarball") || scan_error "cannot digest the downloaded scanner"
	[[ "$actual" == "$want" ]] || scan_error "scanner digest mismatch: refusing to extract"

	tar -xzf "$tarball" -C "$dir" trufflehog || scan_error "cannot extract the pinned scanner"
	chmod +x "$TRUFFLEHOG_BIN" || scan_error "cannot make the scanner executable"
	[[ -x "$TRUFFLEHOG_BIN" ]] || scan_error "the scanner is not executable after extraction"
	digest_of "$TRUFFLEHOG_BIN" >"$stamp" || scan_error "cannot record what was extracted"
}

# The label a staged input was made from. The canary stages nothing, so a path
# with no entry is returned as read.
named() {
	local candidate=$1 index
	index=${candidate##*/input-}
	if [[ "$index" =~ ^[0-9]+$ ]]; then
		printf '%s' "${input_labels[$index]:-$candidate}"
	else
		printf '%s' "$candidate"
	fi
}

# Raw stdout flows only through this pipe into jq. No raw temporary file, no
# artifact, no debug log. jq emits three fields; nothing else survives.
credential_scan() {
	# credential_scan <directory>
	local dir=$1 raw_err rc records detector path n
	raw_err="$WORK/scanner.err"

	records=$("$TRUFFLEHOG_BIN" filesystem "$dir" \
		--json --no-verification --no-update --fail --fail-on-scan-errors \
		2>"$raw_err" |
		jq -r -c 'select(type == "object" and has("DetectorName"))
		          | [ .DetectorName,
		              (.SourceMetadata.Data.Filesystem.file // "<unknown>"),
		              (.SourceMetadata.Data.Filesystem.line // 0) ]
		          | @tsv' 2>"$WORK/jq.err")
	rc=$?

	# stderr is read for classification and discarded without display.
	if [[ -s "$WORK/jq.err" ]]; then
		: >"$WORK/jq.err"
		scan_error "scanner output did not match the expected record shape"
	fi
	: >"$raw_err"

	# 0 = nothing found, 183 = findings (the --fail contract). Anything else is
	# a scan error, which blocks.
	if [[ "$rc" -ne 0 && "$rc" -ne 183 ]]; then
		scan_error "the credential scan did not complete"
	fi

	# A run that exits on findings and emits no record it can name is a protocol
	# nobody declared, not a clean scan.
	if [[ "$rc" -eq 183 && -z "${records//[[:space:]]/}" ]]; then
		scan_error "the scanner reported findings it did not describe"
	fi

	while IFS=$'\t' read -r detector path n; do
		[[ -z "${detector:-}" ]] && continue
		# The scanner reports where it read, which is a staging copy under a
		# temporary directory. Say what the caller called it instead: a
		# diagnostic naming a path nobody recognises tells them nothing, and the
		# temporary path is a machine detail this repository does not publish.
		path=$(named "$path")
		emit_finding "$detector" "$(screen_path "$path")" "${n:-0}"
	done <<<"$records"
}

# ------------------------------------------------------------------------------
# Canary. Proves detection AND non-disclosure before any real material is read.
# ------------------------------------------------------------------------------

run_canary() {
	local dir value out detected real_findings scan_rc

	dir=$(mktemp -d "${TMPDIR:-/tmp}/rhino-public-safety-canary.XXXXXX") ||
		scan_error "cannot create the canary directory"

	# Generated here, never committed, and never printed. A real key rather than
	# a token-shaped string: the token detectors validate a checksum, so a random
	# `ghp_...` would prove nothing about whether detection works.
	openssl genrsa -out "$dir/canary.pem" 2048 2>/dev/null ||
		scan_error "cannot generate the canary key"
	value=$(sed -n '3p' "$dir/canary.pem")
	[[ -n "$value" ]] || scan_error "the canary key is not in the expected form"

	# The canary keeps its own ledger so a proof never becomes a real finding.
	real_findings="$FINDINGS_FILE"
	FINDINGS_FILE="$WORK/canary-findings.txt"
	: >"$FINDINGS_FILE"

	# `scan_error` inside a command substitution exits only the subshell, so the
	# substitution's own status is what says whether the canary scan survived.
	out=$(credential_scan "$dir" 2>&1)
	scan_rc=$?
	detected=$(wc -l <"$FINDINGS_FILE" | tr -d ' ')

	if [[ "$scan_rc" -ne 0 ]]; then
		: >"$FINDINGS_FILE"
		rm -rf "$dir"
		FINDINGS_FILE="$real_findings"
		scan_error "the canary scan could not complete: the scanner cannot be trusted"
	fi

	if [[ "$out$(cat "$FINDINGS_FILE")" == *"$value"* ]]; then
		: >"$FINDINGS_FILE"
		rm -rf "$dir"
		FINDINGS_FILE="$real_findings"
		scan_error "the canary value reached the output: the sanitizer cannot be trusted"
	fi

	: >"$FINDINGS_FILE"
	FINDINGS_FILE="$real_findings"
	rm -rf "$dir"

	[[ "$detected" -ge 1 ]] || scan_error "the canary produced no detection: the scanner cannot be trusted"
	[[ -d "$dir" ]] && scan_error "the canary directory survived"
	return 0
}

# ------------------------------------------------------------------------------
# Run
# ------------------------------------------------------------------------------

resolve_scanner
run_canary

for i in "${!input_files[@]}"; do
	# The name is outbound too: a path can carry a private identifier that its
	# contents never mention.
	printf '%s\n' "${input_labels[$i]}" >"$WORK/name-$i"
	screen_shapes "${input_labels[$i]}" "$WORK/name-$i"
	screen_shapes "${input_labels[$i]}" "${input_files[$i]}"
done

# Every typed input is copied under one directory so the credential scan sees
# text and files alike, under names that carry no private material.
scan_dir="$WORK/inputs"
mkdir -p "$scan_dir"
for i in "${!input_files[@]}"; do
	cp "${input_files[$i]}" "$scan_dir/input-$i" 2>/dev/null ||
		scan_error "cannot stage an outbound input for scanning"
done
credential_scan "$scan_dir"

sort -u "$FINDINGS_FILE" >"$WORK/findings-unique.txt" && mv "$WORK/findings-unique.txt" "$FINDINGS_FILE"
findings=$(wc -l <"$FINDINGS_FILE" | tr -d ' ')
if [[ "$findings" -gt 0 ]]; then
	cat "$FINDINGS_FILE"
	printf '[public-safety] %s: blocked, %s finding(s); publication must not proceed\n' "$surface" "$findings" >&2
	exit 1
fi

printf '[public-safety] %s: clean\n' "$surface"
exit 0
