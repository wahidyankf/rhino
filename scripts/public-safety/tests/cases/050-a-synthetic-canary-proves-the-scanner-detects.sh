# Scenario: A synthetic canary proves the scanner detects.
run() {
	local out rc shapes="$CASE_TMP/shapes.txt"
	write_synthetic_shapes "$shapes"
	printf 'nothing to see\n' >"$CASE_TMP/clean.txt"

	# A scanner that reports nothing at all fails the canary, so substituting a
	# scanner cannot turn a blocked run into a clean one.
	out=$(STUB_MODE=silent "$WRAPPER" --surface release --shapes "$shapes" --file "$CASE_TMP/clean.txt" 2>&1)
	rc=$?
	assert_exit 2 "$rc" "exit code for a scanner that detects nothing" || return 1
	assert_contains "canary" "$out" "refusal" || return 1

	# The ordinary path runs the canary and then the scan, and the canary value
	# reaches neither stream nor any surviving file.
	out=$("$WRAPPER" --surface release --shapes "$shapes" --file "$CASE_TMP/clean.txt" 2>&1)
	rc=$?
	assert_exit 0 "$rc" "exit code for clean content" || return 1
	assert_absent "-----BEGIN" "$out" "clean report" || return 1

	local leftovers
	leftovers=$(find "${TMPDIR:-/tmp}" -maxdepth 1 \
		\( -name 'rhino-public-safety.*' -o -name 'rhino-public-safety-canary.*' \) 2>/dev/null)
	if [[ -n "$leftovers" ]]; then
		echo "    the wrapper left its working directories behind" >&2
		return 1
	fi
}
