#!/usr/bin/env bash
# ==============================================================================
# check.sh — this repository's public-safety gate
# ==============================================================================
# Usage: OSE_GATE_SURFACE=<surface> scripts/public-safety/check.sh [hook args]
#
#   commit-msg   $1 is the file holding the message being written
#   pre-commit   no arguments; the staged tree is the subject
#   pre-push     ref updates arrive on stdin, as Git supplies them
#   ci           no arguments; the checked-out tree is the subject
#
# The surface arrives in the environment and nowhere else. `rhino gate run`
# exports it before starting each child. It is never inferred from an argument's
# filename, from which hook happens to be running, or from whether a remote is
# reachable -- a gate that guesses its own surface will eventually guess a
# weaker one, and that is exactly the case where guessing is expensive.
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

surface="${OSE_GATE_SURFACE:-}"
case "$surface" in
commit-msg | pre-commit | pre-push | ci) ;;
"")
	printf '[public-safety] blocked scan-error OSE_GATE_SURFACE is unset\n' >&2
	exit 2
	;;
*)
	printf '[public-safety] blocked scan-error OSE_GATE_SURFACE is not a known surface\n' >&2
	exit 2
	;;
esac

cd "$root" || exit 2

current_ref() {
	git symbolic-ref --quiet --short HEAD 2>/dev/null || git rev-parse --short HEAD
}

case "$surface" in
commit-msg)
	# The message is passed as text rather than as the file Git names. That path
	# is a local scratch file inside `.git` which is never published, and in a
	# worktree it is an absolute path under somebody's home directory -- so
	# screening the path would report the hook's own argument on every commit.
	[[ $# -ge 1 && -r "$1" ]] || {
		printf '[public-safety] blocked scan-error commit-msg received no readable message file\n' >&2
		exit 2
	}
	"$leaf" --surface commit --text "$(cat "$1")" || exit $?
	;;

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
	# whole tracked tree, then the branch name that is about to appear on a
	# hosted service, then every commit message the push would publish.
	"$leaf" --surface baseline || exit $?
	"$leaf" --surface ref --text "$(current_ref)" || exit $?

	# Git supplies `<local-ref> <local-sha> <remote-ref> <remote-sha>` per line.
	# A deletion carries an all-zero local sha and publishes no message.
	ranges=()
	while read -r local_ref local_sha remote_ref remote_sha; do
		[[ -z "${local_ref:-}" ]] && continue
		[[ "$local_sha" =~ ^0+$ ]] && continue
		if [[ "$remote_sha" =~ ^0+$ ]]; then
			ranges+=("$local_sha --not --remotes")
		else
			ranges+=("$remote_sha..$local_sha")
		fi
	done

	# Invoked outside a hook, with nothing on stdin: screen what is here now
	# rather than reporting a vacuous pass over an empty list.
	[[ ${#ranges[@]} -eq 0 ]] && ranges=("HEAD --not --remotes")

	messages=()
	for range in "${ranges[@]}"; do
		# shellcheck disable=SC2086
		while IFS= read -r sha; do
			[[ -n "$sha" ]] && messages+=(--text "$(git log -1 --format=%B "$sha")")
		done < <(git rev-list $range 2>/dev/null)
	done
	if [[ ${#messages[@]} -gt 0 ]]; then
		"$leaf" --surface commit "${messages[@]}" || exit $?
	fi
	;;

ci)
	# A hosted runner publishes its logs, so the same tree and the same names are
	# outbound again on a surface no local hook can reach.
	"$leaf" --surface baseline || exit $?
	"$leaf" --surface ref --text "$(current_ref)" || exit $?
	"$leaf" --surface commit --text "$(git log -1 --format=%B HEAD)" || exit $?
	;;
esac

printf '[public-safety] %s: clean\n' "$surface"
exit 0
