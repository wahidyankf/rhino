# Scenario: the wrapper is the first gate on every hook surface.
#
# Written after a real failure: the hooks called the wrapper first and then ran
# commitlint anyway, so a blocked commit printed its finding and was created. A
# gate that reports without stopping is the one failure mode a gate cannot have,
# and the wiring is asserted rather than assumed.
#
# The seam moved when this repository adopted `ose/repo-config/v2`. A hook no
# longer names the wrapper; it names a surface, and `repo-config.yml` says which
# gates that surface selects and in what order. So the same property is now two
# assertions instead of one: the hook dispatches its own surface, and the
# surface declares `public-safety` first. Both have to hold -- a hook pointed at
# the wrong surface would pass the second check while screening the wrong thing.
run() {
	local hook path first config
	config="$PUBLIC_SAFETY_ROOT/repo-config.yml"

	if [[ ! -r "$config" ]]; then
		echo "    repo-config.yml is not there, so no surface declares anything" >&2
		return 1
	fi

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
		# itself has to dispatch this hook's own surface.
		first=$(grep -vE '^[[:space:]]*(#|$)' "$path" | grep -vE '^set -e$' | head -1)
		assert_contains "gate run --surface $hook" "$first" "first command in .husky/$hook" || return 1

		# And that surface has to run the screen before anything else. The gate
		# list is ordered, so "first" means the first entry naming the surface.
		if ! first_gate_on "$hook" "$config" | grep -qx "public-safety"; then
			echo "    repo-config.yml does not declare public-safety first on $hook" >&2
			echo "    first gate on $hook: $(first_gate_on "$hook" "$config")" >&2
			return 1
		fi
	done

	# Git hands commit-msg a path inside .git. That file is never published and,
	# in a worktree, is an absolute path under a home directory: screening it
	# would report the hook's own argument on every commit. The dispatcher reads
	# the message out of the file and screens the text.
	# shellcheck disable=SC2016  # the dollar is the dispatcher's argument, not ours
	if grep -qE 'surface commit --file "\$1"' "$PUBLIC_SAFETY_ROOT/scripts/public-safety/check.sh"; then
		echo "    check.sh screens git's message path instead of the message" >&2
		return 1
	fi
}

# The id of the first gate whose surface list contains the given surface.
#
# Reads the declared order rather than a set: two gates on one surface are
# ordered by where they appear, which is the whole property under test.
first_gate_on() {
	local surface=$1 config=$2
	awk -v want="$surface" '
		/^gates:/ { in_gates = 1; next }
		!in_gates { next }
		/^[a-z]/ { exit }
		/^  - id: / { id = $3; on = 0; next }
		/^    surfaces:$/ { on = 1; next }
		/^    [a-z]/ { on = 0; next }
		on && $1 == "-" && $2 == want { print id; exit }
	' "$config"
}
