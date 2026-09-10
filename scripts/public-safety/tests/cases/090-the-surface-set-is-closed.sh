# Scenario: The surface set is closed.
run() {
	local out rc surface shapes="$CASE_TMP/shapes.txt"
	write_synthetic_shapes "$shapes"
	printf 'nothing private here\n' >"$CASE_TMP/clean.txt"

	for surface in baseline diff commit ref pull-request release logs; do
		out=$("$WRAPPER" --surface "$surface" --shapes "$shapes" --file "$CASE_TMP/clean.txt" 2>&1)
		rc=$?
		assert_exit 0 "$rc" "exit code for surface $surface" || return 1
		assert_contains "$surface: clean" "$out" "report for $surface" || return 1
	done

	out=$("$WRAPPER" --surface everything --shapes "$shapes" --file "$CASE_TMP/clean.txt" 2>&1)
	rc=$?
	assert_exit 2 "$rc" "exit code for a surface outside the set" || return 1
	assert_contains "unsupported surface" "$out" "refusal" || return 1

	# No input at all is a scan error rather than a clean run: a wrapper that
	# reported "clean" for nothing would pass on every surface that forgot to
	# hand it anything.
	out=$("$WRAPPER" --surface release --shapes "$shapes" 2>&1)
	rc=$?
	assert_exit 2 "$rc" "exit code when no outbound input was given" || return 1
}
