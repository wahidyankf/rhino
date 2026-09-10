# Scenario: A generic private shape blocks.
run() {
	local out rc shapes="$CASE_TMP/shapes.txt"
	write_synthetic_shapes "$shapes"

	printf 'the build ran from /Users/%s/checkout\n' "$SYNTHETIC_NAME" >"$CASE_TMP/notes.txt"
	out=$("$WRAPPER" --surface pull-request --shapes "$shapes" --file "$CASE_TMP/notes.txt" 2>&1)
	rc=$?
	assert_exit 1 "$rc" "exit code for an absolute maintainer path" || return 1
	assert_contains "finding maintainer-path" "$out" "diagnostic" || return 1
	assert_absent "$SYNTHETIC_NAME" "$out" "diagnostic" || return 1

	# Outbound text is screened the same way a file is.
	out=$("$WRAPPER" --surface commit --shapes "$shapes" \
		--text "deployed to synthetic-host-4242.invalid" 2>&1)
	rc=$?
	assert_exit 1 "$rc" "exit code for an internal host in a commit message" || return 1
	assert_contains "finding internal-host" "$out" "diagnostic" || return 1

	# The wrapper's own shipped shape set is the default, and it is generic: it
	# blocks a private address without any repository having to configure it.
	out=$("$WRAPPER" --surface commit --text "reachable at 10.1.2.3:8006" 2>&1)
	rc=$?
	assert_exit 1 "$rc" "exit code for a private address under the shipped shape set" || return 1
}
