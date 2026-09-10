# Scenario: A scanner error blocks.
run() {
	local out rc shapes="$CASE_TMP/shapes.txt"
	write_synthetic_shapes "$shapes"
	printf 'nothing to see\n' >"$CASE_TMP/clean.txt"

	out=$(STUB_MODE=error "$WRAPPER" --surface release --shapes "$shapes" --file "$CASE_TMP/clean.txt" 2>&1)
	rc=$?
	assert_exit 2 "$rc" "exit code for a scanner that failed" || return 1
	assert_contains "scan-error" "$out" "refusal" || return 1
	assert_absent "stub: the scan did not complete" "$out" "refusal" || return 1

	out=$(STUB_MODE=garbage "$WRAPPER" --surface release --shapes "$shapes" --file "$CASE_TMP/clean.txt" 2>&1)
	rc=$?
	assert_exit 2 "$rc" "exit code for an undeclared record shape" || return 1
	assert_absent "Unexpected" "$out" "refusal" || return 1
}
