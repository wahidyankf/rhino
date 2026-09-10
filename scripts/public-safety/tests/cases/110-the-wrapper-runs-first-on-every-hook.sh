# Scenario: the wrapper is the first gate on every hook surface.
#
# Written after a real failure: the hooks called the wrapper first and then ran
# commitlint anyway, so a blocked commit printed its finding and was created. A
# gate that reports without stopping is the one failure mode a gate cannot have,
# and the wiring is now asserted rather than assumed.
run() {
	local hook path first
	for hook in commit-msg pre-commit pre-push; do
		path="$PUBLIC_SAFETY_ROOT/.husky/$hook"
		if [[ ! -r "$path" ]]; then
			echo "    .husky/$hook is not there" >&2
			return 1
		fi

		if ! grep -qE '^set -e$' "$path"; then
			echo "    .husky/$hook does not stop on a failing gate" >&2
			return 1
		fi

		# The first thing that is not a comment, a blank line, or the `set`
		# itself has to be this wrapper.
		first=$(grep -vE '^[[:space:]]*(#|$)' "$path" | grep -vE '^set -e$' | head -1)
		assert_contains "public-safety.sh" "$first" "first command in .husky/$hook" || return 1
	done

	# Git hands commit-msg a path inside .git. That file is never published and,
	# in a worktree, is an absolute path under a home directory: screening it
	# would report the hook's own argument on every commit.
	# shellcheck disable=SC2016  # the dollar is the hook's argument, not ours
	if grep -qE 'public-safety\.sh.*--file "\$1"' "$PUBLIC_SAFETY_ROOT/.husky/commit-msg"; then
		echo "    .husky/commit-msg screens git's message path instead of the message" >&2
		return 1
	fi
}
