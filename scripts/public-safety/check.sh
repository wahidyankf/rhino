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

cd "$root" || exit 2

current_ref() {
	git symbolic-ref --quiet --short HEAD 2>/dev/null || git rev-parse --short HEAD
}

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
	# service are screened here. Typed commit-message gates own commit text.
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
