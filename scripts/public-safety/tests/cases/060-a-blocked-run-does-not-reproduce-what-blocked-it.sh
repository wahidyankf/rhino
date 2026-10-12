# Scenario: A blocked run does not reproduce what blocked it.
run() {
	local out rc canary shapes="$CASE_TMP/shapes.txt"
	write_synthetic_shapes "$shapes"

	# A locally fabricated unissued marker, assembled only at runtime.
	write_inert_credential "$CASE_TMP/inert.txt"
	canary=$(<"$CASE_TMP/inert.txt")

	{
		printf 'release notes\n'
		cat "$CASE_TMP/inert.txt"
	} >"$CASE_TMP/notes.txt"

	out=$("$WRAPPER" --surface release --shapes "$shapes" --file "$CASE_TMP/notes.txt" 2>&1)
	rc=$?
	assert_exit 1 "$rc" "exit code for content carrying a credential" || return 1

	assert_contains "finding Github" "$out" "diagnostic" || return 1
	assert_contains "blocked" "$out" "diagnostic" || return 1
	assert_absent "$canary" "$out" "diagnostic" || return 1
	assert_absent "${canary#*_}" "$out" "diagnostic" || return 1
	assert_absent '"Raw"' "$out" "diagnostic" || return 1
	assert_absent "REDACTED-BY-NOBODY" "$out" "diagnostic" || return 1
	assert_absent "SourceMetadata" "$out" "diagnostic" || return 1
}
