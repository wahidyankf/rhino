# Scenario: A blocked run does not reproduce what blocked it.
run() {
	local out rc canary shapes="$CASE_TMP/shapes.txt"
	write_synthetic_shapes "$shapes"

	# A real key generated at runtime, so the value exists in no committed file.
	openssl genrsa -out "$CASE_TMP/canary.pem" 2048 2>/dev/null || {
		echo "    cannot generate a canary key" >&2
		return 1
	}
	canary=$(sed -n '3p' "$CASE_TMP/canary.pem")

	{
		printf 'release notes\n'
		cat "$CASE_TMP/canary.pem"
	} >"$CASE_TMP/notes.txt"

	out=$("$WRAPPER" --surface release --shapes "$shapes" --file "$CASE_TMP/notes.txt" 2>&1)
	rc=$?
	assert_exit 1 "$rc" "exit code for content carrying a credential" || return 1

	assert_contains "finding PrivateKey" "$out" "diagnostic" || return 1
	assert_contains "blocked" "$out" "diagnostic" || return 1
	assert_absent "$canary" "$out" "diagnostic" || return 1
	assert_absent '"Raw"' "$out" "diagnostic" || return 1
	assert_absent "REDACTED-BY-NOBODY" "$out" "diagnostic" || return 1
	assert_absent "SourceMetadata" "$out" "diagnostic" || return 1
}
