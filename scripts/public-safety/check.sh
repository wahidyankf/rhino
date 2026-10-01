#!/usr/bin/env bash
# ==============================================================================
# check.sh — this repository's public-safety gate
# ==============================================================================
# Usage: RHINO_GATE_SURFACE=<surface> scripts/public-safety/check.sh
#
#   pre-commit   no arguments; the staged tree is the subject
#   pre-push     no arguments; the tracked baseline is the subject
#   pull-request no arguments; the checked-out review tree is the subject
#
#   pre-push and pull-request with PUBLIC_SAFETY_BASE and PUBLIC_SAFETY_HEAD
#                one declared range, screened commit by commit
#
# Rhino's product-scoped marker is set by the typed lifecycle dispatcher.
# Neither this wrapper nor its callers infer a surface from hook arguments,
# the current directory, or remote reachability.
#
# This file decides *what is outbound* at each surface. `public-safety.sh`
# decides whether any of it is prohibited. Keeping those apart is what lets the
# leaf be tested against synthetic inputs with no repository at all, which is
# what `tests/run.sh` does.
#
# Nothing new is screened here. Every line below is a surface one of the three
# Git hooks already ran by hand; what changed is who decides the order, and the
# answer is now `repo-config.yml` rather than three shell files.
#
# Exit codes pass through from the leaf: 0 clean, 1 blocked, 2 scan error.
# ==============================================================================

set -uo pipefail

here=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
root=$(git -C "$here" rev-parse --show-toplevel 2>/dev/null) || {
	printf '[public-safety] blocked scan-error not inside a Git repository\n' >&2
	exit 2
}
leaf="$here/public-safety.sh"
[[ -x "$leaf" ]] || {
	printf '[public-safety] blocked scan-error the leaf wrapper is missing or not executable\n' >&2
	exit 2
}

surface="${RHINO_GATE_SURFACE:-}"
case "$surface" in
pre-commit | pre-push | pull-request) ;;
"")
	printf '[public-safety] blocked scan-error RHINO_GATE_SURFACE is unset\n' >&2
	exit 2
	;;
*)
	printf '[public-safety] blocked scan-error RHINO_GATE_SURFACE is not a known surface\n' >&2
	exit 2
	;;
esac

# A range is declared whole or not at all. Half of one would quietly fall back
# to a different subject, and a narrower screen than the caller asked for is a
# pass nobody granted.
range=0
if [[ -n "${PUBLIC_SAFETY_BASE:-}" || -n "${PUBLIC_SAFETY_HEAD:-}" ]]; then
	[[ -n "${PUBLIC_SAFETY_BASE:-}" && -n "${PUBLIC_SAFETY_HEAD:-}" ]] || {
		printf '[public-safety] blocked scan-error a range needs both PUBLIC_SAFETY_BASE and PUBLIC_SAFETY_HEAD\n' >&2
		exit 2
	}
	[[ "$surface" != pre-commit ]] || {
		printf '[public-safety] blocked scan-error pre-commit screens the index, not a range\n' >&2
		exit 2
	}
	range=1
fi

cd "$root" || exit 2

current_ref() {
	git symbolic-ref --quiet --short HEAD 2>/dev/null || git rev-parse --short HEAD
}

range_error() {
	printf '[public-safety] blocked scan-error %s\n' "$1" >&2
	exit 2
}

added_lines() {
	# added_lines: reads one file's zero-context diff on standard input and
	# prints the lines it adds, each at the line number it occupies after the
	# change with blank lines between, so a finding names a real location in
	# that commit's file. Exits 3 for a binary change, whose added content a
	# text diff cannot show.
	awk '
		/^Binary files / { binary = 1; exit }
		/^@@ / {
			header = $0
			sub(/^@@ -[0-9,]+ \+/, "", header)
			split(header, at, /[, ]/)
			line = at[1] + 0
			in_hunk = 1
			next
		}
		in_hunk && /^\+/ {
			added[line] = substr($0, 2)
			if (line > last) last = line
			line++
		}
		END {
			if (binary) exit 3
			for (i = 1; i <= last; i++) print ((i in added) ? added[i] : "")
		}
	'
}

