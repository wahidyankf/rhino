# Scenario: Clean content passes.
run() {
	local out rc shapes="$CASE_TMP/shapes.txt"
	write_synthetic_shapes "$shapes"
	printf 'a release that names nothing private\n' >"$CASE_TMP/notes.txt"

	out=$("$WRAPPER" --surface release --shapes "$shapes" --file "$CASE_TMP/notes.txt" 2>&1)
	rc=$?
	assert_exit 0 "$rc" "exit code for clean content" || return 1
	assert_contains "release: clean" "$out" "report" || return 1
}
