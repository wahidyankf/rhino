# Scenario: each hook's first gate blocks, and each hook dispatches its own surface.
#
# Written after a real failure: the hooks called the wrapper first and then ran
# commitlint anyway, so a blocked commit printed its finding and was created. A
# gate that reports without stopping is the one failure mode a gate cannot have,
# and the wiring is asserted rather than assumed.
#
# The surface contract (C6) gives each local hook one kind of work, declared in
# `repo-config.yml` under `gates.wiring.local-surfaces`:
#
#   pre-commit  -> format-staged        (staged-file formatting only)
#   commit-msg  -> commit-message       (the commit-message format lint only)
#   pre-push    -> public-safety-range  (the leak scan over the pushed range only)
#
# So the leak scan is the pre-push hook, and it screens the pushed range commit
# by commit before anything leaves the machine. The tree and message screens
# (public-safety-tree and public-safety-message) are not on any hook. The
# pull-request gate runs them (C11), so this case also asserts that surface still
# declares both.
#
# Each hook therefore has three properties to hold. The hook dispatches its own
# surface, the first gate declared on that surface is the one expected for the
# hook, and the screens that left the hooks still run at the pull-request
# surface. A hook pointed at the wrong surface would pass the second check while
# screening the wrong thing, so the first two have to hold together.
run() {
	local hook path first expected config gate
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

		# Bootstrap may establish the repository-local formatter path. The first
		# executable dispatch still has to select this hook's own typed surface.
		first=$(grep -E '^exec ' "$path" | head -1)
		assert_contains "gate run --surface $hook" "$first" "dispatch in .husky/$hook" || return 1

		# The gate list is ordered, so "first" means the first entry naming the
		# surface. Each local surface holds one gate under C6, and pre-push is the
		# surface that screens the pushed range, so its first gate is the range scan.
		case "$hook" in
		commit-msg) expected=commit-message ;;
		pre-commit) expected=format-staged ;;
		pre-push) expected=public-safety-range ;;
		esac
		if ! first_gate_on "$hook" "$config" | grep -qx "$expected"; then
			echo "    repo-config.yml does not declare $expected first on $hook" >&2
			echo "    first gate on $hook: $(first_gate_on "$hook" "$config")" >&2
			return 1
		fi
	done

	# C11: the tree and message screens run in the pull-request gate. Moving them
	# off the hooks must not drop them from the gate that replaces the hooks.
	for gate in public-safety-tree public-safety-message; do
		if ! gates_on pull-request "$config" | grep -qx "$gate"; then
			echo "    repo-config.yml does not declare $gate on pull-request" >&2
			return 1
		fi
	done

	# Git hands commit-msg a path inside .git. That file is never published and
	# is resolved only by Rhino's typed commit-message binding. The safety wrapper
	# receives the text projection, never the pathname.
	# shellcheck disable=SC2016  # the dollar is the dispatcher's argument, not ours
	if grep -qE 'surface commit --file "\$1"' "$PUBLIC_SAFETY_ROOT/scripts/public-safety/check-commit-message.sh"; then
		echo "    check-commit-message.sh screens git's message path instead of the message" >&2
		return 1
	fi
}

# The ids of the gates whose surface list contains the given surface, in the
# declared order.
#
# Reads the declared order rather than a set: two gates on one surface are
# ordered by where they appear, which is the whole property under test.
gates_on() {
	local surface=$1 config=$2
	awk -v want="$surface" '
		/^gates:/ { in_gates = 1; next }
		!in_gates { next }
		/^[a-z]/ { exit }
		/^    - id: / { id = $3; on = 0; next }
		/^      run-on:$/ { on = 1; next }
		/^      [a-z]/ { on = 0; next }
		on && $1 == want ":" { print id }
	' "$config"
}

# The id of the first gate whose surface list contains the given surface.
first_gate_on() {
	gates_on "$@" | head -n 1
}
