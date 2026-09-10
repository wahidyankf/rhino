# Scenario: The pinned scanner is the only scanner.
#
# Offline by construction. The pin is checked by asking the wrapper what it is
# pinned to and by refusing platforms and cache entries, none of which needs a
# download.
run() {
	local out rc plat

	out=$("$WRAPPER" --print-pin 2>&1)
	rc=$?
	assert_exit 0 "$rc" "exit code for --print-pin" || return 1
	assert_contains "trufflehog 3.97.1" "$out" "pin report" || return 1

	# The four approved rows, copied from the plan's verified baseline. A digest
	# that drifts from this table means the pin moved without the table moving.
	assert_contains "darwin_amd64 1515710bb16be5653ca9986c27ecd1a0e7536fc6e53ad46f7100992692f6a05f" "$out" "pin table" || return 1
	assert_contains "darwin_arm64 1af86cf30c1cc5c1735ec6af9292b399ec9bed3ff1b30be13fcbfd4a30ab449a" "$out" "pin table" || return 1
	assert_contains "linux_amd64 f863ea3a8d786f7d097870496c977944cce7372a2fe1e56707d965016e543ece" "$out" "pin table" || return 1
	assert_contains "linux_arm64 57bfcc0988aae3f2ef97e74abe1138cf37a8fbd84dd26299062c77a6a6b125dd" "$out" "pin table" || return 1

	# A platform outside the closed matrix is refused rather than guessed at.
	local shapes="$CASE_TMP/shapes.txt"
	write_synthetic_shapes "$shapes"
	printf 'nothing to see\n' >"$CASE_TMP/clean.txt"

	out=$(PUBLIC_SAFETY_PLATFORM=plan9_vax PUBLIC_SAFETY_SCANNER='' \
		"$WRAPPER" --surface release --shapes "$shapes" --file "$CASE_TMP/clean.txt" 2>&1)
	rc=$?
	assert_exit 2 "$rc" "exit code for an unsupported platform" || return 1
	assert_contains "unsupported platform" "$out" "refusal" || return 1

	# A cached scanner that no longer matches what was extracted is refused. The
	# stamp is written at extraction, after the tarball digest was checked, so a
	# binary that drifted from it drifted after verification.
	plat=$("$WRAPPER" --print-platform)
	local dir="$CASE_TMP/tampered/trufflehog/3.97.1/$plat"
	mkdir -p "$dir"
	cp "$STUB" "$dir/trufflehog"
	printf '%s\n' "0000000000000000000000000000000000000000000000000000000000000000" >"$dir/trufflehog.sha256"

	out=$(PUBLIC_SAFETY_CACHE="$CASE_TMP/tampered" PUBLIC_SAFETY_SCANNER='' \
		"$WRAPPER" --surface release --shapes "$shapes" --file "$CASE_TMP/clean.txt" 2>&1)
	rc=$?
	assert_exit 2 "$rc" "exit code for a cached scanner that does not match its stamp" || return 1
	assert_contains "does not match" "$out" "refusal" || return 1
}