screen_range() {
	# screen_range <base> <head>: every commit in the range is outbound on its
	# own. History keeps what a later commit deletes, so screening only the
	# range's final files would pass a value that the next commit removed while
	# every clone still carries it. Each commit contributes its message and only
	# the lines it added, staged as `<commit>/<path>` outside the repository and
	# screened from there so a finding names the commit, the file, and the line
	# rather than a temporary path. Content the range did not touch is never
	# rescreened, so the rule binds from its adoption onward. A merge
	# contributes only what it resolved beyond the automatic merge.
	local base=$1 head=$2 commits messages commit parent empty_tree status old new out work rc
	local -a diff_cmd statuses inputs=()
	[[ "$base" =~ ^[0-9a-fA-F]{7,64}$ && "$head" =~ ^[0-9a-fA-F]{7,64}$ ]] ||
		range_error "range input is not a commit ID"
	commits=$(git rev-list --reverse "$base..$head") || range_error "cannot list the range's commits"
	empty_tree=$(git hash-object -t tree /dev/null) || range_error "cannot resolve the empty tree"
	messages=$(git log --format=%B "$base..$head") || range_error "cannot read the range's messages"

	# Messages are outbound text in their own right, whichever hook wrote them.
	# A rebase or an amend can produce a commit no commit-msg hook ever saw.
	"$leaf" --surface commit --text "$messages" || return $?

	work=$(mktemp -d "${TMPDIR:-/tmp}/rhino-public-safety-range.XXXXXX") ||
		range_error "cannot create a staging directory"
	# Removed on every exit path, including a scan error from a subshell.
	# shellcheck disable=SC2064  # the path is fixed now, on purpose
	trap "rm -rf '$work'" EXIT

	for commit in $commits; do
		if git rev-parse --quiet --verify "$commit^2" >/dev/null; then
			diff_cmd=(show --format= --remerge-diff "$commit")
		else
			parent=$(git rev-parse --quiet --verify "$commit^1") || parent=$empty_tree
			diff_cmd=(diff "$parent" "$commit")
		fi
		git "${diff_cmd[@]}" -M --name-status -z --diff-filter=ACMR >"$work/changed" ||
			range_error "cannot list a commit's changes"
		while IFS= read -r -d '' status; do
			IFS= read -r -d '' old || range_error "a commit's change list is truncated"
			new=$old
			if [[ "$status" == R* ]]; then
				IFS= read -r -d '' new || range_error "a commit's change list is truncated"
			fi
			out="$work/history/${commit:0:12}/$new"
			mkdir -p "$(dirname -- "$out")" || range_error "cannot stage a commit's additions"
			git "${diff_cmd[@]}" -M -U0 --no-color --no-ext-diff --no-textconv \
				-- ":(literal)$old" ":(literal)$new" | added_lines >"$out"
			statuses=("${PIPESTATUS[@]}")
			[[ ${statuses[0]} -eq 0 ]] || range_error "cannot read a commit's additions"
			if [[ ${statuses[1]} -eq 3 ]]; then
				git cat-file blob "$commit:$new" >"$out" || range_error "cannot read a binary addition"
			elif [[ ${statuses[1]} -ne 0 ]]; then
				range_error "cannot stage a commit's additions"
			fi
			# An empty addition is still a name this commit publishes.
			inputs+=(--file "${commit:0:12}/$new")
		done <"$work/changed"
	done

	# A range that adds no file has nothing more to screen. Calling the leaf
	# with no inputs would be asking it to guess.
	[[ ${#inputs[@]} -eq 0 ]] && return 0
	(cd "$work/history" && "$leaf" --surface diff "${inputs[@]}")
	rc=$?
	return $rc
}

if [[ "$range" -eq 1 ]]; then
	# The tree gate already screens the tree and the branch name on these same
	# surfaces. This one screens what the range itself publishes into history.
	screen_range "$PUBLIC_SAFETY_BASE" "$PUBLIC_SAFETY_HEAD" || exit $?
	printf '[public-safety] %s range: clean\n' "$surface"
	exit 0
fi

case "$surface" in
pre-commit)
	# The staged surface is what this commit adds -- content and names alike --
	# screened before a formatter rewrites it. The leaf derives the staged list
	# itself, so nothing is assembled here.
	#
	# The tracked baseline is screened at pre-push and in CI rather than here. A
	# full-tree credential scan costs about twenty seconds; paid on every commit
	# it would buy nothing that is not already paid before anything leaves the
	# machine, and a gate that slow on the most frequent surface teaches people
	# to skip it.
	"$leaf" --surface diff || exit $?
	;;

pre-push)
	# Nothing has left the machine yet, so this is the surface that matters. The
	# whole tracked tree and the branch name that is about to appear on a hosted
	# service are screened here. The range gate screens each pushed commit, and
	# typed commit-message gates own the message being written.
	"$leaf" --surface baseline || exit $?
	"$leaf" --surface ref --text "$(current_ref)" || exit $?
	;;

pull-request)
	# A hosted runner publishes its logs, so its checked-out review tree is
	# outbound again on this distinct lifecycle surface. The workflow receives the
	# actual branch name, title, and body from GitHub and screens them separately.
	"$leaf" --surface baseline || exit $?
	;;
esac

printf '[public-safety] %s: clean\n' "$surface"
exit 0
