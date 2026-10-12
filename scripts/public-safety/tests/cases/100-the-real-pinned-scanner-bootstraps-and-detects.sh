# Scenario: the pinned scanner, bootstrapped for real.
#
# Every other case runs against a stub so the suite is offline and fast enough
# for a Git hook. This one downloads, verifies, and canaries the real binary. It
# is skipped unless PUBLIC_SAFETY_ONLINE=1, and reported as skipped rather than
# as passed, because a network gate that quietly reports success when it never
# ran is worse than no gate.
run() {
	if [[ "${PUBLIC_SAFETY_ONLINE:-0}" != "1" ]]; then
		skip_case "set PUBLIC_SAFETY_ONLINE=1 to bootstrap the real scanner"
		return 0
	fi

	local out rc canary shapes="$CASE_TMP/shapes.txt"
	write_synthetic_shapes "$shapes"
	unset PUBLIC_SAFETY_SCANNER

	write_inert_credential "$CASE_TMP/inert.txt"
	canary=$(<"$CASE_TMP/inert.txt")

	out=$("$WRAPPER" --surface release --shapes "$shapes" --file "$CASE_TMP/inert.txt" 2>&1)
	rc=$?
	assert_exit 1 "$rc" "exit code from the real scanner over an inert fixture" || return 1
	assert_absent "$canary" "$out" "diagnostic" || return 1
	assert_absent "${canary#*_}" "$out" "diagnostic" || return 1

	printf 'nothing private here\n' >"$CASE_TMP/clean.txt"
	out=$("$WRAPPER" --surface release --shapes "$shapes" --file "$CASE_TMP/clean.txt" 2>&1)
	rc=$?
	assert_exit 0 "$rc" "exit code from the real scanner over clean content" || return 1
}
